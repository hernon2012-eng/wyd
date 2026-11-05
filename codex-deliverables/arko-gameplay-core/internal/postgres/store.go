package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"arko-gameplay-core/internal/core"
)

const (
	defaultMaxOperations = 10000
)

type Store struct {
	db            *sql.DB
	maxOperations int
}

func NewStore(db *sql.DB, maxOperations int) *Store {
	if maxOperations <= 0 {
		maxOperations = defaultMaxOperations
	}
	return &Store{db: db, maxOperations: maxOperations}
}

func (s *Store) Migrate(ctx context.Context) error {
	if s == nil || s.db == nil {
		return core.ErrInvalidRequest
	}
	statements := []string{
		`CREATE TABLE IF NOT EXISTS gameplay_items (
			item_id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			kind TEXT NOT NULL,
			max_stack BIGINT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS gameplay_recipes (
			recipe_id TEXT NOT NULL,
			recipe_version BIGINT NOT NULL,
			name TEXT NOT NULL,
			materials JSONB NOT NULL DEFAULT '[]'::jsonb,
			product JSONB NOT NULL,
			requirements JSONB NOT NULL DEFAULT '[]'::jsonb,
			active BOOLEAN NOT NULL DEFAULT FALSE,
			demo_only BOOLEAN NOT NULL DEFAULT FALSE,
			PRIMARY KEY (recipe_id, recipe_version)
		)`,
		`CREATE TABLE IF NOT EXISTS gameplay_inventories (
			character_id TEXT PRIMARY KEY,
			slots JSONB NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS gameplay_operations (
			character_id TEXT NOT NULL,
			operation_type TEXT NOT NULL,
			idempotency_key TEXT NOT NULL,
			request_id TEXT NOT NULL,
			fingerprint TEXT NOT NULL,
			operation_id TEXT NOT NULL,
			result JSONB NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			PRIMARY KEY (character_id, operation_type, idempotency_key)
		)`,
	}
	for _, statement := range statements {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("%w: %s", core.ErrStorage, err)
		}
	}
	return nil
}

func (s *Store) SeedItemDefinition(ctx context.Context, definition core.ItemDefinition) error {
	if s == nil || s.db == nil {
		return core.ErrInvalidRequest
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO gameplay_items (item_id, name, kind, max_stack) VALUES ($1, $2, $3, $4)
		 ON CONFLICT (item_id) DO UPDATE SET name = EXCLUDED.name, kind = EXCLUDED.kind, max_stack = EXCLUDED.max_stack`,
		definition.ID,
		definition.Name,
		definition.Kind,
		definition.MaxStack,
	)
	if err != nil {
		return fmt.Errorf("%w: seed item definition: %v", core.ErrStorage, err)
	}
	return nil
}

func (s *Store) SeedRecipe(ctx context.Context, recipe core.Recipe) error {
	if s == nil || s.db == nil {
		return core.ErrInvalidRequest
	}
	materials, err := json.Marshal(recipe.Materials)
	if err != nil {
		return fmt.Errorf("%w: marshal recipe materials: %v", core.ErrStorage, err)
	}
	product, err := json.Marshal(recipe.Product)
	if err != nil {
		return fmt.Errorf("%w: marshal recipe product: %v", core.ErrStorage, err)
	}
	requirements, err := json.Marshal(recipe.Requirements)
	if err != nil {
		return fmt.Errorf("%w: marshal recipe requirements: %v", core.ErrStorage, err)
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO gameplay_recipes (recipe_id, recipe_version, name, materials, product, requirements, active, demo_only)
		 VALUES ($1, $2, $3, $4::jsonb, $5::jsonb, $6::jsonb, $7, $8)
		 ON CONFLICT (recipe_id, recipe_version) DO UPDATE SET name = EXCLUDED.name, materials = EXCLUDED.materials, product = EXCLUDED.product, requirements = EXCLUDED.requirements, active = EXCLUDED.active, demo_only = EXCLUDED.demo_only`,
		recipe.ID,
		recipe.Version,
		recipe.Name,
		materials,
		product,
		requirements,
		recipe.Active,
		recipe.DemoOnly,
	)
	if err != nil {
		return fmt.Errorf("%w: seed recipe: %v", core.ErrStorage, err)
	}
	return nil
}

func (s *Store) SeedInventory(ctx context.Context, inventory core.Inventory) error {
	if s == nil || s.db == nil {
		return core.ErrInvalidRequest
	}
	slots, err := json.Marshal(inventory.Slots)
	if err != nil {
		return fmt.Errorf("%w: marshal inventory slots: %v", core.ErrStorage, err)
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO gameplay_inventories (character_id, slots) VALUES ($1, $2::jsonb)
		 ON CONFLICT (character_id) DO UPDATE SET slots = EXCLUDED.slots`,
		inventory.CharacterID,
		slots,
	)
	if err != nil {
		return fmt.Errorf("%w: seed inventory: %v", core.ErrStorage, err)
	}
	return nil
}

func (s *Store) Transact(ctx context.Context, apply func(core.Transaction) error) error {
	if s == nil || s.db == nil || apply == nil || ctx == nil {
		return core.ErrInvalidRequest
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return err
	}
	adapter := &sqlTransaction{tx: tx, maxOperations: s.maxOperations}
	if err := apply(adapter); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := ctx.Err(); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

type sqlTransaction struct {
	tx            *sql.Tx
	maxOperations int
}

func (tx *sqlTransaction) ItemDefinition(id core.ItemID) (core.ItemDefinition, bool) {
	var definition core.ItemDefinition
	var kind string
	err := tx.tx.QueryRowContext(context.Background(),
		`SELECT item_id, name, kind, max_stack FROM gameplay_items WHERE item_id = $1`,
		id,
	).Scan(&definition.ID, &definition.Name, &kind, &definition.MaxStack)
	if err != nil {
		if err == sql.ErrNoRows {
			return core.ItemDefinition{}, false
		}
		return core.ItemDefinition{}, false
	}
	definition.Kind = core.ItemKind(kind)
	return definition, true
}

func (tx *sqlTransaction) Recipe(id core.RecipeID, version uint64) (core.Recipe, bool) {
	var recipe core.Recipe
	var materials, product, requirements []byte
	var active, demoOnly bool
	err := tx.tx.QueryRowContext(context.Background(),
		`SELECT recipe_id, recipe_version, name, materials, product, requirements, active, demo_only FROM gameplay_recipes WHERE recipe_id = $1 AND recipe_version = $2`,
		id,
		version,
	).Scan(&recipe.ID, &recipe.Version, &recipe.Name, &materials, &product, &requirements, &active, &demoOnly)
	if err != nil {
		if err == sql.ErrNoRows {
			return core.Recipe{}, false
		}
		return core.Recipe{}, false
	}
	if len(materials) > 0 {
		if err := json.Unmarshal(materials, &recipe.Materials); err != nil {
			return core.Recipe{}, false
		}
	}
	if len(product) > 0 {
		if err := json.Unmarshal(product, &recipe.Product); err != nil {
			return core.Recipe{}, false
		}
	}
	if len(requirements) > 0 {
		if err := json.Unmarshal(requirements, &recipe.Requirements); err != nil {
			return core.Recipe{}, false
		}
	}
	recipe.Active = active
	recipe.DemoOnly = demoOnly
	return recipe, true
}

func (tx *sqlTransaction) Inventory(id core.CharacterID) (core.Inventory, bool) {
	var inventory core.Inventory
	var slotsJSON []byte
	err := tx.tx.QueryRowContext(context.Background(),
		`SELECT character_id, slots FROM gameplay_inventories WHERE character_id = $1`,
		id,
	).Scan(&inventory.CharacterID, &slotsJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			return core.Inventory{}, false
		}
		return core.Inventory{}, false
	}
	if len(slotsJSON) > 0 {
		if err := json.Unmarshal(slotsJSON, &inventory.Slots); err != nil {
			return core.Inventory{}, false
		}
	}
	return inventory, true
}

func (tx *sqlTransaction) Operation(scope core.OperationScope) (core.OperationRecord, bool) {
	var record core.OperationRecord
	var resultJSON []byte
	var createdAt, updatedAt time.Time
	err := tx.tx.QueryRowContext(context.Background(),
		`SELECT character_id, operation_type, idempotency_key, request_id, fingerprint, operation_id, result, created_at, updated_at
		 FROM gameplay_operations WHERE character_id = $1 AND operation_type = $2 AND idempotency_key = $3`,
		scope.CharacterID,
		scope.Type,
		scope.Key,
	).Scan(
		&record.Scope.CharacterID,
		&record.Scope.Type,
		&record.Scope.Key,
		&record.RequestID,
		&record.Fingerprint,
		&record.Result.OperationID,
		&resultJSON,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return core.OperationRecord{}, false
		}
		return core.OperationRecord{}, false
	}
	record.Scope = scope
	record.CreatedAt = createdAt
	record.UpdatedAt = updatedAt
	if len(resultJSON) > 0 {
		if err := json.Unmarshal(resultJSON, &record.Result); err != nil {
			return core.OperationRecord{}, false
		}
	}
	return record, true
}

func (tx *sqlTransaction) PutInventory(inventory core.Inventory) error {
	if inventory.CharacterID == "" {
		return core.ErrInvalidRequest
	}
	slotsJSON, err := json.Marshal(inventory.Slots)
	if err != nil {
		return fmt.Errorf("%w: marshal inventory slots: %v", core.ErrStorage, err)
	}
	_, err = tx.tx.ExecContext(context.Background(),
		`INSERT INTO gameplay_inventories (character_id, slots) VALUES ($1, $2::jsonb)
		 ON CONFLICT (character_id) DO UPDATE SET slots = EXCLUDED.slots`,
		inventory.CharacterID,
		slotsJSON,
	)
	if err != nil {
		return fmt.Errorf("%w: put inventory: %v", core.ErrStorage, err)
	}
	return nil
}

func (tx *sqlTransaction) PutOperation(record core.OperationRecord) error {
	if record.Scope.CharacterID == "" || record.Scope.Type == "" || record.Scope.Key == "" {
		return core.ErrInvalidRequest
	}
	resultJSON, err := json.Marshal(record.Result)
	if err != nil {
		return fmt.Errorf("%w: marshal operation result: %v", core.ErrStorage, err)
	}
	_, err = tx.tx.ExecContext(context.Background(),
		`INSERT INTO gameplay_operations (character_id, operation_type, idempotency_key, request_id, fingerprint, operation_id, result, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, $8, $9)
		 ON CONFLICT (character_id, operation_type, idempotency_key) DO UPDATE SET
			request_id = EXCLUDED.request_id,
			fingerprint = EXCLUDED.fingerprint,
			operation_id = EXCLUDED.operation_id,
			result = EXCLUDED.result,
			updated_at = EXCLUDED.updated_at`,
		record.Scope.CharacterID,
		record.Scope.Type,
		record.Scope.Key,
		record.RequestID,
		record.Fingerprint,
		record.Result.OperationID,
		resultJSON,
		record.CreatedAt,
		record.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("%w: put operation: %v", core.ErrStorage, err)
	}
	return nil
}
