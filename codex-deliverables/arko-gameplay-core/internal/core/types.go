package core

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

type CharacterID string
type ItemID string
type RecipeID string
type OperationType string
type IdempotencyKey string

const (
	OperationCraft     OperationType = "craft"
	OperationAdd       OperationType = "inventory.add"
	OperationRemove    OperationType = "inventory.remove"
	OperationTransfer  OperationType = "inventory.transfer"
	MaxBatches                       = 100
	MaxRecipeMaterials               = 64
)

var (
	ErrInvalidRequest      = errors.New("invalid request")
	ErrUnauthorized        = errors.New("character is not authorized")
	ErrNotFound            = errors.New("not found")
	ErrIdempotencyConflict = errors.New("idempotency key conflict")
	ErrStorage             = errors.New("storage failure")
	ErrRetryableStorage    = errors.New("retryable storage failure")
	ErrCapacity            = errors.New("storage capacity reached")
)

type ErrorCode string

const (
	CodeInvalidRequest           ErrorCode = "invalid_request"
	CodeRecipeNotFound           ErrorCode = "recipe_not_found"
	CodeRecipeInactive           ErrorCode = "recipe_inactive"
	CodeRequirementsNotMet       ErrorCode = "requirements_not_met"
	CodeInsufficientMaterials    ErrorCode = "insufficient_materials"
	CodeInventoryFull            ErrorCode = "inventory_full"
	CodeItemNotFound             ErrorCode = "item_not_found"
	CodeInsufficientItems        ErrorCode = "insufficient_items"
	CodeIdempotencyConflict      ErrorCode = "idempotency_conflict"
	CodeDemoRecipeDisabled       ErrorCode = "demo_recipe_disabled"
	CodeMaterialResolutionFailed ErrorCode = "material_resolution_failed"
)

type ItemKind string

const (
	ItemKindMaterial  ItemKind = "material"
	ItemKindEquipment ItemKind = "equipment"
	ItemKindGeneric   ItemKind = "generic"
)

type ItemDefinition struct {
	ID       ItemID   `json:"item_id"`
	Name     string   `json:"name"`
	Kind     ItemKind `json:"kind"`
	MaxStack int64    `json:"max_stack"`
}

type Slot struct {
	ItemID   ItemID `json:"item_id,omitempty"`
	Quantity int64  `json:"quantity"`
}

type Inventory struct {
	CharacterID CharacterID `json:"character_id"`
	Slots       []Slot      `json:"slots"`
}

type ItemAmount struct {
	ItemID   ItemID `json:"item_id"`
	Quantity int64  `json:"quantity"`
}

type RecipeRequirement struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type MaterialSelector struct {
	ItemID   ItemID `json:"item_id,omitempty"`
	Category string `json:"category,omitempty"`
	Quantity int64  `json:"quantity"`
}

type TypedRequirement struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type RecipeOutputMode string

const (
	RecipeOutputNewItem   RecipeOutputMode = "new_item"
	RecipeOutputTransform RecipeOutputMode = "transform"
)

type Recipe struct {
	ID                RecipeID            `json:"recipe_id"`
	Version           uint64              `json:"version"`
	Name              string              `json:"name"`
	Materials         []ItemAmount        `json:"materials,omitempty"`
	MaterialSelectors []MaterialSelector  `json:"material_selectors,omitempty"`
	OutputMode        RecipeOutputMode    `json:"output_mode,omitempty"`
	TransformSource   *ItemAmount         `json:"transform_source,omitempty"`
	Product           ItemAmount          `json:"product"`
	Requirements      []RecipeRequirement `json:"requirements,omitempty"`
	TypedRequirements []TypedRequirement  `json:"typed_requirements,omitempty"`
	Active            bool                `json:"active"`
	DemoOnly          bool                `json:"demo_only"`
}

type Actor struct {
	PrincipalID          string        `json:"-"`
	AuthorizedCharacters []CharacterID `json:"-"`
}

type OperationScope struct {
	CharacterID CharacterID
	Type        OperationType
	Key         IdempotencyKey
}

type OperationStatus string

const (
	OperationCompleted OperationStatus = "completed"
	OperationRejected  OperationStatus = "rejected"
)

type OperationResult struct {
	OperationID string          `json:"operation_id"`
	RequestID   string          `json:"request_id"`
	Status      OperationStatus `json:"status"`
	Code        ErrorCode       `json:"code,omitempty"`
	Consumed    []ItemAmount    `json:"consumed,omitempty"`
	Produced    []ItemAmount    `json:"produced,omitempty"`
	Replayed    bool            `json:"replayed,omitempty"`
}

type OperationRecord struct {
	Scope       OperationScope
	Fingerprint string
	RequestID   string
	Result      OperationResult
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type CraftRequest struct {
	RequestID      string
	IdempotencyKey IdempotencyKey
	CharacterID    CharacterID
	RecipeID       RecipeID
	RecipeVersion  uint64
	Quantity       int64
}

type AddRequest struct {
	RequestID      string
	IdempotencyKey IdempotencyKey
	CharacterID    CharacterID
	Item           ItemAmount
}

type RemoveRequest struct {
	RequestID      string
	IdempotencyKey IdempotencyKey
	CharacterID    CharacterID
	Item           ItemAmount
}

type TransferRequest struct {
	RequestID      string
	IdempotencyKey IdempotencyKey
	Source         CharacterID
	Destination    CharacterID
	Item           ItemAmount
}

type RequirementChecker interface {
	Check(ctx context.Context, actor Actor, characterID CharacterID, requirements []RecipeRequirement) (bool, error)
}

type TypedRequirementChecker interface {
	CheckTyped(ctx context.Context, actor Actor, characterID CharacterID, requirements []TypedRequirement) (bool, error)
}

type MaterialSelectorResolver interface {
	// quantity is the amount required per batch; the core planner applies batch count.
	ResolveCategory(ctx context.Context, actor Actor, characterID CharacterID, category string, quantity int64, inventory Inventory) ([]ItemAmount, error)
}

type RecipeProvider interface {
	ReadRecipe(ctx context.Context, id RecipeID, version uint64) (Recipe, bool, error)
}
