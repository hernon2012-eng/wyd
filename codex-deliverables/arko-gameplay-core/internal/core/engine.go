package core

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

type EngineOptions struct {
	AllowDemoRecipes bool
	Requirements     RequirementChecker
	Now              func() time.Time
}

type Engine struct {
	repository Repository
	options    EngineOptions
}

func NewEngine(repository Repository, options EngineOptions) (*Engine, error) {
	if repository == nil {
		return nil, ErrInvalidRequest
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	return &Engine{repository: repository, options: options}, nil
}

func (e *Engine) Craft(ctx context.Context, actor Actor, request CraftRequest) (OperationResult, error) {
	if !validRequestID(request.RequestID) || !validOperationScope(OperationScope{CharacterID: request.CharacterID, Type: OperationCraft, Key: request.IdempotencyKey}) || !validID(string(request.RecipeID)) || request.RecipeVersion == 0 {
		return OperationResult{}, ErrInvalidRequest
	}
	if !actorAllows(actor, request.CharacterID) {
		return OperationResult{}, ErrUnauthorized
	}
	scope := OperationScope{CharacterID: request.CharacterID, Type: OperationCraft, Key: request.IdempotencyKey}
	fingerprint, err := fingerprint(struct {
		RecipeID      RecipeID `json:"recipe_id"`
		RecipeVersion uint64   `json:"recipe_version"`
		Quantity      int64    `json:"quantity"`
	}{request.RecipeID, request.RecipeVersion, request.Quantity})
	if err != nil {
		return OperationResult{}, err
	}
	return e.execute(ctx, scope, request.RequestID, fingerprint, func(tx Transaction) (OperationResult, error) {
		if request.Quantity <= 0 || request.Quantity > MaxBatches {
			return rejected(CodeInvalidRequest), nil
		}
		recipe, ok := tx.Recipe(request.RecipeID, request.RecipeVersion)
		if !ok {
			return rejected(CodeRecipeNotFound), nil
		}
		if !recipe.Active {
			return rejected(CodeRecipeInactive), nil
		}
		if recipe.DemoOnly && !e.options.AllowDemoRecipes {
			return rejected(CodeDemoRecipeDisabled), nil
		}
		if len(recipe.Requirements) > 0 {
			if e.options.Requirements == nil {
				return rejected(CodeRequirementsNotMet), nil
			}
			accepted, checkErr := e.options.Requirements.Check(ctx, actor, request.CharacterID, append([]RecipeRequirement(nil), recipe.Requirements...))
			if checkErr != nil {
				return OperationResult{}, fmt.Errorf("%w: requirement evaluation: %v", ErrStorage, checkErr)
			}
			if !accepted {
				return rejected(CodeRequirementsNotMet), nil
			}
		}
		inventory, exists := tx.Inventory(request.CharacterID)
		if !exists {
			return OperationResult{}, ErrNotFound
		}
		definitions := make(map[ItemID]ItemDefinition)
		for _, amount := range recipe.Materials {
			if definition, found := tx.ItemDefinition(amount.ItemID); found {
				definitions[amount.ItemID] = definition
			}
		}
		if definition, found := tx.ItemDefinition(recipe.Product.ItemID); found {
			definitions[recipe.Product.ItemID] = definition
		}
		for _, slot := range inventory.Slots {
			if slot.ItemID == "" {
				continue
			}
			if definition, found := tx.ItemDefinition(slot.ItemID); found {
				definitions[slot.ItemID] = definition
			}
		}
		updated, consumed, produced, code := PlanCraft(inventory, recipe, request.Quantity, definitions)
		if code != "" {
			return rejected(code), nil
		}
		if err := tx.PutInventory(updated); err != nil {
			return OperationResult{}, err
		}
		return OperationResult{Status: OperationCompleted, Consumed: consumed, Produced: []ItemAmount{produced}}, nil
	})
}

func (e *Engine) Add(ctx context.Context, actor Actor, request AddRequest) (OperationResult, error) {
	if !validRequestID(request.RequestID) || !validOperationScope(OperationScope{CharacterID: request.CharacterID, Type: OperationAdd, Key: request.IdempotencyKey}) || !validID(string(request.Item.ItemID)) {
		return OperationResult{}, ErrInvalidRequest
	}
	if !actorAllows(actor, request.CharacterID) {
		return OperationResult{}, ErrUnauthorized
	}
	fingerprint, err := fingerprint(request.Item)
	if err != nil {
		return OperationResult{}, err
	}
	scope := OperationScope{CharacterID: request.CharacterID, Type: OperationAdd, Key: request.IdempotencyKey}
	return e.execute(ctx, scope, request.RequestID, fingerprint, func(tx Transaction) (OperationResult, error) {
		if request.Item.Quantity <= 0 {
			return rejected(CodeInvalidRequest), nil
		}
		definition, ok := tx.ItemDefinition(request.Item.ItemID)
		if !ok {
			return rejected(CodeItemNotFound), nil
		}
		inventory, ok := tx.Inventory(request.CharacterID)
		if !ok {
			return OperationResult{}, ErrNotFound
		}
		if err := AddItems(&inventory, definition, request.Item.Quantity); err != nil {
			if errors.Is(err, ErrInventoryFull) {
				return rejected(CodeInventoryFull), nil
			}
			return rejected(CodeInvalidRequest), nil
		}
		if err := tx.PutInventory(inventory); err != nil {
			return OperationResult{}, err
		}
		return OperationResult{Status: OperationCompleted, Produced: []ItemAmount{request.Item}}, nil
	})
}

func (e *Engine) Remove(ctx context.Context, actor Actor, request RemoveRequest) (OperationResult, error) {
	if !validRequestID(request.RequestID) || !validOperationScope(OperationScope{CharacterID: request.CharacterID, Type: OperationRemove, Key: request.IdempotencyKey}) || !validID(string(request.Item.ItemID)) {
		return OperationResult{}, ErrInvalidRequest
	}
	if !actorAllows(actor, request.CharacterID) {
		return OperationResult{}, ErrUnauthorized
	}
	fingerprint, err := fingerprint(request.Item)
	if err != nil {
		return OperationResult{}, err
	}
	scope := OperationScope{CharacterID: request.CharacterID, Type: OperationRemove, Key: request.IdempotencyKey}
	return e.execute(ctx, scope, request.RequestID, fingerprint, func(tx Transaction) (OperationResult, error) {
		if request.Item.Quantity <= 0 {
			return rejected(CodeInvalidRequest), nil
		}
		if _, ok := tx.ItemDefinition(request.Item.ItemID); !ok {
			return rejected(CodeItemNotFound), nil
		}
		inventory, ok := tx.Inventory(request.CharacterID)
		if !ok {
			return OperationResult{}, ErrNotFound
		}
		if err := RemoveItems(&inventory, request.Item.ItemID, request.Item.Quantity); err != nil {
			if errors.Is(err, ErrInsufficientItems) {
				return rejected(CodeInsufficientItems), nil
			}
			return rejected(CodeInvalidRequest), nil
		}
		if err := tx.PutInventory(inventory); err != nil {
			return OperationResult{}, err
		}
		return OperationResult{Status: OperationCompleted, Consumed: []ItemAmount{request.Item}}, nil
	})
}

func (e *Engine) Transfer(ctx context.Context, actor Actor, request TransferRequest) (OperationResult, error) {
	if !validRequestID(request.RequestID) || !validOperationScope(OperationScope{CharacterID: request.Source, Type: OperationTransfer, Key: request.IdempotencyKey}) || !validID(string(request.Destination)) || !validID(string(request.Item.ItemID)) || request.Source == request.Destination {
		return OperationResult{}, ErrInvalidRequest
	}
	if !actorAllows(actor, request.Source) || !actorAllows(actor, request.Destination) {
		return OperationResult{}, ErrUnauthorized
	}
	fingerprint, err := fingerprint(struct {
		Destination CharacterID `json:"destination"`
		Item        ItemAmount  `json:"item"`
	}{request.Destination, request.Item})
	if err != nil {
		return OperationResult{}, err
	}
	scope := OperationScope{CharacterID: request.Source, Type: OperationTransfer, Key: request.IdempotencyKey}
	return e.execute(ctx, scope, request.RequestID, fingerprint, func(tx Transaction) (OperationResult, error) {
		if request.Item.Quantity <= 0 {
			return rejected(CodeInvalidRequest), nil
		}
		definition, ok := tx.ItemDefinition(request.Item.ItemID)
		if !ok {
			return rejected(CodeItemNotFound), nil
		}
		source, sourceOK := tx.Inventory(request.Source)
		destination, destinationOK := tx.Inventory(request.Destination)
		if !sourceOK || !destinationOK {
			return OperationResult{}, ErrNotFound
		}
		if err := TransferItems(&source, &destination, definition, request.Item.Quantity); err != nil {
			if errors.Is(err, ErrInsufficientItems) {
				return rejected(CodeInsufficientItems), nil
			}
			if errors.Is(err, ErrInventoryFull) {
				return rejected(CodeInventoryFull), nil
			}
			return rejected(CodeInvalidRequest), nil
		}
		if err := tx.PutInventory(source); err != nil {
			return OperationResult{}, err
		}
		if err := tx.PutInventory(destination); err != nil {
			return OperationResult{}, err
		}
		return OperationResult{Status: OperationCompleted, Consumed: []ItemAmount{request.Item}}, nil
	})
}

func (e *Engine) Inventory(ctx context.Context, actor Actor, characterID CharacterID) (Inventory, error) {
	if !actorAllows(actor, characterID) {
		return Inventory{}, ErrUnauthorized
	}
	var inventory Inventory
	err := e.repository.Transact(ctx, func(tx Transaction) error {
		var ok bool
		inventory, ok = tx.Inventory(characterID)
		if !ok {
			return ErrNotFound
		}
		return nil
	})
	return inventory, err
}

func (e *Engine) execute(ctx context.Context, scope OperationScope, requestID, requestFingerprint string, apply func(Transaction) (OperationResult, error)) (OperationResult, error) {
	var result OperationResult
	err := e.repository.Transact(ctx, func(tx Transaction) error {
		if previous, exists := tx.Operation(scope); exists {
			if previous.Fingerprint != requestFingerprint {
				return ErrIdempotencyConflict
			}
			result = cloneOperationResult(previous.Result)
			result.RequestID = requestID
			result.Replayed = true
			return nil
		}
		operationID, err := newOperationID()
		if err != nil {
			return fmt.Errorf("%w: operation id generation: %v", ErrStorage, err)
		}
		result, err = apply(tx)
		if err != nil {
			return err
		}
		if result.Status != OperationCompleted && result.Status != OperationRejected {
			return fmt.Errorf("%w: invalid terminal operation result", ErrStorage)
		}
		result.OperationID = operationID
		result.RequestID = requestID
		result.Replayed = false
		now := e.options.Now().UTC()
		record := OperationRecord{Scope: scope, Fingerprint: requestFingerprint, RequestID: requestID, Result: cloneOperationResult(result), CreatedAt: now, UpdatedAt: now}
		return tx.PutOperation(record)
	})
	if errors.Is(err, ErrIdempotencyConflict) {
		operationID, idErr := newOperationID()
		if idErr != nil {
			return OperationResult{}, fmt.Errorf("%w: operation id generation: %v", ErrStorage, idErr)
		}
		return OperationResult{OperationID: operationID, RequestID: requestID, Status: OperationRejected, Code: CodeIdempotencyConflict}, nil
	}
	return result, err
}

func rejected(code ErrorCode) OperationResult {
	return OperationResult{Status: OperationRejected, Code: code}
}

func actorAllows(actor Actor, characterID CharacterID) bool {
	if actor.PrincipalID == "" {
		return false
	}
	for _, allowed := range actor.AuthorizedCharacters {
		if allowed == characterID {
			return true
		}
	}
	return false
}

func validRequestID(requestID string) bool { return validID(requestID) }

func fingerprint(value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func newOperationID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
