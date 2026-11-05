package core

import (
	"errors"
	"reflect"
	"testing"
)

func demoDefinitions() map[ItemID]ItemDefinition {
	return map[ItemID]ItemDefinition{
		"demo:fiber":  {ID: "demo:fiber", Name: "Demo Fiber", Kind: ItemKindMaterial, MaxStack: 10},
		"demo:resin":  {ID: "demo:resin", Name: "Demo Resin", Kind: ItemKindMaterial, MaxStack: 10},
		"demo:widget": {ID: "demo:widget", Name: "Demo Widget", Kind: ItemKindGeneric, MaxStack: 5},
	}
}

func TestAddItemsIsAtomicWhenInventoryFull(t *testing.T) {
	inventory := Inventory{CharacterID: "char_demo", Slots: []Slot{{ItemID: "demo:fiber", Quantity: 10}}}
	before := cloneInventory(inventory)
	err := AddItems(&inventory, demoDefinitions()["demo:resin"], 1)
	if !errors.Is(err, ErrInventoryFull) {
		t.Fatalf("AddItems error = %v, want inventory full", err)
	}
	if !reflect.DeepEqual(inventory, before) {
		t.Fatalf("inventory changed on rejected add: %#v", inventory)
	}
}

func TestFreeSlotsCountsEmptySlots(t *testing.T) {
	inventory := Inventory{CharacterID: "char_demo", Slots: []Slot{{ItemID: "demo:fiber", Quantity: 1}, {}, {}}}
	free, err := FreeSlots(inventory)
	if err != nil || free != 2 {
		t.Fatalf("FreeSlots() = %d, %v; want 2, nil", free, err)
	}
}

func TestRemoveItemsConsumesSlotsInOrder(t *testing.T) {
	inventory := Inventory{CharacterID: "char_demo", Slots: []Slot{{ItemID: "demo:fiber", Quantity: 2}, {ItemID: "demo:fiber", Quantity: 5}}}
	if err := RemoveItems(&inventory, "demo:fiber", 4); err != nil {
		t.Fatal(err)
	}
	want := []Slot{{}, {ItemID: "demo:fiber", Quantity: 3}}
	if !reflect.DeepEqual(inventory.Slots, want) {
		t.Fatalf("slots = %#v, want %#v", inventory.Slots, want)
	}
}

func TestTransferItemsIsAtomic(t *testing.T) {
	source := Inventory{CharacterID: "char_source", Slots: []Slot{{ItemID: "demo:fiber", Quantity: 4}}}
	destination := Inventory{CharacterID: "char_destination", Slots: []Slot{{ItemID: "demo:resin", Quantity: 10}}}
	beforeSource, beforeDestination := cloneInventory(source), cloneInventory(destination)
	err := TransferItems(&source, &destination, demoDefinitions()["demo:fiber"], 2)
	if !errors.Is(err, ErrInventoryFull) {
		t.Fatalf("TransferItems error = %v, want inventory full", err)
	}
	if !reflect.DeepEqual(source, beforeSource) || !reflect.DeepEqual(destination, beforeDestination) {
		t.Fatal("transfer changed one inventory despite rejection")
	}
}

func TestPlanCraftUsesFreedSlotsAndAggregatesMaterials(t *testing.T) {
	definitions := demoDefinitions()
	inventory := Inventory{CharacterID: "char_demo", Slots: []Slot{{ItemID: "demo:fiber", Quantity: 2}, {ItemID: "demo:resin", Quantity: 2}}}
	recipe := Recipe{ID: "demo:widget_v1", Version: 1, Name: "Demo Widget", Active: true, Materials: []ItemAmount{{ItemID: "demo:fiber", Quantity: 1}, {ItemID: "demo:fiber", Quantity: 1}, {ItemID: "demo:resin", Quantity: 1}}, Product: ItemAmount{ItemID: "demo:widget", Quantity: 1}}
	updated, consumed, produced, code := PlanCraft(inventory, recipe, 1, definitions)
	if code != "" {
		t.Fatalf("PlanCraft code = %q", code)
	}
	if !reflect.DeepEqual(consumed, []ItemAmount{{ItemID: "demo:fiber", Quantity: 2}, {ItemID: "demo:resin", Quantity: 1}}) {
		t.Fatalf("consumed = %#v", consumed)
	}
	if produced != (ItemAmount{ItemID: "demo:widget", Quantity: 1}) || updated.Slots[0] != (Slot{ItemID: "demo:widget", Quantity: 1}) {
		t.Fatalf("unexpected plan: updated=%#v produced=%#v", updated, produced)
	}
	if !reflect.DeepEqual(inventory.Slots, []Slot{{ItemID: "demo:fiber", Quantity: 2}, {ItemID: "demo:resin", Quantity: 2}}) {
		t.Fatal("planning mutated source inventory")
	}
}

func TestPlanCraftRejectsOverflowAndPreservesInput(t *testing.T) {
	definitions := demoDefinitions()
	recipe := Recipe{ID: "demo:overflow", Version: 1, Name: "Overflow", Active: true, Materials: []ItemAmount{{ItemID: "demo:fiber", Quantity: int64(^uint64(0) >> 1)}}, Product: ItemAmount{ItemID: "demo:widget", Quantity: 1}}
	inventory := Inventory{CharacterID: "char_demo", Slots: []Slot{{ItemID: "demo:fiber", Quantity: 1}}}
	before := cloneInventory(inventory)
	_, _, _, code := PlanCraft(inventory, recipe, 2, definitions)
	if code != CodeInvalidRequest || !reflect.DeepEqual(inventory, before) {
		t.Fatalf("code = %q, inventory mutated = %v", code, !reflect.DeepEqual(inventory, before))
	}
}

func FuzzAddRemoveConservesQuantity(f *testing.F) {
	f.Add(uint8(1), uint8(1))
	f.Add(uint8(9), uint8(10))
	f.Fuzz(func(t *testing.T, initial, addition uint8) {
		initialQuantity := int64(initial % 20)
		addQuantity := int64(addition % 20)
		definition := ItemDefinition{ID: "demo:fiber", Name: "Demo Fiber", Kind: ItemKindMaterial, MaxStack: 20}
		inventory := Inventory{CharacterID: "char_demo", Slots: []Slot{{ItemID: "demo:fiber", Quantity: initialQuantity}, {}}}
		if initialQuantity == 0 {
			inventory.Slots[0] = Slot{}
		}
		if addQuantity == 0 {
			return
		}
		if err := AddItems(&inventory, definition, addQuantity); err != nil {
			return
		}
		var total int64
		for _, slot := range inventory.Slots {
			total += slot.Quantity
		}
		if total != initialQuantity+addQuantity {
			t.Fatalf("quantity not conserved: total %d, want %d", total, initialQuantity+addQuantity)
		}
	})
}
