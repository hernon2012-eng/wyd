package postgres

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"arko-gameplay-core/internal/core"
	"github.com/DATA-DOG/go-sqlmock"
)

func TestStoreMigrateAppliesVersionedSchemaExplicitly(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	store := NewStore(db, 1024)
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("CREATE TABLE IF NOT EXISTS arko_gameplay_schema_migrations")).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`SELECT pg_advisory_xact_lock\(hashtextextended\(\$1, 0\)\)`).
		WithArgs("arko-gameplay-core:migrations").
		WillReturnRows(sqlmock.NewRows([]string{"pg_advisory_xact_lock"}).AddRow(nil))
	mock.ExpectQuery(`SELECT COALESCE\(MAX\(version\), 0\) FROM arko_gameplay_schema_migrations`).
		WillReturnRows(sqlmock.NewRows([]string{"coalesce"}).AddRow(0))

	migration, err := migrationFiles.ReadFile("migrations/" + migrationName)
	if err != nil {
		t.Fatalf("read embedded migration: %v", err)
	}
	for _, statement := range strings.Split(string(migration), ";") {
		statement = strings.TrimSpace(statement)
		if statement == "" {
			continue
		}
		table := regexp.QuoteMeta(statement[:strings.Index(statement, " (")])
		mock.ExpectExec("(?s)" + table + ".*").WillReturnResult(sqlmock.NewResult(0, 0))
	}
	mock.ExpectExec(`INSERT INTO arko_gameplay_schema_migrations`).
		WithArgs(migrationVersion, migrationName).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unfulfilled expectations: %v", err)
	}
}

func TestStoreTransactPersistsInventoryAndOperation(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	store := NewStore(db, 10)
	inventoryJSON := `[{"item_id":"demo:fiber","quantity":5}]`
	operationResultJSON := `{"operation_id":"op_existing","request_id":"req_existing","status":"completed","produced":[{"item_id":"demo:widget","quantity":1}]}`
	createdAt := time.Now().UTC().Truncate(time.Second)
	updatedAt := createdAt.Add(time.Minute)

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT character_id, slots FROM arko_gameplay_inventories WHERE character_id IN \(\$1\) ORDER BY character_id FOR UPDATE`).
		WithArgs("char_demo").
		WillReturnRows(sqlmock.NewRows([]string{"character_id", "slots"}).AddRow("char_demo", inventoryJSON))
	mock.ExpectQuery(`SELECT item_id, name, kind, max_stack FROM arko_gameplay_items WHERE item_id = \$1`).
		WithArgs("demo:fiber").
		WillReturnRows(sqlmock.NewRows([]string{"item_id", "name", "kind", "max_stack"}).AddRow("demo:fiber", "Fiber", "material", 10))
	mock.ExpectQuery(`SELECT pg_advisory_xact_lock\(hashtextextended\(\$1, 0\)\)`).
		WithArgs("9:char_demo:5:craft:11:op_existing").
		WillReturnRows(sqlmock.NewRows([]string{"pg_advisory_xact_lock"}).AddRow(nil))
	mock.ExpectQuery(`SELECT character_id, operation_type, idempotency_key, request_id, fingerprint, operation_id, result, created_at, updated_at FROM arko_gameplay_operations WHERE character_id = \$1 AND operation_type = \$2 AND idempotency_key = \$3 FOR UPDATE`).
		WithArgs("char_demo", core.OperationCraft, "op_existing").
		WillReturnRows(sqlmock.NewRows([]string{"character_id", "operation_type", "idempotency_key", "request_id", "fingerprint", "operation_id", "result", "created_at", "updated_at"}).
			AddRow("char_demo", core.OperationCraft, "op_existing", "req_existing", "fingerprint_existing", "op_existing", operationResultJSON, createdAt, updatedAt))
	mock.ExpectQuery(`SELECT item_id, name, kind, max_stack FROM arko_gameplay_items WHERE item_id = \$1`).
		WithArgs("demo:fiber").
		WillReturnRows(sqlmock.NewRows([]string{"item_id", "name", "kind", "max_stack"}).AddRow("demo:fiber", "Fiber", "material", 10))
	mock.ExpectExec(`INSERT INTO arko_gameplay_inventories`).
		WithArgs("char_demo", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`SELECT pg_advisory_xact_lock\(hashtextextended\(\$1, 0\)\)`).
		WithArgs("arko-gameplay-core:operation-capacity").
		WillReturnRows(sqlmock.NewRows([]string{"pg_advisory_xact_lock"}).AddRow(nil))
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM arko_gameplay_operations`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectExec(`INSERT INTO arko_gameplay_operations`).
		WithArgs("char_demo", core.OperationCraft, "op_next", "req_next", "fingerprint_next", "op_next", sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err = store.Transact(context.Background(), func(tx core.Transaction) error {
		inventory, ok, err := tx.ReadInventory("char_demo")
		if err != nil {
			return err
		}
		if !ok {
			return core.ErrNotFound
		}
		if got, want := len(inventory.Slots), 1; got != want {
			t.Fatalf("inventory slots = %d, want %d", got, want)
		}
		_, ok, err = tx.ReadOperation(core.OperationScope{CharacterID: "char_demo", Type: core.OperationCraft, Key: "op_existing"})
		if err != nil {
			return err
		}
		if !ok {
			return core.ErrNotFound
		}
		if err := tx.PutInventory(core.Inventory{CharacterID: "char_demo", Slots: []core.Slot{{ItemID: "demo:fiber", Quantity: 7}}}); err != nil {
			return err
		}
		record := core.OperationRecord{
			Scope:       core.OperationScope{CharacterID: "char_demo", Type: core.OperationCraft, Key: "op_next"},
			Fingerprint: "fingerprint_next",
			RequestID:   "req_next",
			Result: core.OperationResult{
				OperationID: "op_next",
				RequestID:   "req_next",
				Status:      core.OperationCompleted,
				Produced:    []core.ItemAmount{{ItemID: "demo:widget", Quantity: 1}},
			},
			CreatedAt: createdAt,
			UpdatedAt: updatedAt,
		}
		return tx.PutOperation(record)
	})
	if err != nil {
		t.Fatalf("Transact() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unfulfilled expectations: %v", err)
	}
}

func TestNewStoreDoesNotMigrateAutomatically(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	store := NewStore(db, 10)
	if store == nil {
		t.Fatal("NewStore() returned nil")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unexpected database operation during construction: %v", err)
	}
}

func TestStoreTransactPropagatesReadStorageErrors(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	store := NewStore(db, 10)
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT item_id, name, kind, max_stack FROM arko_gameplay_items WHERE item_id = \$1`).
		WithArgs("demo:missing").
		WillReturnError(errors.New("database unavailable"))
	mock.ExpectRollback()

	err = store.Transact(context.Background(), func(tx core.Transaction) error {
		_, _, readErr := tx.ReadItemDefinition("demo:missing")
		return readErr
	})
	if !errors.Is(err, core.ErrStorage) || errors.Is(err, core.ErrNotFound) {
		t.Fatalf("Transact() error = %v, want storage failure, not not-found", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unfulfilled expectations: %v", err)
	}
}

func TestStoreTransactRollsBackOnContextCancellation(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	store := NewStore(db, 10)
	ctx, cancel := context.WithCancel(context.Background())
	mock.ExpectBegin()
	mock.ExpectRollback()
	err = store.Transact(ctx, func(core.Transaction) error {
		cancel()
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Transact() error = %v, want context cancellation", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unfulfilled expectations: %v", err)
	}
}
