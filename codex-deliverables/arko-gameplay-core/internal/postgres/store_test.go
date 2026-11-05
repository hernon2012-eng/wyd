package postgres

import (
	"context"
	"regexp"
	"testing"
	"time"

	"arko-gameplay-core/internal/core"
	"github.com/DATA-DOG/go-sqlmock"
)

func TestStoreMigrateCreatesSchema(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	store := NewStore(db, 1024)
	mock.ExpectExec(regexp.QuoteMeta("CREATE TABLE IF NOT EXISTS gameplay_items")).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta("CREATE TABLE IF NOT EXISTS gameplay_recipes")).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta("CREATE TABLE IF NOT EXISTS gameplay_inventories")).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta("CREATE TABLE IF NOT EXISTS gameplay_operations")).WillReturnResult(sqlmock.NewResult(0, 0))

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
	mock.ExpectQuery(`SELECT character_id, slots FROM gameplay_inventories WHERE character_id = \$1`).
		WithArgs("char_demo").
		WillReturnRows(sqlmock.NewRows([]string{"character_id", "slots"}).AddRow("char_demo", inventoryJSON))
	mock.ExpectQuery(`SELECT character_id, operation_type, idempotency_key, request_id, fingerprint, operation_id, result, created_at, updated_at FROM gameplay_operations WHERE character_id = \$1 AND operation_type = \$2 AND idempotency_key = \$3`).
		WithArgs("char_demo", core.OperationCraft, "op_existing").
		WillReturnRows(sqlmock.NewRows([]string{"character_id", "operation_type", "idempotency_key", "request_id", "fingerprint", "operation_id", "result", "created_at", "updated_at"}).
			AddRow("char_demo", core.OperationCraft, "op_existing", "req_existing", "fingerprint_existing", "op_existing", operationResultJSON, createdAt, updatedAt))
	mock.ExpectExec("INSERT INTO gameplay_inventories").
		WithArgs("char_demo", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO gameplay_operations").
		WithArgs("char_demo", core.OperationCraft, "op_next", "req_next", "fingerprint_next", "op_next", sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err = store.Transact(context.Background(), func(tx core.Transaction) error {
		inv, ok := tx.Inventory("char_demo")
		if !ok {
			return core.ErrNotFound
		}
		if got, want := len(inv.Slots), 1; got != want {
			t.Fatalf("inventory slots = %d, want %d", got, want)
		}
		_, ok = tx.Operation(core.OperationScope{CharacterID: "char_demo", Type: core.OperationCraft, Key: "op_existing"})
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
