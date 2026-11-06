package core

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"
)

type EngineOptions struct {
	AllowDemoRecipes  bool
	Requirements      RequirementChecker
	TypedRequirements TypedRequirementChecker
	MaterialResolver  MaterialSelectorResolver
	RecipeProvider    RecipeProvider
	Now               func() time.Time
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
	if !validRequestID(request.RequestID) || !validOperationScope(OperationScope{CharacterID: request.CharacterID, Type: OperationCraft, Key: request.IdempotencyKey}) || !validID(string(request.RecipeID)) || request.RecipeVersion == 0 || request.RecipeVersion > math.MaxInt64 {
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
		var recipe Recipe
		var ok bool
		var err error
		if e.options.RecipeProvider != nil {
			recipe, ok, err = e.options.RecipeProvider.ReadRecipe(ctx, request.RecipeID, request.RecipeVersion)
		} else {
			recipe, ok, err = tx.ReadRecipe(request.RecipeID, request.RecipeVersion)
		}
		if err != nil {
			return OperationResult{}, err
		}
		if !ok {
			return rejected(CodeRecipeNotFound), nil
		}
		var validationErr error
		if e.options.RecipeProvider != nil {
			validationErr = ValidateRecipeDefinitionV2(recipe)
		} else {
			validationErr = ValidateRecipe(recipe)
		}
		if validationErr != nil {
			return rejected(CodeInvalidRequest), nil
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
		if len(recipe.TypedRequirements) > 0 {
			if e.options.TypedRequirements == nil {
				return rejected(CodeRequirementsNotMet), nil
			}
			accepted, checkErr := e.options.TypedRequirements.CheckTyped(ctx, actor, request.CharacterID, cloneRecipe(recipe).TypedRequirements)
			if checkErr != nil {
				return OperationResult{}, fmt.Errorf("%w: typed requirement evaluation: %v", ErrStorage, checkErr)
			}
			if !accepted {
				return rejected(CodeRequirementsNotMet), nil
			}
		}
		inventory, exists, err := tx.ReadInventory(request.CharacterID)
		if err != nil {
			return OperationResult{}, err
		}
		if !exists {
			return OperationResult{}, ErrNotFound
		}
		planningRecipe, resolved, resolveErr := e.resolveMaterials(ctx, actor, request.CharacterID, inventory, recipe)
		if resolveErr != nil {
			return OperationResult{}, fmt.Errorf("%w: material selection: %v", ErrStorage, resolveErr)
		}
		if !resolved {
			return rejected(CodeMaterialResolutionFailed), nil
		}
		if planningRecipe.OutputMode == RecipeOutputTransform {
			planningRecipe.Materials = append(planningRecipe.Materials, *planningRecipe.TransformSource)
			planningRecipe.MaterialSelectors = nil
		}
		definitions := make(map[ItemID]ItemDefinition)
		for _, amount := range planningRecipe.Materials {
			definition, found, err := tx.ReadItemDefinition(amount.ItemID)
			if err != nil {
				return OperationResult{}, err
			}
			if found {
				definitions[amount.ItemID] = definition
			}
		}
		if definition, found, err := tx.ReadItemDefinition(planningRecipe.Product.ItemID); err != nil {
			return OperationResult{}, err
		} else if found {
			definitions[planningRecipe.Product.ItemID] = definition
		}
		for _, slot := range inventory.Slots {
			if slot.ItemID == "" {
				continue
			}
			definition, found, err := tx.ReadItemDefinition(slot.ItemID)
			if err != nil {
				return OperationResult{}, err
			}
			if found {
				definitions[slot.ItemID] = definition
			}
		}
		updated, consumed, produced, code := PlanCraft(inventory, planningRecipe, request.Quantity, definitions)
		if code != "" {
			return rejected(code), nil
		}

		if err := tx.PutInventory(updated); err != nil {
			return OperationResult{}, err
		}
		return OperationResult{Status: OperationCompleted, Consumed: consumed, Produced: []ItemAmount{produced}}, nil
	})
}

func (e *Engine) resolveMaterials(ctx context.Context, actor Actor, characterID CharacterID, inventory Inventory, recipe Recipe) (Recipe, bool, error) {
	if len(recipe.MaterialSelectors) == 0 {
		return recipe, true, nil
	}
	if len(recipe.Materials) != 0 {
		return Recipe{}, false, nil
	}
	resolved := make(map[ItemID]int64)
	for _, selector := range recipe.MaterialSelectors {
		if selector.Quantity <= 0 {
			return Recipe{}, false, nil
		}
		if selector.ItemID != "" {
			total, ok := checkedAdd(resolved[selector.ItemID], selector.Quantity)
			if !ok {
				return Recipe{}, false, nil
			}
			resolved[selector.ItemID] = total
			continue
		}
		if selector.Category == "" || e.options.MaterialResolver == nil {
			return Recipe{}, false, nil
		}
		allocations, err := e.options.MaterialResolver.ResolveCategory(ctx, actor, characterID, selector.Category, selector.Quantity, cloneInventory(inventory))
		if err != nil {
			return Recipe{}, false, err
		}
		if len(allocations) == 0 {
			return Recipe{}, false, nil
		}
		var allocated int64
		for _, allocation := range allocations {
			if !validID(string(allocation.ItemID)) || allocation.Quantity <= 0 {
				return Recipe{}, false, nil
			}
			var valid bool
			allocated, valid = checkedAdd(allocated, allocation.Quantity)
			if !valid {
				return Recipe{}, false, nil
			}
			total, valid := checkedAdd(resolved[allocation.ItemID], allocation.Quantity)
			if !valid {
				return Recipe{}, false, nil
			}
			resolved[allocation.ItemID] = total
		}
		if allocated != selector.Quantity {
			return Recipe{}, false, nil
		}
	}
	recipe.Materials = make([]ItemAmount, 0, len(resolved))
	for itemID, quantity := range resolved {
		recipe.Materials = append(recipe.Materials, ItemAmount{ItemID: itemID, Quantity: quantity})
	}
	recipe.MaterialSelectors = nil
	return recipe, true, nil
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
		definition, ok, err := tx.ReadItemDefinition(request.Item.ItemID)
		if err != nil {
			return OperationResult{}, err
		}
		if !ok {
			return rejected(CodeItemNotFound), nil
		}
		inventory, ok, err := tx.ReadInventory(request.CharacterID)
		if err != nil {
			return OperationResult{}, err
		}
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
		if _, ok, err := tx.ReadItemDefinition(request.Item.ItemID); err != nil {
			return OperationResult{}, err
		} else if !ok {
			return rejected(CodeItemNotFound), nil
		}
		inventory, ok, err := tx.ReadInventory(request.CharacterID)
		if err != nil {
			return OperationResult{}, err
		}
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
		definition, ok, err := tx.ReadItemDefinition(request.Item.ItemID)
		if err != nil {
			return OperationResult{}, err
		}
		if !ok {
			return rejected(CodeItemNotFound), nil
		}
		inventories, err := tx.ReadInventories(request.Source, request.Destination)
		if err != nil {
			return OperationResult{}, err
		}
		source, sourceOK := inventories[request.Source]
		destination, destinationOK := inventories[request.Destination]
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
		var readErr error
		inventory, ok, readErr = tx.ReadInventory(characterID)
		if readErr != nil {
			return readErr
		}
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
		previous, exists, readErr := tx.ReadOperation(scope)
		if readErr != nil {
			return readErr
		}
		if exists {
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
