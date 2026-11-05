package core

import (
	"math"
	"sort"
	"strings"
)

func ValidateInventory(inventory Inventory, definitions map[ItemID]ItemDefinition) error {
	if !validInventoryShape(inventory) {
		return ErrInvalidRequest
	}
	for _, slot := range inventory.Slots {
		if slot.ItemID == "" {
			if slot.Quantity != 0 {
				return ErrInvalidRequest
			}
			continue
		}
		definition, ok := definitions[slot.ItemID]
		if !ok || !validDefinition(definition) || slot.Quantity <= 0 || slot.Quantity > definition.MaxStack {
			return ErrInvalidRequest
		}
	}
	return nil
}

func AddItems(inventory *Inventory, definition ItemDefinition, quantity int64) error {
	if inventory == nil || !validDefinition(definition) || quantity <= 0 || !validInventoryShape(*inventory) {
		return ErrInvalidRequest
	}
	updated := cloneInventory(*inventory)
	remaining := quantity
	for index := range updated.Slots {
		slot := &updated.Slots[index]
		if slot.ItemID != definition.ID {
			continue
		}
		if slot.Quantity > definition.MaxStack {
			return ErrInvalidRequest
		}
		space := definition.MaxStack - slot.Quantity
		if space <= 0 {
			continue
		}
		added := min(space, remaining)
		slot.Quantity += added
		remaining -= added
		if remaining == 0 {
			*inventory = updated
			return nil
		}
	}
	for index := range updated.Slots {
		slot := &updated.Slots[index]
		if slot.ItemID != "" {
			continue
		}
		added := min(definition.MaxStack, remaining)
		slot.ItemID = definition.ID
		slot.Quantity = added
		remaining -= added
		if remaining == 0 {
			*inventory = updated
			return nil
		}
	}
	return ErrInventoryFull
}

func RemoveItems(inventory *Inventory, itemID ItemID, quantity int64) error {
	if inventory == nil || !validID(string(itemID)) || quantity <= 0 || !validInventoryShape(*inventory) {
		return ErrInvalidRequest
	}
	updated := cloneInventory(*inventory)
	remaining := quantity
	for index := range updated.Slots {
		slot := &updated.Slots[index]
		if slot.ItemID != itemID {
			continue
		}
		removed := min(slot.Quantity, remaining)
		slot.Quantity -= removed
		remaining -= removed
		if slot.Quantity == 0 {
			slot.ItemID = ""
		}
		if remaining == 0 {
			*inventory = updated
			return nil
		}
	}
	return ErrInsufficientItems
}

func TransferItems(source, destination *Inventory, definition ItemDefinition, quantity int64) error {
	if source == nil || destination == nil || source.CharacterID == destination.CharacterID || !validDefinition(definition) || quantity <= 0 || !validInventoryShape(*source) || !validInventoryShape(*destination) {
		return ErrInvalidRequest
	}
	updatedSource := cloneInventory(*source)
	updatedDestination := cloneInventory(*destination)
	if err := RemoveItems(&updatedSource, definition.ID, quantity); err != nil {
		return err
	}
	if err := AddItems(&updatedDestination, definition, quantity); err != nil {
		return err
	}
	*source = updatedSource
	*destination = updatedDestination
	return nil
}

func PlanCraft(inventory Inventory, recipe Recipe, batches int64, definitions map[ItemID]ItemDefinition) (Inventory, []ItemAmount, ItemAmount, ErrorCode) {
	if !validRecipe(recipe) || batches <= 0 || batches > MaxBatches || ValidateInventory(inventory, definitions) != nil {
		return Inventory{}, nil, ItemAmount{}, CodeInvalidRequest
	}
	required := make(map[ItemID]int64, len(recipe.Materials))
	for _, material := range recipe.Materials {
		definition, ok := definitions[material.ItemID]
		if !ok || !validDefinition(definition) || material.Quantity <= 0 {
			return Inventory{}, nil, ItemAmount{}, CodeInvalidRequest
		}
		batchQuantity, ok := checkedMul(material.Quantity, batches)
		if !ok {
			return Inventory{}, nil, ItemAmount{}, CodeInvalidRequest
		}
		total, ok := checkedAdd(required[material.ItemID], batchQuantity)
		if !ok {
			return Inventory{}, nil, ItemAmount{}, CodeInvalidRequest
		}
		required[material.ItemID] = total
	}
	productDefinition, ok := definitions[recipe.Product.ItemID]
	if !ok || !validDefinition(productDefinition) || recipe.Product.Quantity <= 0 {
		return Inventory{}, nil, ItemAmount{}, CodeInvalidRequest
	}
	productQuantity, ok := checkedMul(recipe.Product.Quantity, batches)
	if !ok {
		return Inventory{}, nil, ItemAmount{}, CodeInvalidRequest
	}

	updated := cloneInventory(inventory)
	materialIDs := make([]string, 0, len(required))
	for itemID := range required {
		materialIDs = append(materialIDs, string(itemID))
	}
	sort.Strings(materialIDs)
	consumed := make([]ItemAmount, 0, len(materialIDs))
	for _, id := range materialIDs {
		itemID := ItemID(id)
		quantity := required[itemID]
		if RemoveItems(&updated, itemID, quantity) != nil {
			return Inventory{}, nil, ItemAmount{}, CodeInsufficientMaterials
		}
		consumed = append(consumed, ItemAmount{ItemID: itemID, Quantity: quantity})
	}
	produced := ItemAmount{ItemID: recipe.Product.ItemID, Quantity: productQuantity}
	if err := AddItems(&updated, productDefinition, productQuantity); err != nil {
		if err == ErrInventoryFull {
			return Inventory{}, nil, ItemAmount{}, CodeInventoryFull
		}
		return Inventory{}, nil, ItemAmount{}, CodeInvalidRequest
	}
	return updated, consumed, produced, ""
}

var ErrInventoryFull = &inventoryError{"inventory has no available slots"}
var ErrInsufficientItems = &inventoryError{"insufficient item quantity"}

type inventoryError struct{ message string }

func (e *inventoryError) Error() string { return e.message }

func validDefinition(definition ItemDefinition) bool {
	return validID(string(definition.ID)) && definition.MaxStack > 0 && definition.Name != "" && len(definition.Name) <= 128 && (definition.Kind == ItemKindGeneric || definition.Kind == ItemKindMaterial || definition.Kind == ItemKindEquipment)
}

func validInventoryShape(inventory Inventory) bool {
	if !validID(string(inventory.CharacterID)) || len(inventory.Slots) > 10000 {
		return false
	}
	for _, slot := range inventory.Slots {
		if slot.ItemID == "" {
			if slot.Quantity != 0 {
				return false
			}
			continue
		}
		if !validID(string(slot.ItemID)) || slot.Quantity <= 0 {
			return false
		}
	}
	return true
}

func validRecipe(recipe Recipe) bool {
	if !validID(string(recipe.ID)) || recipe.Version == 0 || strings.TrimSpace(recipe.Name) == "" || len(recipe.Name) > 128 || len(recipe.Materials) == 0 || len(recipe.Materials) > MaxRecipeMaterials {
		return false
	}
	if !validID(string(recipe.Product.ItemID)) || recipe.Product.Quantity <= 0 {
		return false
	}
	for _, material := range recipe.Materials {
		if !validID(string(material.ItemID)) || material.Quantity <= 0 {
			return false
		}
	}
	for _, requirement := range recipe.Requirements {
		if !validID(requirement.Key) || len(requirement.Value) > 256 {
			return false
		}
	}
	return true
}

func validID(value string) bool {
	if len(value) == 0 || len(value) > 128 {
		return false
	}
	for _, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || strings.ContainsRune("._:-", char) {
			continue
		}
		return false
	}
	return true
}

func checkedAdd(left, right int64) (int64, bool) {
	if left < 0 || right < 0 || left > math.MaxInt64-right {
		return 0, false
	}
	return left + right, true
}

func checkedMul(left, right int64) (int64, bool) {
	if left < 0 || right < 0 || (right != 0 && left > math.MaxInt64/right) {
		return 0, false
	}
	return left * right, true
}

func cloneInventory(inventory Inventory) Inventory {
	inventory.Slots = append([]Slot(nil), inventory.Slots...)
	return inventory
}
