package core

import (
	"context"
	"fmt"
	"sync"
)

type Transaction interface {
	ItemDefinition(ItemID) (ItemDefinition, bool)
	Recipe(RecipeID, uint64) (Recipe, bool)
	Inventory(CharacterID) (Inventory, bool)
	Operation(OperationScope) (OperationRecord, bool)
	ReadItemDefinition(ItemID) (ItemDefinition, bool, error)
	ReadRecipe(RecipeID, uint64) (Recipe, bool, error)
	ReadInventory(CharacterID) (Inventory, bool, error)
	ReadInventories(...CharacterID) (map[CharacterID]Inventory, error)
	ReadOperation(OperationScope) (OperationRecord, bool, error)
	PutInventory(Inventory) error
	PutOperation(OperationRecord) error
}

type Repository interface {
	Transact(context.Context, func(Transaction) error) error
}

type scopeKey struct {
	character CharacterID
	typeName  OperationType
	key       IdempotencyKey
}

type recipeKey struct {
	id      RecipeID
	version uint64
}

type MemoryStore struct {
	mu            sync.Mutex
	definitions   map[ItemID]ItemDefinition
	recipes       map[recipeKey]Recipe
	inventories   map[CharacterID]Inventory
	operations    map[scopeKey]OperationRecord
	maxOperations int
	failures      map[string]int
}

func NewMemoryStore(maxOperations int) *MemoryStore {
	if maxOperations <= 0 {
		maxOperations = 10000
	}
	return &MemoryStore{
		definitions:   make(map[ItemID]ItemDefinition),
		recipes:       make(map[recipeKey]Recipe),
		inventories:   make(map[CharacterID]Inventory),
		operations:    make(map[scopeKey]OperationRecord),
		maxOperations: maxOperations,
		failures:      make(map[string]int),
	}
}

func (s *MemoryStore) SeedItemDefinition(definition ItemDefinition) error {
	if !validDefinition(definition) {
		return ErrInvalidRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.definitions[definition.ID] = definition
	return nil
}

func (s *MemoryStore) SeedRecipe(recipe Recipe) error {
	if !validRecipe(recipe) {
		return ErrInvalidRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, amount := range recipe.Materials {
		if _, ok := s.definitions[amount.ItemID]; !ok {
			return ErrNotFound
		}
	}
	if _, ok := s.definitions[recipe.Product.ItemID]; !ok {
		return ErrNotFound
	}
	s.recipes[recipeKey{id: recipe.ID, version: recipe.Version}] = cloneRecipe(recipe)
	return nil
}

func (s *MemoryStore) SeedInventory(inventory Inventory) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ValidateInventory(inventory, s.definitions); err != nil {
		return err
	}
	s.inventories[inventory.CharacterID] = cloneInventory(inventory)
	return nil
}

func (s *MemoryStore) FailNext(point string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failures[point]++
}

func (s *MemoryStore) LookupOperation(scope OperationScope) (OperationRecord, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.operations[toScopeKey(scope)]
	return cloneOperationRecord(record), ok
}

func (s *MemoryStore) Snapshot(characterID CharacterID) (Inventory, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	inventory, ok := s.inventories[characterID]
	return cloneInventory(inventory), ok
}

func (s *MemoryStore) Transact(ctx context.Context, apply func(Transaction) error) error {
	if ctx == nil || apply == nil {
		return ErrInvalidRequest
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	tx := &memoryTransaction{
		store:       s,
		ctx:         ctx,
		definitions: cloneDefinitions(s.definitions),
		recipes:     cloneRecipes(s.recipes),
		inventories: cloneInventories(s.inventories),
		operations:  cloneOperations(s.operations),
	}
	if err := apply(tx); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := s.failLocked("commit"); err != nil {
		return err
	}
	s.definitions = tx.definitions
	s.recipes = tx.recipes
	s.inventories = tx.inventories
	s.operations = tx.operations
	return nil
}

func (s *MemoryStore) failLocked(point string) error {
	if s.failures[point] == 0 {
		return nil
	}
	s.failures[point]--
	return fmt.Errorf("%w: injected failure at %s", ErrStorage, point)
}

type memoryTransaction struct {
	store       *MemoryStore
	ctx         context.Context
	definitions map[ItemID]ItemDefinition
	recipes     map[recipeKey]Recipe
	inventories map[CharacterID]Inventory
	operations  map[scopeKey]OperationRecord
}

func (tx *memoryTransaction) ReadItemDefinition(id ItemID) (ItemDefinition, bool, error) {
	if err := tx.ctx.Err(); err != nil {
		return ItemDefinition{}, false, err
	}
	definition, ok := tx.definitions[id]
	return definition, ok, nil
}

func (tx *memoryTransaction) ReadRecipe(id RecipeID, version uint64) (Recipe, bool, error) {
	if err := tx.ctx.Err(); err != nil {
		return Recipe{}, false, err
	}
	recipe, ok := tx.recipes[recipeKey{id: id, version: version}]
	return cloneRecipe(recipe), ok, nil
}

func (tx *memoryTransaction) ReadInventory(id CharacterID) (Inventory, bool, error) {
	inventories, err := tx.ReadInventories(id)
	if err != nil {
		return Inventory{}, false, err
	}
	inventory, ok := inventories[id]
	return inventory, ok, nil
}

func (tx *memoryTransaction) ReadInventories(ids ...CharacterID) (map[CharacterID]Inventory, error) {
	if err := tx.ctx.Err(); err != nil {
		return nil, err
	}
	inventories := make(map[CharacterID]Inventory, len(ids))
	for _, id := range ids {
		if inventory, ok := tx.inventories[id]; ok {
			inventories[id] = cloneInventory(inventory)
		}
	}
	return inventories, nil
}

func (tx *memoryTransaction) ReadOperation(scope OperationScope) (OperationRecord, bool, error) {
	if err := tx.ctx.Err(); err != nil {
		return OperationRecord{}, false, err
	}
	record, ok := tx.operations[toScopeKey(scope)]
	return cloneOperationRecord(record), ok, nil
}

func (tx *memoryTransaction) ItemDefinition(id ItemID) (ItemDefinition, bool) {
	definition, ok, _ := tx.ReadItemDefinition(id)
	return definition, ok
}

func (tx *memoryTransaction) Recipe(id RecipeID, version uint64) (Recipe, bool) {
	recipe, ok, _ := tx.ReadRecipe(id, version)
	return recipe, ok
}

func (tx *memoryTransaction) Inventory(id CharacterID) (Inventory, bool) {
	inventory, ok, _ := tx.ReadInventory(id)
	return inventory, ok
}

func (tx *memoryTransaction) Operation(scope OperationScope) (OperationRecord, bool) {
	record, ok, _ := tx.ReadOperation(scope)
	return record, ok
}

func (tx *memoryTransaction) PutInventory(inventory Inventory) error {
	if err := ValidateInventory(inventory, tx.definitions); err != nil {
		return err
	}
	tx.inventories[inventory.CharacterID] = cloneInventory(inventory)
	return tx.store.failLocked("inventory_write")
}

func (tx *memoryTransaction) PutOperation(record OperationRecord) error {
	if err := ValidateOperationRecord(record); err != nil {
		return err
	}
	key := toScopeKey(record.Scope)
	if _, exists := tx.operations[key]; exists {
		return ErrIdempotencyConflict
	}
	if len(tx.operations) >= tx.store.maxOperations {
		return ErrCapacity
	}
	tx.operations[key] = cloneOperationRecord(record)
	return tx.store.failLocked("operation_write")
}

func toScopeKey(scope OperationScope) scopeKey {
	return scopeKey{character: scope.CharacterID, typeName: scope.Type, key: scope.Key}
}

func validOperationScope(scope OperationScope) bool {
	validType := scope.Type == OperationCraft || scope.Type == OperationAdd || scope.Type == OperationRemove || scope.Type == OperationTransfer
	return validID(string(scope.CharacterID)) && validID(string(scope.Key)) && validType
}

func ValidateOperationRecord(record OperationRecord) error {
	if !validOperationScope(record.Scope) ||
		record.Fingerprint == "" ||
		!validRequestID(record.RequestID) ||
		!validID(record.Result.OperationID) ||
		record.Result.RequestID != record.RequestID ||
		(record.Result.Status != OperationCompleted && record.Result.Status != OperationRejected) {
		return ErrInvalidRequest
	}
	return nil
}

func cloneDefinitions(source map[ItemID]ItemDefinition) map[ItemID]ItemDefinition {
	clone := make(map[ItemID]ItemDefinition, len(source))
	for key, value := range source {
		clone[key] = value
	}
	return clone
}

func cloneRecipes(source map[recipeKey]Recipe) map[recipeKey]Recipe {
	clone := make(map[recipeKey]Recipe, len(source))
	for key, value := range source {
		clone[key] = cloneRecipe(value)
	}
	return clone
}

func cloneInventories(source map[CharacterID]Inventory) map[CharacterID]Inventory {
	clone := make(map[CharacterID]Inventory, len(source))
	for key, value := range source {
		clone[key] = cloneInventory(value)
	}
	return clone
}

func cloneOperations(source map[scopeKey]OperationRecord) map[scopeKey]OperationRecord {
	clone := make(map[scopeKey]OperationRecord, len(source))
	for key, value := range source {
		clone[key] = cloneOperationRecord(value)
	}
	return clone
}

func cloneRecipe(recipe Recipe) Recipe {
	recipe.Materials = append([]ItemAmount(nil), recipe.Materials...)
	recipe.Requirements = append([]RecipeRequirement(nil), recipe.Requirements...)
	return recipe
}

func cloneOperationRecord(record OperationRecord) OperationRecord {
	record.Result = cloneOperationResult(record.Result)
	return record
}

func cloneOperationResult(result OperationResult) OperationResult {
	result.Consumed = append([]ItemAmount(nil), result.Consumed...)
	result.Produced = append([]ItemAmount(nil), result.Produced...)
	return result
}
