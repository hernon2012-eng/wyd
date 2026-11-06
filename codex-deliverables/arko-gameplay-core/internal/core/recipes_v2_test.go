package core

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type recipeProviderFunc func(context.Context, RecipeID, uint64) (Recipe, bool, error)

func (fn recipeProviderFunc) ReadRecipe(ctx context.Context, id RecipeID, version uint64) (Recipe, bool, error) {
	return fn(ctx, id, version)
}

type categoryResolverFunc func(context.Context, Actor, CharacterID, string, int64, Inventory) ([]ItemAmount, error)

func (fn categoryResolverFunc) ResolveCategory(ctx context.Context, actor Actor, characterID CharacterID, category string, quantity int64, inventory Inventory) ([]ItemAmount, error) {
	return fn(ctx, actor, characterID, category, quantity, inventory)
}

type typedRequirementCheckerFunc func(context.Context, Actor, CharacterID, []TypedRequirement) (bool, error)

func (fn typedRequirementCheckerFunc) CheckTyped(ctx context.Context, actor Actor, characterID CharacterID, requirements []TypedRequirement) (bool, error) {
	return fn(ctx, actor, characterID, requirements)
}

func newV2CraftFixture(t *testing.T, recipe Recipe, slots []Slot, options EngineOptions) (*MemoryStore, *Engine, Actor) {
	t.Helper()
	store := NewMemoryStore(100)
	for _, definition := range []ItemDefinition{
		{ID: "demo:ore", Name: "Demo Ore", Kind: ItemKindMaterial, MaxStack: 20},
		{ID: "demo:other", Name: "Demo Other", Kind: ItemKindMaterial, MaxStack: 20},
		{ID: "demo:refined", Name: "Demo Refined", Kind: ItemKindGeneric, MaxStack: 20},
		{ID: "demo:byproduct", Name: "Demo Byproduct", Kind: ItemKindGeneric, MaxStack: 20},
	} {
		if err := store.SeedItemDefinition(definition); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.SeedInventory(Inventory{CharacterID: "char_demo", Slots: slots}); err != nil {
		t.Fatal(err)
	}
	options.RecipeProvider = recipeProviderFunc(func(_ context.Context, id RecipeID, version uint64) (Recipe, bool, error) {
		if id != recipe.ID || version != recipe.Version {
			return Recipe{}, false, nil
		}
		return cloneRecipe(recipe), true, nil
	})
	engine, err := NewEngine(store, options)
	if err != nil {
		t.Fatal(err)
	}
	return store, engine, demoActor()
}

func categoryRecipe() Recipe {
	return Recipe{
		ID: "demo:refining", Version: 1, Name: "DEMONSTRAÇÃO - refining",
		OutputMode:        RecipeOutputNewItem,
		MaterialSelectors: []MaterialSelector{{Category: "demo:ore-category", Quantity: 2}},
		Product:           ItemAmount{ItemID: "demo:refined", Quantity: 1}, Active: true, DemoOnly: true,
		TypedRequirements: []TypedRequirement{{Type: "demo:opaque-check", Payload: []byte(`{"flag":true}`)}},
	}
}

func v2Request(key IdempotencyKey, version uint64) CraftRequest {
	return CraftRequest{
		RequestID: "req_v2", IdempotencyKey: key, CharacterID: "char_demo",
		RecipeID: "demo:refining", RecipeVersion: version, Quantity: 1,
	}
}

func TestCraftV2NewItemCategoryResolutionAndTypedRequirement(t *testing.T) {
	recipe := categoryRecipe()
	store, engine, actor := newV2CraftFixture(t, recipe, []Slot{{ItemID: "demo:ore", Quantity: 4}, {}}, EngineOptions{
		AllowDemoRecipes: true,
		MaterialResolver: categoryResolverFunc(func(_ context.Context, _ Actor, character CharacterID, category string, quantity int64, inventory Inventory) ([]ItemAmount, error) {
			if character != "char_demo" || category != "demo:ore-category" || quantity != 2 || inventory.Slots[0].Quantity != 4 {
				t.Fatalf("unexpected resolver input: %q %q %d %#v", character, category, quantity, inventory)
			}
			return []ItemAmount{{ItemID: "demo:ore", Quantity: quantity}}, nil
		}),
		TypedRequirements: typedRequirementCheckerFunc(func(_ context.Context, _ Actor, _ CharacterID, requirements []TypedRequirement) (bool, error) {
			return len(requirements) == 1 && string(requirements[0].Payload) == `{"flag":true}`, nil
		}),
	})
	result, err := engine.Craft(context.Background(), actor, v2Request("v2_transform", 1))
	if err != nil || result.Status != OperationCompleted || !reflect.DeepEqual(result.Consumed, []ItemAmount{{ItemID: "demo:ore", Quantity: 2}}) || !reflect.DeepEqual(result.Produced, []ItemAmount{{ItemID: "demo:refined", Quantity: 1}}) {
		t.Fatalf("Craft() = %#v, %v", result, err)
	}
	inventory, _ := store.Snapshot("char_demo")
	if inventory.Slots[0].Quantity != 2 || inventory.Slots[1].ItemID != "demo:refined" {
		t.Fatalf("transformation inventory = %#v", inventory)
	}
}

func TestCraftV2TransformConsumesSourceAndProducesConfiguredTarget(t *testing.T) {
	source := ItemAmount{ItemID: "demo:ore", Quantity: 1}
	recipe := Recipe{
		ID: "demo:source-transform", Version: 1, Name: "DEMONSTRAÇÃO - source transform",
		OutputMode: RecipeOutputTransform, TransformSource: &source,
		Materials: []ItemAmount{{ItemID: "demo:other", Quantity: 1}},
		Product:   ItemAmount{ItemID: "demo:refined", Quantity: 2}, Active: true, DemoOnly: true,
	}
	store, engine, actor := newV2CraftFixture(t, recipe, []Slot{
		{ItemID: "demo:ore", Quantity: 2},
		{ItemID: "demo:other", Quantity: 2},
		{},
	}, EngineOptions{AllowDemoRecipes: true})
	result, err := engine.Craft(context.Background(), actor, CraftRequest{
		RequestID: "req_transform", IdempotencyKey: "transform_once", CharacterID: "char_demo",
		RecipeID: recipe.ID, RecipeVersion: recipe.Version, Quantity: 1,
	})
	if err != nil || result.Status != OperationCompleted ||
		!reflect.DeepEqual(result.Consumed, []ItemAmount{{ItemID: "demo:ore", Quantity: 1}, {ItemID: "demo:other", Quantity: 1}}) ||
		!reflect.DeepEqual(result.Produced, []ItemAmount{{ItemID: "demo:refined", Quantity: 2}}) {
		t.Fatalf("transform Craft() = %#v, %v", result, err)
	}
	inventory, _ := store.Snapshot("char_demo")
	if inventory.Slots[0].Quantity != 1 || inventory.Slots[1].Quantity != 1 || inventory.Slots[2] != (Slot{ItemID: "demo:refined", Quantity: 2}) {
		t.Fatalf("transform inventory = %#v", inventory)
	}
}

func TestCraftV2TransformRollbackAfterPartialTargetDelivery(t *testing.T) {
	source := ItemAmount{ItemID: "demo:ore", Quantity: 1}
	recipe := Recipe{
		ID: "demo:source-transform", Version: 1, Name: "DEMONSTRAÇÃO - source transform rollback",
		OutputMode: RecipeOutputTransform, TransformSource: &source,
		Materials: []ItemAmount{{ItemID: "demo:ore", Quantity: 1}},
		Product:   ItemAmount{ItemID: "demo:refined", Quantity: 21}, Active: true, DemoOnly: true,
	}
	store, engine, actor := newV2CraftFixture(t, recipe, []Slot{{ItemID: "demo:ore", Quantity: 2}}, EngineOptions{AllowDemoRecipes: true})
	before, _ := store.Snapshot("char_demo")
	result, err := engine.Craft(context.Background(), actor, CraftRequest{
		RequestID: "req_transform_rollback", IdempotencyKey: "transform_rollback", CharacterID: "char_demo",
		RecipeID: recipe.ID, RecipeVersion: recipe.Version, Quantity: 1,
	})
	after, _ := store.Snapshot("char_demo")
	if err != nil || result.Code != CodeInventoryFull || !reflect.DeepEqual(before, after) {
		t.Fatalf("transform rollback = %#v, %v, state changed=%v", result, err, !reflect.DeepEqual(before, after))
	}
}

func TestCraftV2NewItemRollbackAfterPartialTargetDelivery(t *testing.T) {
	recipe := Recipe{
		ID: "demo:new-item-rollback", Version: 1, Name: "DEMONSTRAÇÃO - new item rollback",
		OutputMode: RecipeOutputNewItem,
		Materials:  []ItemAmount{{ItemID: "demo:ore", Quantity: 1}},
		Product:    ItemAmount{ItemID: "demo:refined", Quantity: 21}, Active: true, DemoOnly: true,
	}
	store, engine, actor := newV2CraftFixture(t, recipe, []Slot{{ItemID: "demo:ore", Quantity: 1}}, EngineOptions{AllowDemoRecipes: true})
	before, _ := store.Snapshot("char_demo")
	result, err := engine.Craft(context.Background(), actor, CraftRequest{
		RequestID: "req_new_item_rollback", IdempotencyKey: "new_item_rollback", CharacterID: "char_demo",
		RecipeID: recipe.ID, RecipeVersion: recipe.Version, Quantity: 1,
	})
	after, _ := store.Snapshot("char_demo")
	if err != nil || result.Code != CodeInventoryFull || !reflect.DeepEqual(before, after) {
		t.Fatalf("new-item rollback = %#v, %v, state changed=%v", result, err, !reflect.DeepEqual(before, after))
	}
}

func TestCraftV2MissingVersionAndInsufficientMaterials(t *testing.T) {
	recipe := categoryRecipe()
	store, engine, actor := newV2CraftFixture(t, recipe, []Slot{{ItemID: "demo:ore", Quantity: 1}}, EngineOptions{
		AllowDemoRecipes: true,
		MaterialResolver: categoryResolverFunc(func(_ context.Context, _ Actor, _ CharacterID, _ string, quantity int64, _ Inventory) ([]ItemAmount, error) {
			return []ItemAmount{{ItemID: "demo:ore", Quantity: quantity}}, nil
		}),
		TypedRequirements: typedRequirementCheckerFunc(func(context.Context, Actor, CharacterID, []TypedRequirement) (bool, error) { return true, nil }),
	})
	before, _ := store.Snapshot("char_demo")
	missing := v2Request("v2_missing_version", 2)
	result, err := engine.Craft(context.Background(), actor, missing)
	if err != nil || result.Code != CodeRecipeNotFound {
		t.Fatalf("missing-version Craft() = %#v, %v", result, err)
	}
	result, err = engine.Craft(context.Background(), actor, v2Request("v2_insufficient", 1))
	after, _ := store.Snapshot("char_demo")
	if err != nil || result.Code != CodeInsufficientMaterials || !reflect.DeepEqual(before, after) {
		t.Fatalf("insufficient Craft() = %#v, %v, state changed=%v", result, err, !reflect.DeepEqual(before, after))
	}
}

func TestCraftV2MissingOutputModeRejectedWithoutMutation(t *testing.T) {
	recipe := categoryRecipe()
	recipe.OutputMode = ""
	store, engine, actor := newV2CraftFixture(t, recipe, []Slot{{ItemID: "demo:ore", Quantity: 4}}, EngineOptions{AllowDemoRecipes: true})
	before, _ := store.Snapshot("char_demo")
	result, err := engine.Craft(context.Background(), actor, v2Request("v2_missing_output_mode", 1))
	after, _ := store.Snapshot("char_demo")
	if err != nil || result.Code != CodeInvalidRequest || !reflect.DeepEqual(before, after) {
		t.Fatalf("missing output mode = %#v, %v, state changed=%v", result, err, !reflect.DeepEqual(before, after))
	}
}

func TestCraftV2RepeatedRequestAndDifferentContentConflict(t *testing.T) {
	recipe := categoryRecipe()
	resolverCalls := 0
	store, engine, actor := newV2CraftFixture(t, recipe, []Slot{{ItemID: "demo:ore", Quantity: 8}, {}}, EngineOptions{
		AllowDemoRecipes: true,
		MaterialResolver: categoryResolverFunc(func(_ context.Context, _ Actor, _ CharacterID, _ string, quantity int64, _ Inventory) ([]ItemAmount, error) {
			resolverCalls++
			return []ItemAmount{{ItemID: "demo:ore", Quantity: quantity}}, nil
		}),
		TypedRequirements: typedRequirementCheckerFunc(func(context.Context, Actor, CharacterID, []TypedRequirement) (bool, error) { return true, nil }),
	})
	first, err := engine.Craft(context.Background(), actor, v2Request("v2_replay", 1))
	if err != nil {
		t.Fatal(err)
	}
	sameRequest, err := engine.Craft(context.Background(), actor, v2Request("v2_replay", 1))
	if err != nil || !sameRequest.Replayed || sameRequest.RequestID != first.RequestID || sameRequest.OperationID != first.OperationID {
		t.Fatalf("same request replay first=%#v second=%#v err=%v", first, sameRequest, err)
	}
	retry := v2Request("v2_replay", 1)
	retry.RequestID = "req_v2_retry"
	second, err := engine.Craft(context.Background(), actor, retry)
	if err != nil || !second.Replayed || second.RequestID != retry.RequestID || first.OperationID != second.OperationID || resolverCalls != 1 {
		t.Fatalf("replay first=%#v second=%#v err=%v resolver_calls=%d", first, second, err, resolverCalls)
	}
	conflict := v2Request("v2_replay", 1)
	conflict.Quantity = 2
	result, err := engine.Craft(context.Background(), actor, conflict)
	if err != nil || result.Code != CodeIdempotencyConflict {
		t.Fatalf("same-key/different-content = %#v, %v", result, err)
	}
	inventory, _ := store.Snapshot("char_demo")
	if inventory.Slots[0].Quantity != 6 || inventory.Slots[1].Quantity != 1 {
		t.Fatalf("inventory after idempotency checks = %#v", inventory)
	}
}

func TestCraftV2FailsClosedWithoutCategoryPolicyOrTypedChecker(t *testing.T) {
	recipe := categoryRecipe()
	_, engine, actor := newV2CraftFixture(t, recipe, []Slot{{ItemID: "demo:ore", Quantity: 4}}, EngineOptions{AllowDemoRecipes: true})
	result, err := engine.Craft(context.Background(), actor, v2Request("v2_fail_closed", 1))
	if err != nil || result.Code != CodeRequirementsNotMet {
		t.Fatalf("missing typed policy = %#v, %v", result, err)
	}
	_, engine, actor = newV2CraftFixture(t, recipe, []Slot{{ItemID: "demo:ore", Quantity: 4}}, EngineOptions{
		AllowDemoRecipes: true,
		TypedRequirements: typedRequirementCheckerFunc(func(context.Context, Actor, CharacterID, []TypedRequirement) (bool, error) {
			return true, nil
		}),
	})
	result, err = engine.Craft(context.Background(), actor, v2Request("v2_missing_category_resolver", 1))
	if err != nil || result.Code != CodeMaterialResolutionFailed {
		t.Fatalf("missing category policy = %#v, %v", result, err)
	}
}

func TestCraftV2CategoryResolverMustResolveExactQuantity(t *testing.T) {
	recipe := categoryRecipe()
	store, engine, actor := newV2CraftFixture(t, recipe, []Slot{{ItemID: "demo:ore", Quantity: 4}}, EngineOptions{
		AllowDemoRecipes: true,
		MaterialResolver: categoryResolverFunc(func(context.Context, Actor, CharacterID, string, int64, Inventory) ([]ItemAmount, error) {
			return []ItemAmount{{ItemID: "demo:ore", Quantity: 1}}, nil
		}),
		TypedRequirements: typedRequirementCheckerFunc(func(context.Context, Actor, CharacterID, []TypedRequirement) (bool, error) { return true, nil }),
	})
	result, err := engine.Craft(context.Background(), actor, v2Request("v2_bad_allocation", 1))
	inventory, _ := store.Snapshot("char_demo")
	if err != nil || result.Code != CodeMaterialResolutionFailed || inventory.Slots[0].Quantity != 4 {
		t.Fatalf("bad allocation = %#v, %v, inventory=%#v", result, err, inventory)
	}
}

func TestCraftV2ResolverInfrastructureErrorIsNotMemorized(t *testing.T) {
	recipe := categoryRecipe()
	store, engine, actor := newV2CraftFixture(t, recipe, []Slot{{ItemID: "demo:ore", Quantity: 4}}, EngineOptions{
		AllowDemoRecipes: true,
		MaterialResolver: categoryResolverFunc(func(context.Context, Actor, CharacterID, string, int64, Inventory) ([]ItemAmount, error) {
			return nil, errors.New("policy backend unavailable")
		}),
		TypedRequirements: typedRequirementCheckerFunc(func(context.Context, Actor, CharacterID, []TypedRequirement) (bool, error) {
			return true, nil
		}),
	})
	request := v2Request("v2_resolver_error", 1)
	before, _ := store.Snapshot("char_demo")
	_, err := engine.Craft(context.Background(), actor, request)
	after, _ := store.Snapshot("char_demo")
	if !errors.Is(err, ErrStorage) || !reflect.DeepEqual(before, after) {
		t.Fatalf("resolver error=%v, state changed=%v", err, !reflect.DeepEqual(before, after))
	}
	if _, exists := store.LookupOperation(OperationScope{CharacterID: "char_demo", Type: OperationCraft, Key: request.IdempotencyKey}); exists {
		t.Fatal("infrastructure failure was memorized as a terminal operation")
	}
}

func TestCraftV2InvalidTypedPayloadIsRejected(t *testing.T) {
	recipe := categoryRecipe()
	recipe.TypedRequirements[0].Payload = []byte("{")
	if ValidateRecipe(recipe) == nil {
		t.Fatal("invalid opaque requirement payload accepted")
	}
}
