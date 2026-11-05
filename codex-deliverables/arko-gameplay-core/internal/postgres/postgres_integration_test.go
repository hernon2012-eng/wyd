package postgres

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"arko-gameplay-core/internal/core"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

const (
	integrationDSNEnv = "ARKO_POSTGRES_TEST_DSN"
	isolatedHost      = "127.0.0.1"
	isolatedPort      = 55432
	isolatedDatabase  = "arko_test"
	isolatedUser      = "arko_test"
)

type postgresHarness struct {
	t          *testing.T
	adminDB    *sql.DB
	db         *sql.DB
	config     *pgx.ConnConfig
	schema     string
	appName    string
	store      *Store
	engine     *core.Engine
	actor      core.Actor
	characters []core.CharacterID
}

func newPostgresHarness(t *testing.T) *postgresHarness {
	t.Helper()

	dsn := os.Getenv(integrationDSNEnv)
	if strings.TrimSpace(dsn) == "" {
		t.Skipf("set %s to the isolated PostgreSQL test service to run integration tests", integrationDSNEnv)
	}

	config, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parse %s: %v", integrationDSNEnv, err)
	}
	if config.Host != isolatedHost || config.Port != isolatedPort ||
		config.Database != isolatedDatabase || config.User != isolatedUser {
		t.Fatalf(
			"refusing PostgreSQL endpoint %q:%d database %q user %q; integration tests are restricted to %s:%d/%s as %s",
			config.Host, config.Port, config.Database, config.User,
			isolatedHost, isolatedPort, isolatedDatabase, isolatedUser,
		)
	}

	var schemaID [12]byte
	if _, err := rand.Read(schemaID[:]); err != nil {
		t.Fatalf("generate disposable schema name: %v", err)
	}
	schema := "arko_it_" + hex.EncodeToString(schemaID[:])
	appName := schema

	adminConfig, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parse admin PostgreSQL config: %v", err)
	}
	adminConfig.RuntimeParams["application_name"] = appName + "_admin"
	adminDB := stdlib.OpenDB(*adminConfig)
	adminDB.SetMaxOpenConns(2)
	adminDB.SetMaxIdleConns(1)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := adminDB.PingContext(ctx); err != nil {
		_ = adminDB.Close()
		t.Fatalf("connect to isolated PostgreSQL service: %v", err)
	}
	if _, err := adminDB.ExecContext(ctx, `CREATE SCHEMA `+quoteIdentifier(schema)); err != nil {
		_ = adminDB.Close()
		t.Fatalf("create disposable schema %q: %v", schema, err)
	}

	h := &postgresHarness{
		t:          t,
		adminDB:    adminDB,
		config:     config,
		schema:     schema,
		appName:    appName,
		characters: []core.CharacterID{"char_alpha", "char_beta", "char_gamma"},
	}
	h.actor = core.Actor{PrincipalID: "integration-test", AuthorizedCharacters: h.characters}
	t.Cleanup(func() {
		if h.db != nil {
			_ = h.db.Close()
		}
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cleanupCancel()
		if _, err := h.adminDB.ExecContext(cleanupCtx, `DROP SCHEMA IF EXISTS `+quoteIdentifier(h.schema)+` CASCADE`); err != nil {
			t.Errorf("drop disposable schema %q: %v", h.schema, err)
		}
		if err := h.adminDB.Close(); err != nil {
			t.Errorf("close PostgreSQL admin connection: %v", err)
		}
	})

	h.openStore()
	h.migrate()
	h.seedDefinitionsAndRecipe()
	return h
}

func (h *postgresHarness) openStore() {
	h.t.Helper()
	config := h.config.Copy()
	config.RuntimeParams["search_path"] = h.schema
	config.RuntimeParams["application_name"] = h.appName
	h.db = stdlib.OpenDB(*config)
	h.db.SetMaxOpenConns(8)
	h.db.SetMaxIdleConns(8)
	h.store = NewStore(h.db, 1000)
	engine, err := core.NewEngine(h.store, core.EngineOptions{})
	if err != nil {
		h.t.Fatalf("create gameplay engine: %v", err)
	}
	h.engine = engine
}

func (h *postgresHarness) reopenStore(maxConnections int) {
	h.t.Helper()
	if err := h.db.Close(); err != nil {
		h.t.Fatalf("close store before restart: %v", err)
	}
	config := h.config.Copy()
	config.RuntimeParams["search_path"] = h.schema
	config.RuntimeParams["application_name"] = h.appName
	h.db = stdlib.OpenDB(*config)
	h.db.SetMaxOpenConns(maxConnections)
	h.db.SetMaxIdleConns(maxConnections)
	h.store = NewStore(h.db, 1000)
	engine, err := core.NewEngine(h.store, core.EngineOptions{})
	if err != nil {
		h.t.Fatalf("recreate gameplay engine: %v", err)
	}
	h.engine = engine
}

func (h *postgresHarness) migrate() {
	h.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := h.store.Migrate(ctx); err != nil {
		h.t.Fatalf("apply PostgreSQL migrations in disposable schema %q: %v", h.schema, err)
	}
	var version int
	var name string
	if err := h.db.QueryRowContext(ctx,
		`SELECT version, name FROM arko_gameplay_schema_migrations ORDER BY version DESC LIMIT 1`,
	).Scan(&version, &name); err != nil {
		h.t.Fatalf("read applied PostgreSQL migration version: %v", err)
	}
	if version != migrationVersion || name != migrationName {
		h.t.Fatalf("latest migration = (%d, %q), want (%d, %q)", version, name, migrationVersion, migrationName)
	}
}

func (h *postgresHarness) seedDefinitionsAndRecipe() {
	h.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	for _, definition := range []core.ItemDefinition{
		{ID: "demo:ore", Name: "Ore", Kind: core.ItemKindMaterial, MaxStack: 99},
		{ID: "demo:bar", Name: "Bar", Kind: core.ItemKindGeneric, MaxStack: 99},
	} {
		if err := h.store.SeedItemDefinition(ctx, definition); err != nil {
			h.t.Fatalf("seed item definition %q: %v", definition.ID, err)
		}
	}
	recipe := core.Recipe{
		ID:        "demo:smelt",
		Version:   1,
		Name:      "Smelt",
		Materials: []core.ItemAmount{{ItemID: "demo:ore", Quantity: 2}},
		Product:   core.ItemAmount{ItemID: "demo:bar", Quantity: 1},
		Active:    true,
	}
	if err := h.store.SeedRecipe(ctx, recipe); err != nil {
		h.t.Fatalf("seed recipe: %v", err)
	}
}

func (h *postgresHarness) seedInventory(characterID core.CharacterID, ore int64) {
	h.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	inventory := core.Inventory{
		CharacterID: characterID,
		Slots: []core.Slot{
			{ItemID: "demo:ore", Quantity: ore},
			{},
			{},
		},
	}
	if err := h.store.SeedInventory(ctx, inventory); err != nil {
		h.t.Fatalf("seed inventory for %q: %v", characterID, err)
	}
}

func (h *postgresHarness) craft(characterID core.CharacterID, requestID string, key core.IdempotencyKey, quantity int64) (core.OperationResult, error) {
	return h.engine.Craft(context.Background(), h.actor, core.CraftRequest{
		RequestID:      requestID,
		IdempotencyKey: key,
		CharacterID:    characterID,
		RecipeID:       "demo:smelt",
		RecipeVersion:  1,
		Quantity:       quantity,
	})
}

func (h *postgresHarness) inventory(characterID core.CharacterID) core.Inventory {
	h.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	inventory, err := h.engine.Inventory(ctx, h.actor, characterID)
	if err != nil {
		h.t.Fatalf("read inventory for %q: %v", characterID, err)
	}
	return inventory
}

func (h *postgresHarness) operationCount() int {
	h.t.Helper()
	var count int
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := h.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM arko_gameplay_operations`).Scan(&count); err != nil {
		h.t.Fatalf("count operation records: %v", err)
	}
	return count
}

func (h *postgresHarness) installFailureTrigger(table, trigger, function, body string) {
	h.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	qualifiedFunction := quoteIdentifier(h.schema) + "." + quoteIdentifier(function)
	qualifiedTable := quoteIdentifier(h.schema) + "." + quoteIdentifier(table)
	if _, err := h.db.ExecContext(ctx,
		`CREATE FUNCTION `+qualifiedFunction+`() RETURNS trigger LANGUAGE plpgsql AS $$ `+body+` $$`); err != nil {
		h.t.Fatalf("create failure trigger function: %v", err)
	}
	if _, err := h.db.ExecContext(ctx,
		`CREATE TRIGGER `+quoteIdentifier(trigger)+` BEFORE INSERT OR UPDATE ON `+qualifiedTable+
			` FOR EACH ROW EXECUTE FUNCTION `+qualifiedFunction+`()`,
	); err != nil {
		h.t.Fatalf("create failure trigger: %v", err)
	}
}

func TestPostgresPersistentInventoryAndRestartRecovery(t *testing.T) {
	h := newPostgresHarness(t)
	character := h.characters[0]
	h.seedInventory(character, 8)

	first, err := h.craft(character, "restart-request-1", "restart-key", 1)
	if err != nil || first.Status != core.OperationCompleted || first.Replayed {
		t.Fatalf("initial craft = (%+v, %v), want completed non-replay", first, err)
	}

	h.reopenStore(8)
	replayed, err := h.craft(character, "restart-request-2", "restart-key", 1)
	if err != nil || replayed.Status != core.OperationCompleted || !replayed.Replayed {
		t.Fatalf("craft after restart = (%+v, %v), want completed replay", replayed, err)
	}
	if replayed.OperationID != first.OperationID {
		t.Fatalf("replayed operation ID = %q, want %q", replayed.OperationID, first.OperationID)
	}
	assertInventory(t, h.inventory(character), map[core.ItemID]int64{"demo:ore": 6, "demo:bar": 1})
	if got := h.operationCount(); got != 1 {
		t.Fatalf("operation count after replay = %d, want 1", got)
	}
}

func TestPostgresCraftingTransactionAtomicity(t *testing.T) {
	h := newPostgresHarness(t)
	character := h.characters[0]
	h.seedInventory(character, 10)

	result, err := h.craft(character, "atomic-request", "atomic-key", 2)
	if err != nil {
		t.Fatalf("Craft() error = %v", err)
	}
	if result.Status != core.OperationCompleted || result.Replayed {
		t.Fatalf("Craft() result = %+v, want completed non-replay", result)
	}
	assertInventory(t, h.inventory(character), map[core.ItemID]int64{"demo:ore": 6, "demo:bar": 2})
	if got := h.operationCount(); got != 1 {
		t.Fatalf("operation count = %d, want 1", got)
	}
}

func TestPostgresTransfers(t *testing.T) {
	h := newPostgresHarness(t)
	source, destination := h.characters[0], h.characters[1]
	h.seedInventory(source, 7)
	h.seedInventory(destination, 1)

	result, err := h.engine.Transfer(context.Background(), h.actor, core.TransferRequest{
		RequestID:      "transfer-request",
		IdempotencyKey: "transfer-key",
		Source:         source,
		Destination:    destination,
		Item:           core.ItemAmount{ItemID: "demo:ore", Quantity: 3},
	})
	if err != nil {
		t.Fatalf("Transfer() error = %v", err)
	}
	if result.Status != core.OperationCompleted || result.Replayed {
		t.Fatalf("Transfer() result = %+v, want completed non-replay", result)
	}
	assertInventory(t, h.inventory(source), map[core.ItemID]int64{"demo:ore": 4})
	assertInventory(t, h.inventory(destination), map[core.ItemID]int64{"demo:ore": 4})
	if got := h.operationCount(); got != 1 {
		t.Fatalf("operation count = %d, want 1", got)
	}
}

func TestPostgresConcurrentDuplicateRequests(t *testing.T) {
	h := newPostgresHarness(t)
	character := h.characters[0]
	h.seedInventory(character, 30)

	const attempts = 8
	results := make([]core.OperationResult, attempts)
	errs := make([]error, attempts)
	runConcurrently(attempts, func(index int) {
		results[index], errs[index] = h.craft(character, fmt.Sprintf("duplicate-request-%d", index), "duplicate-key", 1)
	})
	replays := 0
	var operationID string
	for index, result := range results {
		if errs[index] != nil {
			t.Errorf("duplicate request %d error = %v", index, errs[index])
			continue
		}
		if result.Status != core.OperationCompleted {
			t.Errorf("duplicate request %d result = %+v, want completed", index, result)
		}
		if result.Replayed {
			replays++
		}
		if operationID == "" {
			operationID = result.OperationID
		} else if result.OperationID != operationID {
			t.Errorf("duplicate request %d operation ID = %q, want %q", index, result.OperationID, operationID)
		}
	}
	if got := attempts - replays; got != 1 {
		t.Errorf("non-replayed duplicate requests = %d, want 1", got)
	}
	assertInventory(t, h.inventory(character), map[core.ItemID]int64{"demo:ore": 28, "demo:bar": 1})
	if got := h.operationCount(); got != 1 {
		t.Errorf("operation count = %d, want 1", got)
	}
}

func TestPostgresSameIdempotencyKeyDifferentContent(t *testing.T) {
	h := newPostgresHarness(t)
	character := h.characters[0]
	h.seedInventory(character, 10)

	first, err := h.craft(character, "same-key-first", "same-key", 1)
	if err != nil || first.Status != core.OperationCompleted {
		t.Fatalf("first craft = (%+v, %v), want completed", first, err)
	}
	second, err := h.craft(character, "same-key-second", "same-key", 2)
	if err != nil {
		t.Fatalf("conflicting craft error = %v", err)
	}
	if second.Status != core.OperationRejected || second.Code != core.CodeIdempotencyConflict {
		t.Fatalf("conflicting craft = %+v, want idempotency conflict rejection", second)
	}
	assertInventory(t, h.inventory(character), map[core.ItemID]int64{"demo:ore": 8, "demo:bar": 1})
	if got := h.operationCount(); got != 1 {
		t.Fatalf("operation count = %d, want 1", got)
	}
}

func TestPostgresRollbackOnInventoryWriteFailure(t *testing.T) {
	h := newPostgresHarness(t)
	character := h.characters[0]
	h.seedInventory(character, 8)
	h.installFailureTrigger(
		"arko_gameplay_inventories",
		"fail_inventory_write",
		"fail_inventory_write_fn",
		`BEGIN RAISE EXCEPTION 'injected inventory write failure'; END;`,
	)

	_, err := h.craft(character, "inventory-failure-request", "inventory-failure-key", 1)
	if !errors.Is(err, core.ErrStorage) {
		t.Fatalf("Craft() error = %v, want storage failure", err)
	}
	assertInventory(t, h.inventory(character), map[core.ItemID]int64{"demo:ore": 8})
	if got := h.operationCount(); got != 0 {
		t.Fatalf("operation count after failed inventory write = %d, want 0", got)
	}
}

func TestPostgresRollbackOnDeliveryFailure(t *testing.T) {
	h := newPostgresHarness(t)
	source, destination := h.characters[0], h.characters[1]
	h.seedInventory(source, 8)
	h.seedInventory(destination, 1)
	h.installFailureTrigger(
		"arko_gameplay_inventories",
		"fail_delivery",
		"fail_delivery_fn",
		`BEGIN IF NEW.character_id = 'char_beta' THEN RAISE EXCEPTION 'injected delivery failure'; END IF; RETURN NEW; END;`,
	)

	_, err := h.engine.Transfer(context.Background(), h.actor, core.TransferRequest{
		RequestID:      "delivery-failure-request",
		IdempotencyKey: "delivery-failure-key",
		Source:         source,
		Destination:    destination,
		Item:           core.ItemAmount{ItemID: "demo:ore", Quantity: 2},
	})
	if !errors.Is(err, core.ErrStorage) {
		t.Fatalf("Transfer() error = %v, want storage failure", err)
	}
	assertInventory(t, h.inventory(source), map[core.ItemID]int64{"demo:ore": 8})
	assertInventory(t, h.inventory(destination), map[core.ItemID]int64{"demo:ore": 1})
	if got := h.operationCount(); got != 0 {
		t.Fatalf("operation count after failed delivery = %d, want 0", got)
	}
}

func TestPostgresRollbackOnOperationRecordFailure(t *testing.T) {
	h := newPostgresHarness(t)
	character := h.characters[0]
	h.seedInventory(character, 8)
	h.installFailureTrigger(
		"arko_gameplay_operations",
		"fail_operation_record",
		"fail_operation_record_fn",
		`BEGIN RAISE EXCEPTION 'injected operation record failure'; END;`,
	)

	_, err := h.craft(character, "operation-failure-request", "operation-failure-key", 1)
	if !errors.Is(err, core.ErrStorage) {
		t.Fatalf("Craft() error = %v, want storage failure", err)
	}
	assertInventory(t, h.inventory(character), map[core.ItemID]int64{"demo:ore": 8})
	if got := h.operationCount(); got != 0 {
		t.Fatalf("operation count after failed operation record = %d, want 0", got)
	}
}

func TestPostgresSimultaneousCrafts(t *testing.T) {
	h := newPostgresHarness(t)
	character := h.characters[0]
	h.seedInventory(character, 10)

	results := make([]core.OperationResult, 2)
	errs := make([]error, 2)
	runConcurrently(2, func(index int) {
		results[index], errs[index] = h.craft(character, fmt.Sprintf("parallel-craft-%d", index), core.IdempotencyKey(fmt.Sprintf("parallel-key-%d", index)), 1)
	})
	for index := range results {
		if errs[index] != nil {
			t.Errorf("craft %d error = %v", index, errs[index])
		} else if results[index].Status != core.OperationCompleted {
			t.Errorf("craft %d result = %+v, want completed", index, results[index])
		}
	}
	assertInventory(t, h.inventory(character), map[core.ItemID]int64{"demo:ore": 6, "demo:bar": 2})
	if got := h.operationCount(); got != 2 {
		t.Errorf("operation count = %d, want 2", got)
	}
}

func TestPostgresOppositeDirectionTransfers(t *testing.T) {
	h := newPostgresHarness(t)
	alpha, beta := h.characters[0], h.characters[1]
	h.seedInventory(alpha, 10)
	h.seedInventory(beta, 10)

	results := make([]core.OperationResult, 2)
	errs := make([]error, 2)
	runConcurrently(2, func(index int) {
		source, destination := alpha, beta
		if index == 1 {
			source, destination = beta, alpha
		}
		results[index], errs[index] = h.engine.Transfer(context.Background(), h.actor, core.TransferRequest{
			RequestID:      fmt.Sprintf("opposite-transfer-%d", index),
			IdempotencyKey: core.IdempotencyKey(fmt.Sprintf("opposite-transfer-key-%d", index)),
			Source:         source,
			Destination:    destination,
			Item:           core.ItemAmount{ItemID: "demo:ore", Quantity: 2},
		})
	})
	for index := range results {
		if errs[index] != nil {
			t.Errorf("transfer %d error = %v", index, errs[index])
		} else if results[index].Status != core.OperationCompleted {
			t.Errorf("transfer %d result = %+v, want completed", index, results[index])
		}
	}
	assertInventory(t, h.inventory(alpha), map[core.ItemID]int64{"demo:ore": 10})
	assertInventory(t, h.inventory(beta), map[core.ItemID]int64{"demo:ore": 10})
	if got := h.operationCount(); got != 2 {
		t.Errorf("operation count = %d, want 2", got)
	}
}

func TestPostgresIndependentCharacterConcurrency(t *testing.T) {
	h := newPostgresHarness(t)
	alpha, beta := h.characters[0], h.characters[1]
	h.seedInventory(alpha, 8)
	h.seedInventory(beta, 8)

	results := make([]core.OperationResult, 2)
	errs := make([]error, 2)
	runConcurrently(2, func(index int) {
		character := []core.CharacterID{alpha, beta}[index]
		results[index], errs[index] = h.craft(character,
			fmt.Sprintf("independent-request-%d", index),
			core.IdempotencyKey(fmt.Sprintf("independent-key-%d", index)), 1)
	})
	for index := range results {
		if errs[index] != nil {
			t.Errorf("character craft %d error = %v", index, errs[index])
		} else if results[index].Status != core.OperationCompleted {
			t.Errorf("character craft %d result = %+v, want completed", index, results[index])
		}
	}
	assertInventory(t, h.inventory(alpha), map[core.ItemID]int64{"demo:ore": 6, "demo:bar": 1})
	assertInventory(t, h.inventory(beta), map[core.ItemID]int64{"demo:ore": 6, "demo:bar": 1})
}

func TestPostgresRecoveryAfterConnectionFailure(t *testing.T) {
	h := newPostgresHarness(t)
	character := h.characters[0]
	h.seedInventory(character, 8)
	h.reopenStore(1)

	entered := make(chan struct{})
	continueAfterTermination := make(chan struct{})
	transactionDone := make(chan error, 1)
	go func() {
		transactionDone <- h.store.Transact(context.Background(), func(tx core.Transaction) error {
			if _, ok := tx.Inventory(character); !ok {
				return errors.New("initial inventory read failed")
			}
			close(entered)
			<-continueAfterTermination
			if _, ok := tx.Inventory(character); ok {
				return errors.New("inventory read unexpectedly succeeded after backend termination")
			}
			return errors.New("expected connection failure after backend termination")
		})
	}()

	select {
	case <-entered:
	case <-time.After(20 * time.Second):
		t.Fatal("transaction did not acquire a PostgreSQL connection")
	}
	var backendPID int
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	err := h.adminDB.QueryRowContext(ctx,
		`SELECT pid FROM pg_stat_activity WHERE application_name = $1 AND datname = $2 AND state = 'idle in transaction'`,
		h.appName, isolatedDatabase,
	).Scan(&backendPID)
	if err != nil {
		cancel()
		t.Fatalf("find active test backend: %v", err)
	}
	var terminated bool
	if err := h.adminDB.QueryRowContext(ctx, `SELECT pg_terminate_backend($1)`, backendPID).Scan(&terminated); err != nil {
		cancel()
		t.Fatalf("terminate test backend %d: %v", backendPID, err)
	}
	cancel()
	if !terminated {
		t.Fatalf("PostgreSQL did not terminate test backend %d", backendPID)
	}
	close(continueAfterTermination)
	select {
	case err := <-transactionDone:
		if err == nil {
			t.Fatal("transaction unexpectedly succeeded after its PostgreSQL backend was terminated")
		}
	case <-time.After(20 * time.Second):
		t.Fatal("transaction did not finish after connection termination")
	}

	result, err := h.craft(character, "recovery-request", "recovery-key", 1)
	if err != nil || result.Status != core.OperationCompleted {
		t.Fatalf("craft after connection recovery = (%+v, %v), want completed", result, err)
	}
	assertInventory(t, h.inventory(character), map[core.ItemID]int64{"demo:ore": 6, "demo:bar": 1})
}

func assertInventory(t *testing.T, inventory core.Inventory, want map[core.ItemID]int64) {
	t.Helper()
	got := make(map[core.ItemID]int64)
	for _, slot := range inventory.Slots {
		if slot.ItemID != "" {
			got[slot.ItemID] += slot.Quantity
		}
	}
	if len(got) != len(want) {
		t.Fatalf("inventory items = %#v, want %#v", got, want)
	}
	for item, quantity := range want {
		if got[item] != quantity {
			t.Fatalf("inventory quantity for %q = %d, want %d (all items: %#v)", item, got[item], quantity, got)
		}
	}
}

func runConcurrently(count int, run func(int)) {
	var ready sync.WaitGroup
	ready.Add(count)
	start := make(chan struct{})
	var done sync.WaitGroup
	done.Add(count)
	for index := 0; index < count; index++ {
		go func(index int) {
			defer done.Done()
			ready.Done()
			<-start
			run(index)
		}(index)
	}
	ready.Wait()
	close(start)
	done.Wait()
}

func quoteIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}
