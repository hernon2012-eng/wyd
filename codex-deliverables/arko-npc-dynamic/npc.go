// Package npc provides an in-memory, read-only query interface over validated
// versioned NPC definition metadata.
package npc

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"
	"sort"
	"sync/atomic"
	"unicode/utf8"
)

const ContractVersion = "1.0.0"

// Service is a service label allowed by the NPC definition v1 schema.
type Service string

const (
	ServiceShop           Service = "shop"
	ServiceTeleport       Service = "teleport"
	ServiceCrafting       Service = "crafting"
	ServiceProduction     Service = "production"
	ServiceQuest          Service = "quest"
	ServiceArena          Service = "arena"
	ServiceAdministration Service = "administration"
)

// Definition is the metadata shape defined by schemas/v1/npc-definition.schema.json.
type Definition struct {
	ContractVersion string    `json:"contract_version"`
	DefinitionRef   string    `json:"definition_ref"`
	Revision        int64     `json:"revision"`
	ModelRef        string    `json:"model_ref"`
	Name            string    `json:"name"`
	Location        Location  `json:"location"`
	Services        []Service `json:"services"`
	Active          bool      `json:"active"`
}

// Location contains opaque map metadata and integer coordinates from the
// contract. This package does not validate map references or coordinate bounds.
type Location struct {
	MapRef string `json:"map_ref"`
	X      int64  `json:"x"`
	Y      int64  `json:"y"`
}

type catalogSnapshot struct {
	definitions map[string]Definition
}

// Catalog atomically replaces complete definition sets. Its zero value is
// ready for use. Do not copy a Catalog after first use.
type Catalog struct {
	current atomic.Pointer[catalogSnapshot]
}

type definitionWire struct {
	ContractVersion *string        `json:"contract_version"`
	DefinitionRef   *string        `json:"definition_ref"`
	Revision        *schemaInteger `json:"revision"`
	ModelRef        *string        `json:"model_ref"`
	Name            *string        `json:"name"`
	Location        *locationWire  `json:"location"`
	Services        *[]string      `json:"services"`
	Active          *bool          `json:"active"`
}

type locationWire struct {
	MapRef *string        `json:"map_ref"`
	X      *schemaInteger `json:"x"`
	Y      *schemaInteger `json:"y"`
}

type schemaInteger int64

func (i *schemaInteger) UnmarshalJSON(data []byte) error {
	rational, ok := new(big.Rat).SetString(string(data))
	if !ok || !rational.IsInt() {
		return errors.New("must be an integer")
	}
	value := rational.Num()
	if !value.IsInt64() {
		return errors.New("integer is outside the supported int64 range")
	}
	*i = schemaInteger(value.Int64())
	return nil
}

var allowedServices = map[Service]struct{}{
	ServiceShop:           {},
	ServiceTeleport:       {},
	ServiceCrafting:       {},
	ServiceProduction:     {},
	ServiceQuest:          {},
	ServiceArena:          {},
	ServiceAdministration: {},
}

// ReloadJSON validates a JSON array of definitions and atomically replaces the
// current catalog only if every definition is valid.
func (c *Catalog) ReloadJSON(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()

	var wire []definitionWire
	if err := decoder.Decode(&wire); err != nil {
		return fmt.Errorf("decode NPC catalog: %w", err)
	}
	if wire == nil {
		return errors.New("NPC catalog must be a JSON array")
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return fmt.Errorf("decode NPC catalog: %w", err)
	}

	definitions := make([]Definition, len(wire))
	for i := range wire {
		definition, err := wire[i].definition()
		if err != nil {
			return fmt.Errorf("definition at index %d: %w", i, err)
		}
		definitions[i] = definition
	}
	return c.Replace(definitions)
}

// ReloadFile reads and validates a complete JSON catalog from path. A read or
// validation error leaves the current snapshot unchanged.
func (c *Catalog) ReloadFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read NPC catalog: %w", err)
	}
	return c.ReloadJSON(data)
}

// Replace validates and atomically installs a complete set of definitions.
// Duplicate definition_ref values reject the whole replacement.
func (c *Catalog) Replace(definitions []Definition) error {
	next := make(map[string]Definition, len(definitions))
	for i, definition := range definitions {
		if err := definition.Validate(); err != nil {
			return fmt.Errorf("definition at index %d: %w", i, err)
		}
		if _, exists := next[definition.DefinitionRef]; exists {
			return fmt.Errorf("duplicate definition_ref %q", definition.DefinitionRef)
		}
		next[definition.DefinitionRef] = cloneDefinition(definition)
	}
	c.current.Store(&catalogSnapshot{definitions: next})
	return nil
}

// Lookup returns an independent copy of the definition with ref.
func (c *Catalog) Lookup(ref string) (Definition, bool) {
	snapshot := c.snapshot()
	definition, ok := snapshot.definitions[ref]
	if !ok {
		return Definition{}, false
	}
	return cloneDefinition(definition), true
}

// List returns independent copies sorted by definition_ref.
func (c *Catalog) List() []Definition {
	snapshot := c.snapshot()
	result := make([]Definition, 0, len(snapshot.definitions))
	for _, definition := range snapshot.definitions {
		result = append(result, cloneDefinition(definition))
	}
	sortDefinitions(result)
	return result
}

// Len returns the number of definitions in the current snapshot.
func (c *Catalog) Len() int {
	return len(c.snapshot().definitions)
}

func (c *Catalog) snapshot() *catalogSnapshot {
	if snapshot := c.current.Load(); snapshot != nil {
		return snapshot
	}
	return &catalogSnapshot{definitions: map[string]Definition{}}
}

func (w definitionWire) definition() (Definition, error) {
	if w.ContractVersion == nil || w.DefinitionRef == nil || w.Revision == nil ||
		w.ModelRef == nil || w.Name == nil || w.Location == nil ||
		w.Services == nil || w.Active == nil {
		return Definition{}, errors.New("missing required property")
	}
	if w.Location.MapRef == nil || w.Location.X == nil || w.Location.Y == nil {
		return Definition{}, errors.New("location is missing a required property")
	}

	services := make([]Service, len(*w.Services))
	for i, service := range *w.Services {
		services[i] = Service(service)
	}
	definition := Definition{
		ContractVersion: *w.ContractVersion,
		DefinitionRef:   *w.DefinitionRef,
		Revision:        int64(*w.Revision),
		ModelRef:        *w.ModelRef,
		Name:            *w.Name,
		Location: Location{
			MapRef: *w.Location.MapRef,
			X:      int64(*w.Location.X),
			Y:      int64(*w.Location.Y),
		},
		Services: services,
		Active:   *w.Active,
	}
	if err := definition.Validate(); err != nil {
		return Definition{}, err
	}
	return definition, nil
}

// Validate checks the constraints expressed by npc-definition.schema.json.
func (d Definition) Validate() error {
	if d.ContractVersion != ContractVersion {
		return fmt.Errorf("contract_version must be %q", ContractVersion)
	}
	if err := validateString("definition_ref", d.DefinitionRef, 1, 128); err != nil {
		return err
	}
	if d.Revision < 1 {
		return errors.New("revision must be at least 1")
	}
	if err := validateString("model_ref", d.ModelRef, 1, 128); err != nil {
		return err
	}
	if err := validateString("name", d.Name, 1, 64); err != nil {
		return err
	}
	if err := validateString("location.map_ref", d.Location.MapRef, 1, 128); err != nil {
		return err
	}
	if d.Services == nil {
		return errors.New("services must be an array")
	}

	seen := make(map[Service]struct{}, len(d.Services))
	for _, service := range d.Services {
		if _, ok := allowedServices[service]; !ok {
			return fmt.Errorf("services contains unsupported value %q", service)
		}
		if _, exists := seen[service]; exists {
			return fmt.Errorf("services contains duplicate value %q", service)
		}
		seen[service] = struct{}{}
	}
	return nil
}

func validateString(name, value string, min, max int) error {
	if !utf8.ValidString(value) {
		return fmt.Errorf("%s must be valid UTF-8", name)
	}
	length := utf8.RuneCountInString(value)
	if length < min || length > max {
		return fmt.Errorf("%s length must be between %d and %d characters", name, min, max)
	}
	return nil
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("unexpected data after catalog")
		}
		return err
	}
	return nil
}

func cloneDefinition(definition Definition) Definition {
	if definition.Services != nil {
		services := make([]Service, len(definition.Services))
		copy(services, definition.Services)
		definition.Services = services
	}
	return definition
}

func sortDefinitions(definitions []Definition) {
	sort.Slice(definitions, func(i, j int) bool {
		return definitions[i].DefinitionRef < definitions[j].DefinitionRef
	})
}
