package core

import (
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"sync"
	"testing"
)

func newCraftFixture(t *testing.T, slots []Slot, recipe Recipe) (*MemoryStore, *Engine, Actor) {
	t.Helper()
	store := NewMemoryStore(100)
	definitions := []ItemDefinition{
		{ID: "demo:fiber", Name: "Demo Fiber", Kind: ItemKindMaterial, MaxStack: 10},
		{ID: "demo:resin", Name: "Demo Resin", Kind: ItemKindMaterial, MaxStack: 10},
		{ID: "demo:widget", Name: "Demo Widget", Kind: ItemKindGeneric, MaxStack: 5},
	}
	for _, definition := range definitions {
		if err := store.SeedItemDefinition(definition); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.SeedRecipe(recipe); err != nil {
		t.Fatal(err)
	}
	if err := store.SeedInventory(Inventory{CharacterID: "char_demo", Slots: slots}); err != nil {
		t.Fatal(err)
	}
	engine, err := NewEngine(store, EngineOptions{AllowDemoRecipes: true})
	if err != nil {
		t.Fatal(err)
	}
	actor := Actor{PrincipalID: "principal_demo", AuthorizedCharacters: []CharacterID{"char_demo", "char_alt"}}
	return store, engine, actor
}

func demoRecipe() Recipe {
	return Recipe{
		ID:        "demo:widget_v1",
		Version:   1,
		Name:      "Demonstration Widget",
		Materials: []ItemAmount{{ItemID: "demo:fiber", Quantity: 2}, {ItemID: "demo:resin", Quantity: 1}},
		Product:   ItemAmount{ItemID: "demo:widget", Quantity: 1},
		Active:    true,
		DemoOnly:  true,
	}
}

func demoActor() Actor {
	return Actor{PrincipalID: "principal_demo", AuthorizedCharacters: []CharacterID{"char_demo", "char_alt"}}
}

func craftRequest(key IdempotencyKey, quantity int64) CraftRequest {
	return CraftRequest{RequestID: "req_demo", IdempotencyKey: key, CharacterID: "char_demo", RecipeID: "demo:widget_v1", RecipeVersion: 1, Quantity: quantity}
}

func TestCraftSuccessfulConsumesMaterialsAndDeliversProduct(t *testing.T) {
	store, engine, actor := newCraftFixture(t, []Slot{{ItemID: "demo:fiber", Quantity: 5}, {ItemID: "demo:resin", Quantity: 3}, {}, {}}, demoRecipe())
	result, err := engine.Craft(context.Background(), actor, craftRequest("craft_success", 2))
	if err != nil || result.Status != OperationCompleted {
		t.Fatalf("Craft() = %#v, %v", result, err)
	}
	want := []Slot{{ItemID: "demo:fiber", Quantity: 1}, {ItemID: "demo:resin", Quantity: 1}, {ItemID: "demo:widget", Quantity: 2}, {}}
	got, _ := store.Snapshot("char_demo")
	if !reflect.DeepEqual(got.Slots, want) {
		t.Fatalf("slots = %#v, want %#v", got.Slots, want)
	}
}

func TestCraftInsufficientMaterialsDoesNotMutate(t *testing.T) {
	store, engine, actor := newCraftFixture(t, []Slot{{ItemID: "demo:fiber", Quantity: 1}, {ItemID: "demo:resin", Quantity: 4}}, demoRecipe())
	before, _ := store.Snapshot("char_demo")
	result, err := engine.Craft(context.Background(), actor, craftRequest("craft_short", 1))
	after, _ := store.Snapshot("char_demo")
	if err != nil || result.Code != CodeInsufficientMaterials || !reflect.DeepEqual(after, before) {
		t.Fatalf("result=%#v err=%v changed=%v", result, err, !reflect.DeepEqual(after, before))
	}
}

func TestCraftInventoryFullDoesNotConsume(t *testing.T) {
	slots := []Slot{{ItemID: "demo:fiber", Quantity: 3}, {ItemID: "demo:resin", Quantity: 2}, {ItemID: "demo:widget", Quantity: 5}}
	store, engine, actor := newCraftFixture(t, slots, demoRecipe())
	before, _ := store.Snapshot("char_demo")
	result, err := engine.Craft(context.Background(), actor, craftRequest("craft_full", 1))
	after, _ := store.Snapshot("char_demo")
	if err != nil || result.Code != CodeInventoryFull || !reflect.DeepEqual(after, before) {
		t.Fatalf("result=%#v err=%v changed=%v", result, err, !reflect.DeepEqual(after, before))
	}
}

func TestCraftInvalidQuantityDoesNotMutate(t *testing.T) {
	store, engine, actor := newCraftFixture(t, []Slot{{ItemID: "demo:fiber", Quantity: 5}, {ItemID: "demo:resin", Quantity: 5}, {}}, demoRecipe())
	before, _ := store.Snapshot("char_demo")
	result, err := engine.Craft(context.Background(), actor, craftRequest("craft_zero", 0))
	after, _ := store.Snapshot("char_demo")
	if err != nil || result.Code != CodeInvalidRequest || !reflect.DeepEqual(after, before) {
		t.Fatalf("result=%#v err=%v changed=%v", result, err, !reflect.DeepEqual(after, before))
	}
}

func TestCraftInactiveRecipeRejected(t *testing.T) {
	recipe := demoRecipe()
	recipe.Active = false
	_, engine, actor := newCraftFixture(t, []Slot{{ItemID: "demo:fiber", Quantity: 5}, {ItemID: "demo:resin", Quantity: 5}}, recipe)
	result, err := engine.Craft(context.Background(), actor, craftRequest("craft_inactive", 1))
	if err != nil || result.Code != CodeRecipeInactive {
		t.Fatalf("Craft() = %#v, %v", result, err)
	}
}

func TestCraftMissingRecipeRejected(t *testing.T) {
	store, engine, actor := newCraftFixture(t, []Slot{{ItemID: "demo:fiber", Quantity: 5}}, demoRecipe())
	before, _ := store.Snapshot("char_demo")
	request := craftRequest("craft_missing", 1)
	request.RecipeID = "demo:missing"
	result, err := engine.Craft(context.Background(), actor, request)
	after, _ := store.Snapshot("char_demo")
	if err != nil || result.Code != CodeRecipeNotFound || !reflect.DeepEqual(before, after) {
		t.Fatalf("Craft() = %#v, %v, state_changed=%v", result, err, !reflect.DeepEqual(before, after))
	}
	if _, exists := store.LookupOperation(OperationScope{CharacterID: "char_demo", Type: OperationCraft, Key: request.IdempotencyKey}); !exists {
		t.Fatal("deterministic rejection was not persisted")
	}
}

func TestCraftReplayReturnsStoredResultWithoutSecondDelivery(t *testing.T) {
	store, engine, actor := newCraftFixture(t, []Slot{{ItemID: "demo:fiber", Quantity: 5}, {ItemID: "demo:resin", Quantity: 5}, {}}, demoRecipe())
	first, err := engine.Craft(context.Background(), actor, craftRequest("craft_replay", 1))
	if err != nil {
		t.Fatal(err)
	}
	request := craftRequest("craft_replay", 1)
	request.RequestID = "req_retry"
	second, err := engine.Craft(context.Background(), actor, request)
	if err != nil || !second.Replayed || second.RequestID != "req_retry" || first.OperationID != second.OperationID {
		t.Fatalf("first=%#v second=%#v err=%v", first, second, err)
	}
	inventory, _ := store.Snapshot("char_demo")
	if inventory.Slots[2].Quantity != 1 {
		t.Fatalf("product delivered %d times", inventory.Slots[2].Quantity)
	}
}

func TestCraftConcurrentSameRequestCommitsOnce(t *testing.T) {
	store, engine, actor := newCraftFixture(t, []Slot{{ItemID: "demo:fiber", Quantity: 10}, {ItemID: "demo:resin", Quantity: 10}, {}}, demoRecipe())
	const callers = 24
	results := make([]OperationResult, callers)
	errorsSeen := make([]error, callers)
	var wait sync.WaitGroup
	for index := range results {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			request := craftRequest("craft_parallel", 1)
			request.RequestID = fmt.Sprintf("req_parallel_%d", index)
			results[index], errorsSeen[index] = engine.Craft(context.Background(), actor, request)
		}(index)
	}
	wait.Wait()
	operationID := results[0].OperationID
	for index, result := range results {
		if errorsSeen[index] != nil || result.Status != OperationCompleted || result.OperationID != operationID {
			t.Fatalf("result[%d]=%#v err=%v", index, result, errorsSeen[index])
		}
	}
	inventory, _ := store.Snapshot("char_demo")
	if inventory.Slots[2].Quantity != 1 {
		t.Fatalf("product quantity = %d, want 1", inventory.Slots[2].Quantity)
	}
}

func TestCraftRetryAfterLostResponseReturnsCommittedResult(t *testing.T) {
	store, engine, actor := newCraftFixture(t, []Slot{{ItemID: "demo:fiber", Quantity: 2}, {ItemID: "demo:resin", Quantity: 1}, {}}, demoRecipe())
	request := craftRequest("craft_disconnect", 1)
	if _, err := engine.Craft(context.Background(), actor, request); err != nil {
		t.Fatal(err)
	}
	committed, _ := store.Snapshot("char_demo")
	request.RequestID = "req_after_disconnect"
	result, err := engine.Craft(context.Background(), actor, request)
	current, _ := store.Snapshot("char_demo")
	if err != nil || !result.Replayed || !reflect.DeepEqual(current, committed) {
		t.Fatalf("result=%#v err=%v state_changed=%v", result, err, !reflect.DeepEqual(current, committed))
	}
}

func TestCraftRetriesAfterTransientStorageFailure(t *testing.T) {
	store, engine, actor := newCraftFixture(t, []Slot{{ItemID: "demo:fiber", Quantity: 2}, {ItemID: "demo:resin", Quantity: 1}, {}}, demoRecipe())
	before, _ := store.Snapshot("char_demo")
	store.FailNext("inventory_write")
	request := craftRequest("craft_recover", 1)
	if _, err := engine.Craft(context.Background(), actor, request); !errors.Is(err, ErrStorage) {
		t.Fatalf("first Craft error = %v, want storage failure", err)
	}
	unchanged, _ := store.Snapshot("char_demo")
	if !reflect.DeepEqual(before, unchanged) {
		t.Fatal("inventory changed after failed transaction")
	}
	if _, exists := store.LookupOperation(OperationScope{CharacterID: "char_demo", Type: OperationCraft, Key: request.IdempotencyKey}); exists {
		t.Fatal("failed storage operation was recorded as a domain result")
	}
	result, err := engine.Craft(context.Background(), actor, request)
	if err != nil || result.Status != OperationCompleted {
		t.Fatalf("retry = %#v, %v", result, err)
	}
}

func TestCraftTwoSessionsSerializeSameCharacter(t *testing.T) {
	store, engine, actor := newCraftFixture(t, []Slot{{ItemID: "demo:fiber", Quantity: 2}, {ItemID: "demo:resin", Quantity: 1}, {}}, demoRecipe())
	results := make([]OperationResult, 2)
	errorsSeen := make([]error, 2)
	var wait sync.WaitGroup
	for index := range results {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			request := craftRequest(IdempotencyKey(fmt.Sprintf("craft_session_%d", index)), 1)
			results[index], errorsSeen[index] = engine.Craft(context.Background(), actor, request)
		}(index)
	}
	wait.Wait()
	completed, rejected := 0, 0
	for index, result := range results {
		if errorsSeen[index] != nil {
			t.Fatalf("result[%d] err=%v", index, errorsSeen[index])
		}
		if result.Status == OperationCompleted {
			completed++
		} else if result.Code == CodeInsufficientMaterials {
			rejected++
		}
	}
	if completed != 1 || rejected != 1 {
		t.Fatalf("completed=%d insufficient=%d", completed, rejected)
	}
	final, _ := store.Snapshot("char_demo")
	if inventoryQuantity(final, "demo:fiber") != 0 || inventoryQuantity(final, "demo:resin") != 0 || inventoryQuantity(final, "demo:widget") != 1 {
		t.Fatalf("unexpected inventory after race: %#v", final.Slots)
	}
}

func inventoryQuantity(inventory Inventory, itemID ItemID) int64 {
	var quantity int64
	for _, slot := range inventory.Slots {
		if slot.ItemID == itemID {
			quantity += slot.Quantity
		}
	}
	return quantity
}

func TestCraftPartialMaterialConsumptionRollsBack(t *testing.T) {
	store, engine, actor := newCraftFixture(t, []Slot{{ItemID: "demo:fiber", Quantity: 2}, {}, {}}, demoRecipe())
	before, _ := store.Snapshot("char_demo")
	result, err := engine.Craft(context.Background(), actor, craftRequest("craft_partial", 1))
	after, _ := store.Snapshot("char_demo")
	if err != nil || result.Code != CodeInsufficientMaterials || !reflect.DeepEqual(before, after) {
		t.Fatalf("result=%#v err=%v state_changed=%v", result, err, !reflect.DeepEqual(before, after))
	}
}

func TestCraftSameKeyDifferentPayloadConflicts(t *testing.T) {
	_, engine, actor := newCraftFixture(t, []Slot{{ItemID: "demo:fiber", Quantity: 10}, {ItemID: "demo:resin", Quantity: 10}, {}}, demoRecipe())
	if _, err := engine.Craft(context.Background(), actor, craftRequest("craft_conflict", 1)); err != nil {
		t.Fatal(err)
	}
	result, err := engine.Craft(context.Background(), actor, craftRequest("craft_conflict", 2))
	if err != nil || result.Code != CodeIdempotencyConflict || result.Status != OperationRejected {
		t.Fatalf("Craft() = %#v, %v", result, err)
	}
}

func TestCraftNegativeQuantityRejected(t *testing.T) {
	_, engine, actor := newCraftFixture(t, []Slot{{ItemID: "demo:fiber", Quantity: 5}, {ItemID: "demo:resin", Quantity: 5}}, demoRecipe())
	result, err := engine.Craft(context.Background(), actor, craftRequest("craft_negative", -1))
	if err != nil || result.Code != CodeInvalidRequest {
		t.Fatalf("Craft() = %#v, %v", result, err)
	}
}

func TestCraftOverflowRejected(t *testing.T) {
	recipe := demoRecipe()
	recipe.Materials[0].Quantity = math.MaxInt64
	store, engine, actor := newCraftFixture(t, []Slot{{ItemID: "demo:fiber", Quantity: 1}}, recipe)
	result, err := engine.Craft(context.Background(), actor, craftRequest("craft_overflow", 2))
	if err != nil || result.Code != CodeInvalidRequest {
		t.Fatalf("Craft() = %#v, %v", result, err)
	}
	if inventory, _ := store.Snapshot("char_demo"); inventory.Slots[0].Quantity != 1 {
		t.Fatalf("inventory changed: %#v", inventory.Slots)
	}
}

func TestCraftWriteFailuresRollbackInventoryAndOperation(t *testing.T) {
	for _, point := range []string{"inventory_write", "operation_write", "commit"} {
		t.Run(point, func(t *testing.T) {
			store, engine, actor := newCraftFixture(t, []Slot{{ItemID: "demo:fiber", Quantity: 2}, {ItemID: "demo:resin", Quantity: 1}, {}}, demoRecipe())
			before, _ := store.Snapshot("char_demo")
			request := craftRequest(IdempotencyKey("craft_fail_"+point), 1)
			store.FailNext(point)
			if _, err := engine.Craft(context.Background(), actor, request); !errors.Is(err, ErrStorage) {
				t.Fatalf("Craft error = %v, want storage failure", err)
			}
			after, _ := store.Snapshot("char_demo")
			if !reflect.DeepEqual(before, after) {
				t.Fatal("inventory partially changed after injected failure")
			}
			if _, exists := store.LookupOperation(OperationScope{CharacterID: "char_demo", Type: OperationCraft, Key: request.IdempotencyKey}); exists {
				t.Fatal("idempotency record committed without inventory")
			}
		})
	}
}

func TestCraftRequiresAuthorizedCharacterAndDemoMode(t *testing.T) {
	_, engine, actor := newCraftFixture(t, []Slot{{ItemID: "demo:fiber", Quantity: 5}, {ItemID: "demo:resin", Quantity: 5}}, demoRecipe())
	request := craftRequest("craft_unauthorized", 1)
	request.CharacterID = "char_other"
	if _, err := engine.Craft(context.Background(), actor, request); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("unauthorized Craft error = %v", err)
	}
	productionEngine, err := NewEngine(engine.repository, EngineOptions{})
	if err != nil {
		t.Fatal(err)
	}
	result, err := productionEngine.Craft(context.Background(), actor, craftRequest("craft_demo_disabled", 1))
	if err != nil || result.Code != CodeDemoRecipeDisabled {
		t.Fatalf("demo recipe result=%#v err=%v", result, err)
	}
}

func TestMemoryStoreFailsClosedAtOperationCapacity(t *testing.T) {
	store, engine, actor := newCraftFixture(t, []Slot{{ItemID: "demo:fiber", Quantity: 10}, {ItemID: "demo:resin", Quantity: 10}, {}}, demoRecipe())
	store.maxOperations = 1
	first := craftRequest("capacity_first", 1)
	if _, err := engine.Craft(context.Background(), actor, first); err != nil {
		t.Fatal(err)
	}
	second := craftRequest("capacity_second", 1)
	if _, err := engine.Craft(context.Background(), actor, second); !errors.Is(err, ErrCapacity) {
		t.Fatalf("second operation error = %v, want capacity", err)
	}
	if inventory, _ := store.Snapshot("char_demo"); inventory.Slots[2].Quantity != 1 {
		t.Fatalf("capacity rejection changed inventory: %#v", inventory.Slots)
	}
}
