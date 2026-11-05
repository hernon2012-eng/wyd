package postgres

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"arko-gameplay-core/internal/core"
	_ "github.com/jackc/pgx/v5/stdlib"
)

const (
	defaultMaxOperations = 10000
	migrationVersion     = 1
	migrationName        = "001_gameplay.sql"
)

//go:embed migrations/001_gameplay.sql
var migrationFiles embed.FS

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

// Migrate explicitly applies the embedded, versioned ARKO gameplay migrations.
// Constructing a Store or beginning a transaction never changes the schema.
func (s *Store) Migrate(ctx context.Context) error {
	if s == nil || s.db == nil || ctx == nil {
		return core.ErrInvalidRequest
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return databaseError(ctx, "begin migrations", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS arko_gameplay_schema_migrations (
		version BIGINT PRIMARY KEY,
		name TEXT NOT NULL,
		applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`); err != nil {
		return databaseError(ctx, "create migration ledger", err)
	}
	var migrationLock any
	if err := tx.QueryRowContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, "arko-gameplay-core:migrations").Scan(&migrationLock); err != nil {
		return databaseError(ctx, "lock migrations", err)
	}
	var currentVersion int64
	err = tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) FROM arko_gameplay_schema_migrations`).Scan(&currentVersion)
	if err != nil {
		return databaseError(ctx, "read migration version", err)
	}
	if currentVersion > migrationVersion {
		return fmt.Errorf("%w: database schema version %d is newer than supported version %d", core.ErrStorage, currentVersion, migrationVersion)
	}
	if currentVersion == migrationVersion {
		return databaseError(ctx, "commit migrations", tx.Commit())
	}

	migration, err := migrationFiles.ReadFile("migrations/" + migrationName)
	if err != nil {
		return fmt.Errorf("%w: read migration %s: %v", core.ErrStorage, migrationName, err)
	}
	for _, statement := range strings.Split(string(migration), ";") {
		if strings.TrimSpace(statement) == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return databaseError(ctx, "apply migration "+migrationName, err)
		}
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO arko_gameplay_schema_migrations (version, name) VALUES ($1, $2)`,
		migrationVersion, migrationName,
	); err != nil {
		return databaseError(ctx, "record migration "+migrationName, err)
	}
	return databaseError(ctx, "commit migrations", tx.Commit())
}

func (s *Store) SeedItemDefinition(ctx context.Context, definition core.ItemDefinition) error {
	if s == nil || s.db == nil || ctx == nil {
		return core.ErrInvalidRequest
	}
	if err := core.ValidateDefinition(definition); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO arko_gameplay_items (item_id, name, kind, max_stack) VALUES ($1, $2, $3, $4)
		 ON CONFLICT (item_id) DO UPDATE SET name = EXCLUDED.name, kind = EXCLUDED.kind, max_stack = EXCLUDED.max_stack`,
		definition.ID, definition.Name, definition.Kind, definition.MaxStack,
	)
	if err != nil {
		return databaseError(ctx, "seed item definition", err)
	}
	return nil
}

func (s *Store) SeedRecipe(ctx context.Context, recipe core.Recipe) error {
	if s == nil || s.db == nil || ctx == nil {
		return core.ErrInvalidRequest
	}
	if err := core.ValidateRecipe(recipe); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return databaseError(ctx, "begin recipe seed", err)
	}
	defer tx.Rollback()
	for _, itemID := range recipeItemIDs(recipe) {
		if _, found, readErr := readDefinition(ctx, tx, itemID); readErr != nil {
			return readErr
		} else if !found {
			return core.ErrNotFound
		}
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
	_, err = tx.ExecContext(ctx,
		`INSERT INTO arko_gameplay_recipes (recipe_id, recipe_version, name, materials, product, requirements, active, demo_only)
		 VALUES ($1, $2, $3, $4::jsonb, $5::jsonb, $6::jsonb, $7, $8)
		 ON CONFLICT (recipe_id, recipe_version) DO UPDATE SET name = EXCLUDED.name, materials = EXCLUDED.materials, product = EXCLUDED.product, requirements = EXCLUDED.requirements, active = EXCLUDED.active, demo_only = EXCLUDED.demo_only`,
		recipe.ID, recipe.Version, recipe.Name, materials, product, requirements, recipe.Active, recipe.DemoOnly,
	)
	if err != nil {
		return databaseError(ctx, "seed recipe", err)
	}
	return databaseError(ctx, "commit recipe seed", tx.Commit())
}

func (s *Store) SeedInventory(ctx context.Context, inventory core.Inventory) error {
	if s == nil || s.db == nil || ctx == nil {
		return core.ErrInvalidRequest
	}
	if !validInventoryShape(inventory) {
		return core.ErrInvalidRequest
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return databaseError(ctx, "begin inventory seed", err)
	}
	defer tx.Rollback()
	definitions, err := loadDefinitions(ctx, tx, inventory)
	if err != nil {
		return err
	}
	if err := core.ValidateInventory(inventory, definitions); err != nil {
		return err
	}
	slots, err := json.Marshal(inventory.Slots)
	if err != nil {
		return fmt.Errorf("%w: marshal inventory slots: %v", core.ErrStorage, err)
	}
	_, err = tx.ExecContext(ctx,
		`INSERT INTO arko_gameplay_inventories (character_id, slots) VALUES ($1, $2::jsonb)
		 ON CONFLICT (character_id) DO UPDATE SET slots = EXCLUDED.slots`,
		inventory.CharacterID, slots,
	)
	if err != nil {
		return databaseError(ctx, "seed inventory", err)
	}
	return databaseError(ctx, "commit inventory seed", tx.Commit())
}

func (s *Store) Transact(ctx context.Context, apply func(core.Transaction) error) error {
	if s == nil || s.db == nil || apply == nil || ctx == nil {
		return core.ErrInvalidRequest
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return databaseError(ctx, "begin transaction", err)
	}
	adapter := &sqlTransaction{tx: tx, ctx: ctx, maxOperations: s.maxOperations}
	if err := apply(adapter); err != nil {
		_ = tx.Rollback()
		if adapter.readErr != nil {
			return adapter.readErr
		}
		return err
	}
	if adapter.readErr != nil {
		_ = tx.Rollback()
		return adapter.readErr
	}
	if err := ctx.Err(); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return databaseError(ctx, "commit transaction", err)
	}
	return nil
}

type sqlTransaction struct {
	tx            *sql.Tx
	ctx           context.Context
	maxOperations int
	readErr       error
}

func (tx *sqlTransaction) ItemDefinition(id core.ItemID) (core.ItemDefinition, bool) {
	value, found, err := tx.ReadItemDefinition(id)
	tx.rememberReadError(err)
	return value, found
}

func (tx *sqlTransaction) Recipe(id core.RecipeID, version uint64) (core.Recipe, bool) {
	value, found, err := tx.ReadRecipe(id, version)
	tx.rememberReadError(err)
	return value, found
}

func (tx *sqlTransaction) Inventory(id core.CharacterID) (core.Inventory, bool) {
	value, found, err := tx.ReadInventory(id)
	tx.rememberReadError(err)
	return value, found
}

func (tx *sqlTransaction) Operation(scope core.OperationScope) (core.OperationRecord, bool) {
	value, found, err := tx.ReadOperation(scope)
	tx.rememberReadError(err)
	return value, found
}

func (tx *sqlTransaction) rememberReadError(err error) {
	if err != nil && tx.readErr == nil {
		tx.readErr = err
	}
}

func (tx *sqlTransaction) ReadItemDefinition(id core.ItemID) (core.ItemDefinition, bool, error) {
	return readDefinition(tx.ctx, tx.tx, id)
}

func (tx *sqlTransaction) ReadRecipe(id core.RecipeID, version uint64) (core.Recipe, bool, error) {
	if err := tx.ctx.Err(); err != nil {
		return core.Recipe{}, false, err
	}
	var recipe core.Recipe
	var materials, product, requirements []byte
	err := tx.tx.QueryRowContext(tx.ctx,
		`SELECT recipe_id, recipe_version, name, materials, product, requirements, active, demo_only
		 FROM arko_gameplay_recipes WHERE recipe_id = $1 AND recipe_version = $2`,
		id, version,
	).Scan(&recipe.ID, &recipe.Version, &recipe.Name, &materials, &product, &requirements, &recipe.Active, &recipe.DemoOnly)
	if errors.Is(err, sql.ErrNoRows) {
		return core.Recipe{}, false, nil
	}
	if err != nil {
		return core.Recipe{}, false, databaseError(tx.ctx, "read recipe", err)
	}
	if err := json.Unmarshal(materials, &recipe.Materials); err != nil {
		return core.Recipe{}, false, dataError("decode recipe materials", err)
	}
	if err := json.Unmarshal(product, &recipe.Product); err != nil {
		return core.Recipe{}, false, dataError("decode recipe product", err)
	}
	if err := json.Unmarshal(requirements, &recipe.Requirements); err != nil {
		return core.Recipe{}, false, dataError("decode recipe requirements", err)
	}
	if err := core.ValidateRecipe(recipe); err != nil {
		return core.Recipe{}, false, dataError("validate recipe", err)
	}
	for _, itemID := range recipeItemIDs(recipe) {
		if _, found, err := readDefinition(tx.ctx, tx.tx, itemID); err != nil {
			return core.Recipe{}, false, err
		} else if !found {
			return core.Recipe{}, false, dataError("validate recipe item definition", core.ErrNotFound)
		}
	}
	return recipe, true, nil
}

func (tx *sqlTransaction) ReadInventory(id core.CharacterID) (core.Inventory, bool, error) {
	inventories, err := tx.ReadInventories(id)
	if err != nil {
		return core.Inventory{}, false, err
	}
	inventory, found := inventories[id]
	return inventory, found, nil
}

func (tx *sqlTransaction) ReadInventories(ids ...core.CharacterID) (map[core.CharacterID]core.Inventory, error) {
	if err := tx.ctx.Err(); err != nil {
		return nil, err
	}
	uniqueIDs := make([]string, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if _, found := seen[string(id)]; found {
			continue
		}
		seen[string(id)] = struct{}{}
		uniqueIDs = append(uniqueIDs, string(id))
	}
	sort.Strings(uniqueIDs)
	inventories := make(map[core.CharacterID]core.Inventory, len(uniqueIDs))
	if len(uniqueIDs) == 0 {
		return inventories, nil
	}
	placeholders := make([]string, len(uniqueIDs))
	args := make([]any, len(uniqueIDs))
	for index, id := range uniqueIDs {
		placeholders[index] = fmt.Sprintf("$%d", index+1)
		args[index] = id
	}
	query := `SELECT character_id, slots FROM arko_gameplay_inventories WHERE character_id IN (` +
		strings.Join(placeholders, ", ") + `) ORDER BY character_id FOR UPDATE`
	rows, err := tx.tx.QueryContext(tx.ctx, query, args...)
	if err != nil {
		return nil, databaseError(tx.ctx, "lock inventories", err)
	}
	defer rows.Close()
	for rows.Next() {
		var inventory core.Inventory
		var slots []byte
		if err := rows.Scan(&inventory.CharacterID, &slots); err != nil {
			return nil, databaseError(tx.ctx, "scan inventory", err)
		}
		if err := json.Unmarshal(slots, &inventory.Slots); err != nil {
			return nil, dataError("decode inventory", err)
		}
		inventories[inventory.CharacterID] = inventory
	}
	if err := rows.Err(); err != nil {
		return nil, databaseError(tx.ctx, "read inventories", err)
	}
	if err := rows.Close(); err != nil {
		return nil, databaseError(tx.ctx, "close inventory rows", err)
	}
	for _, inventory := range inventories {
		definitions, err := loadDefinitions(tx.ctx, tx.tx, inventory)
		if err != nil {
			return nil, err
		}
		if err := core.ValidateInventory(inventory, definitions); err != nil {
			return nil, dataError("validate inventory", err)
		}
	}
	return inventories, nil
}

func (tx *sqlTransaction) ReadOperation(scope core.OperationScope) (core.OperationRecord, bool, error) {
	if err := tx.ctx.Err(); err != nil {
		return core.OperationRecord{}, false, err
	}
	if err := core.ValidateOperationRecord(core.OperationRecord{
		Scope: scope, Fingerprint: "validation", RequestID: "validation",
		Result: core.OperationResult{OperationID: "validation", RequestID: "validation", Status: core.OperationCompleted},
	}); err != nil {
		return core.OperationRecord{}, false, err
	}
	lockKey := fmt.Sprintf("%d:%s:%d:%s:%d:%s",
		len(scope.CharacterID), scope.CharacterID, len(scope.Type), scope.Type, len(scope.Key), scope.Key)
	var operationLock any
	if err := tx.tx.QueryRowContext(tx.ctx,
		`SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, lockKey,
	).Scan(&operationLock); err != nil {
		return core.OperationRecord{}, false, databaseError(tx.ctx, "lock operation key", err)
	}
	var record core.OperationRecord
	var resultJSON []byte
	var createdAt, updatedAt time.Time
	err := tx.tx.QueryRowContext(tx.ctx,
		`SELECT character_id, operation_type, idempotency_key, request_id, fingerprint, operation_id, result, created_at, updated_at
		 FROM arko_gameplay_operations WHERE character_id = $1 AND operation_type = $2 AND idempotency_key = $3 FOR UPDATE`,
		scope.CharacterID, scope.Type, scope.Key,
	).Scan(
		&record.Scope.CharacterID, &record.Scope.Type, &record.Scope.Key,
		&record.RequestID, &record.Fingerprint, &record.Result.OperationID,
		&resultJSON, &createdAt, &updatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return core.OperationRecord{}, false, nil
	}
	if err != nil {
		return core.OperationRecord{}, false, databaseError(tx.ctx, "read operation", err)
	}
	record.CreatedAt = createdAt
	record.UpdatedAt = updatedAt
	if err := json.Unmarshal(resultJSON, &record.Result); err != nil {
		return core.OperationRecord{}, false, dataError("decode operation result", err)
	}
	if err := core.ValidateOperationRecord(record); err != nil {
		return core.OperationRecord{}, false, dataError("validate operation record", err)
	}
	return record, true, nil
}

func (tx *sqlTransaction) PutInventory(inventory core.Inventory) error {
	if !validInventoryShape(inventory) {
		return core.ErrInvalidRequest
	}
	definitions, err := loadDefinitions(tx.ctx, tx.tx, inventory)
	if err != nil {
		return err
	}
	if err := core.ValidateInventory(inventory, definitions); err != nil {
		return err
	}
	slotsJSON, err := json.Marshal(inventory.Slots)
	if err != nil {
		return fmt.Errorf("%w: marshal inventory slots: %v", core.ErrStorage, err)
	}
	_, err = tx.tx.ExecContext(tx.ctx,
		`INSERT INTO arko_gameplay_inventories (character_id, slots) VALUES ($1, $2::jsonb)
		 ON CONFLICT (character_id) DO UPDATE SET slots = EXCLUDED.slots`,
		inventory.CharacterID, slotsJSON,
	)
	if err != nil {
		return databaseError(tx.ctx, "put inventory", err)
	}
	return nil
}

func (tx *sqlTransaction) PutOperation(record core.OperationRecord) error {
	if err := core.ValidateOperationRecord(record); err != nil {
		return err
	}
	resultJSON, err := json.Marshal(record.Result)
	if err != nil {
		return fmt.Errorf("%w: marshal operation result: %v", core.ErrStorage, err)
	}
	var capacityLock any
	if err := tx.tx.QueryRowContext(tx.ctx,
		`SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, "arko-gameplay-core:operation-capacity",
	).Scan(&capacityLock); err != nil {
		return databaseError(tx.ctx, "lock operation capacity", err)
	}
	var operationCount int
	if err := tx.tx.QueryRowContext(tx.ctx, `SELECT COUNT(*) FROM arko_gameplay_operations`).Scan(&operationCount); err != nil {
		return databaseError(tx.ctx, "count operations", err)
	}
	if operationCount >= tx.maxOperations {
		return core.ErrCapacity
	}
	_, err = tx.tx.ExecContext(tx.ctx,
		`INSERT INTO arko_gameplay_operations (character_id, operation_type, idempotency_key, request_id, fingerprint, operation_id, result, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, $8, $9)`,
		record.Scope.CharacterID, record.Scope.Type, record.Scope.Key, record.RequestID,
		record.Fingerprint, record.Result.OperationID, resultJSON, record.CreatedAt, record.UpdatedAt,
	)
	if err != nil {
		return databaseError(tx.ctx, "put operation", err)
	}
	return nil
}

type queryRower interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func readDefinition(ctx context.Context, queryer queryRower, id core.ItemID) (core.ItemDefinition, bool, error) {
	if err := ctx.Err(); err != nil {
		return core.ItemDefinition{}, false, err
	}
	var definition core.ItemDefinition
	var kind string
	err := queryer.QueryRowContext(ctx,
		`SELECT item_id, name, kind, max_stack FROM arko_gameplay_items WHERE item_id = $1`, id,
	).Scan(&definition.ID, &definition.Name, &kind, &definition.MaxStack)
	if errors.Is(err, sql.ErrNoRows) {
		return core.ItemDefinition{}, false, nil
	}
	if err != nil {
		return core.ItemDefinition{}, false, databaseError(ctx, "read item definition", err)
	}
	definition.Kind = core.ItemKind(kind)
	if err := core.ValidateDefinition(definition); err != nil {
		return core.ItemDefinition{}, false, dataError("validate item definition", err)
	}
	return definition, true, nil
}

func loadDefinitions(ctx context.Context, queryer queryRower, inventory core.Inventory) (map[core.ItemID]core.ItemDefinition, error) {
	definitions := make(map[core.ItemID]core.ItemDefinition)
	for _, slot := range inventory.Slots {
		if slot.ItemID == "" {
			continue
		}
		if _, loaded := definitions[slot.ItemID]; loaded {
			continue
		}
		definition, found, err := readDefinition(ctx, queryer, slot.ItemID)
		if err != nil {
			return nil, err
		}
		if found {
			definitions[slot.ItemID] = definition
		}
	}
	return definitions, nil
}

func recipeItemIDs(recipe core.Recipe) []core.ItemID {
	seen := map[core.ItemID]struct{}{}
	ids := make([]core.ItemID, 0, len(recipe.Materials)+1)
	for _, material := range recipe.Materials {
		if _, exists := seen[material.ItemID]; !exists {
			seen[material.ItemID] = struct{}{}
			ids = append(ids, material.ItemID)
		}
	}
	if _, exists := seen[recipe.Product.ItemID]; !exists {
		ids = append(ids, recipe.Product.ItemID)
	}
	return ids
}

func validInventoryShape(inventory core.Inventory) bool {
	if inventory.CharacterID == "" || len(inventory.Slots) > 10000 {
		return false
	}
	for _, slot := range inventory.Slots {
		if slot.ItemID == "" {
			if slot.Quantity != 0 {
				return false
			}
		} else if slot.Quantity <= 0 {
			return false
		}
	}
	return true
}

func dataError(operation string, err error) error {
	return fmt.Errorf("%w: %s: %v", core.ErrStorage, operation, err)
}

func databaseError(ctx context.Context, operation string, err error) error {
	if err == nil {
		return nil
	}
	if ctx != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
	}
	var sqlStateErr interface{ SQLState() string }
	if errors.As(err, &sqlStateErr) && (sqlStateErr.SQLState() == "40001" || sqlStateErr.SQLState() == "40P01") {
		return fmt.Errorf("%w: %w: %s: %v", core.ErrStorage, core.ErrRetryableStorage, operation, err)
	}
	return fmt.Errorf("%w: %s: %v", core.ErrStorage, operation, err)
}
