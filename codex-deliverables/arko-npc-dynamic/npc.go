// Package npc provides an in-memory catalog for validated, versioned NPC
// definition metadata.
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
	"sync"
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
	history     map[string][]RevisionRecord
	published   map[string]int64
}

// RevisionRecord is an immutable catalog history entry. Published reports
// whether this revision has ever been published in this catalog's lifetime.
type RevisionRecord struct {
	Definition Definition
	Published  bool
}

// Catalog atomically manages validated definitions and their in-memory
// revision history. Its zero value is ready for use. Do not copy a Catalog
// after first use.
type Catalog struct {
	mu      sync.Mutex
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
	history := make(map[string][]RevisionRecord, len(next))
	for ref, definition := range next {
		history[ref] = []RevisionRecord{{Definition: cloneDefinition(definition)}}
	}
	c.mu.Lock()
	c.current.Store(&catalogSnapshot{
		definitions: next,
		history:     history,
		published:   make(map[string]int64),
	})
	c.mu.Unlock()
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

// Create adds a new definition as revision 1. A definition reference already
// present in the catalog is rejected.
func (c *Catalog) Create(definition Definition) error {
	if definition.Revision != 1 {
		return errors.New("new definition revision must be 1")
	}
	if err := definition.Validate(); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	current := c.snapshot()
	if _, exists := current.definitions[definition.DefinitionRef]; exists {
		return fmt.Errorf("definition_ref %q already exists", definition.DefinitionRef)
	}
	next := cloneSnapshot(current)
	definition = cloneDefinition(definition)
	next.definitions[definition.DefinitionRef] = definition
	next.history[definition.DefinitionRef] = []RevisionRecord{{Definition: cloneDefinition(definition)}}
	c.current.Store(next)
	return nil
}

// CreateJSON strictly decodes one definition and adds it as revision 1.
func (c *Catalog) CreateJSON(data []byte) error {
	definition, err := decodeDefinition(data)
	if err != nil {
		return err
	}
	return c.Create(definition)
}

// Duplicate copies an existing definition under newRef as revision 1. All
// metadata, including active state and service labels, is copied.
func (c *Catalog) Duplicate(sourceRef, newRef string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	current := c.snapshot()
	source, exists := current.definitions[sourceRef]
	if !exists {
		return fmt.Errorf("definition_ref %q not found", sourceRef)
	}
	if _, exists := current.definitions[newRef]; exists {
		return fmt.Errorf("definition_ref %q already exists", newRef)
	}
	duplicate := cloneDefinition(source)
	duplicate.DefinitionRef = newRef
	duplicate.Revision = 1
	if err := duplicate.Validate(); err != nil {
		return err
	}
	next := cloneSnapshot(current)
	next.definitions[newRef] = duplicate
	next.history[newRef] = []RevisionRecord{{Definition: cloneDefinition(duplicate)}}
	c.current.Store(next)
	return nil
}

// Update replaces a definition's metadata and records a new revision. The
// supplied revision is ignored; the catalog assigns the next revision.
func (c *Catalog) Update(ref string, definition Definition) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	current := c.snapshot()
	old, exists := current.definitions[ref]
	if !exists {
		return fmt.Errorf("definition_ref %q not found", ref)
	}
	if definition.DefinitionRef != ref {
		return fmt.Errorf("updated definition_ref must remain %q", ref)
	}
	revision, err := nextRevision(old.Revision)
	if err != nil {
		return err
	}
	definition.Revision = revision
	if err := definition.Validate(); err != nil {
		return err
	}
	next := cloneSnapshot(current)
	appendRevision(next, definition)
	c.current.Store(next)
	return nil
}

// UpdateJSON strictly decodes one definition and records it as a new revision.
func (c *Catalog) UpdateJSON(ref string, data []byte) error {
	definition, err := decodeDefinition(data)
	if err != nil {
		return err
	}
	return c.Update(ref, definition)
}

// Activate records a revision with active=true. It is a no-op if already active.
func (c *Catalog) Activate(ref string) error {
	return c.setActive(ref, true)
}

// Deactivate records a revision with active=false. It is a no-op if already inactive.
func (c *Catalog) Deactivate(ref string) error {
	return c.setActive(ref, false)
}

func (c *Catalog) setActive(ref string, active bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	current := c.snapshot()
	definition, exists := current.definitions[ref]
	if !exists {
		return fmt.Errorf("definition_ref %q not found", ref)
	}
	if definition.Active == active {
		return nil
	}
	revision, err := nextRevision(definition.Revision)
	if err != nil {
		return err
	}
	definition.Active = active
	definition.Revision = revision
	next := cloneSnapshot(current)
	appendRevision(next, definition)
	c.current.Store(next)
	return nil
}

// Publish marks the current revision as the catalog's published version.
func (c *Catalog) Publish(ref string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	current := c.snapshot()
	definition, exists := current.definitions[ref]
	if !exists {
		return fmt.Errorf("definition_ref %q not found", ref)
	}
	next := cloneSnapshot(current)
	next.published[ref] = definition.Revision
	markPublished(next.history[ref], definition.Revision)
	c.current.Store(next)
	return nil
}

// Unpublish removes the published pointer for ref without changing its
// definition or revision history.
func (c *Catalog) Unpublish(ref string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	current := c.snapshot()
	if _, exists := current.definitions[ref]; !exists {
		return fmt.Errorf("definition_ref %q not found", ref)
	}
	if _, published := current.published[ref]; !published {
		return nil
	}
	next := cloneSnapshot(current)
	delete(next.published, ref)
	c.current.Store(next)
	return nil
}

// Published returns the definition currently published for ref.
func (c *Catalog) Published(ref string) (Definition, bool) {
	snapshot := c.snapshot()
	revision, exists := snapshot.published[ref]
	if !exists {
		return Definition{}, false
	}
	for _, entry := range snapshot.history[ref] {
		if entry.Definition.Revision == revision {
			return cloneDefinition(entry.Definition), true
		}
	}
	return Definition{}, false
}

// Rollback restores metadata from a historical revision as a new revision and
// immediately publishes that new revision. History is never deleted.
func (c *Catalog) Rollback(ref string, revision int64) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	current := c.snapshot()
	definition, exists := current.definitions[ref]
	if !exists {
		return fmt.Errorf("definition_ref %q not found", ref)
	}
	var restored Definition
	found := false
	for _, entry := range current.history[ref] {
		if entry.Definition.Revision == revision {
			restored = cloneDefinition(entry.Definition)
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("revision %d for definition_ref %q not found", revision, ref)
	}
	nextRev, err := nextRevision(definition.Revision)
	if err != nil {
		return err
	}
	restored.Revision = nextRev
	next := cloneSnapshot(current)
	appendRevision(next, restored)
	next.published[ref] = nextRev
	markPublished(next.history[ref], nextRev)
	c.current.Store(next)
	return nil
}

// History returns defensive copies of all retained revisions, oldest first.
func (c *Catalog) History(ref string) []RevisionRecord {
	entries := c.snapshot().history[ref]
	result := make([]RevisionRecord, len(entries))
	for i, entry := range entries {
		result[i] = RevisionRecord{
			Definition: cloneDefinition(entry.Definition),
			Published:  entry.Published,
		}
	}
	return result
}

// ReferenceChecker reports whether a definition is referenced by tracked
// metadata. Implementations must return an error when reference status cannot
// be established.
type ReferenceChecker interface {
	IsReferenced(definitionRef string) (bool, error)
}

// ReferenceCheckFunc adapts a function to ReferenceChecker.
type ReferenceCheckFunc func(definitionRef string) (bool, error)

func (f ReferenceCheckFunc) IsReferenced(definitionRef string) (bool, error) {
	return f(definitionRef)
}

// Remove deletes an unpublished definition only when a non-nil reference
// checker confirms it is not referenced. Missing or failing checks fail closed.
func (c *Catalog) Remove(ref string, checker ReferenceChecker) error {
	if checker == nil {
		return errors.New("reference checker is required to remove a definition")
	}
	if functionChecker, ok := checker.(ReferenceCheckFunc); ok && functionChecker == nil {
		return errors.New("reference checker function is nil")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	current := c.snapshot()
	if _, exists := current.definitions[ref]; !exists {
		return fmt.Errorf("definition_ref %q not found", ref)
	}
	if _, published := current.published[ref]; published {
		return fmt.Errorf("definition_ref %q is published; unpublish it before removal", ref)
	}
	referenced, err := checker.IsReferenced(ref)
	if err != nil {
		return fmt.Errorf("check references for %q: %w", ref, err)
	}
	if referenced {
		return fmt.Errorf("definition_ref %q is referenced", ref)
	}
	next := cloneSnapshot(current)
	delete(next.definitions, ref)
	delete(next.history, ref)
	c.current.Store(next)
	return nil
}

func (c *Catalog) snapshot() *catalogSnapshot {
	if snapshot := c.current.Load(); snapshot != nil {
		return snapshot
	}
	return &catalogSnapshot{
		definitions: map[string]Definition{},
		history:     map[string][]RevisionRecord{},
		published:   map[string]int64{},
	}
}

func decodeDefinition(data []byte) (Definition, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var wire definitionWire
	if err := decoder.Decode(&wire); err != nil {
		return Definition{}, fmt.Errorf("decode NPC definition: %w", err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return Definition{}, fmt.Errorf("decode NPC definition: %w", err)
	}
	definition, err := wire.definition()
	if err != nil {
		return Definition{}, err
	}
	return definition, nil
}

func cloneSnapshot(snapshot *catalogSnapshot) *catalogSnapshot {
	next := &catalogSnapshot{
		definitions: make(map[string]Definition, len(snapshot.definitions)),
		history:     make(map[string][]RevisionRecord, len(snapshot.history)),
		published:   make(map[string]int64, len(snapshot.published)),
	}
	for ref, definition := range snapshot.definitions {
		next.definitions[ref] = cloneDefinition(definition)
	}
	for ref, entries := range snapshot.history {
		copied := make([]RevisionRecord, len(entries))
		for i, entry := range entries {
			copied[i] = RevisionRecord{
				Definition: cloneDefinition(entry.Definition),
				Published:  entry.Published,
			}
		}
		next.history[ref] = copied
	}
	for ref, revision := range snapshot.published {
		next.published[ref] = revision
	}
	return next
}

func appendRevision(snapshot *catalogSnapshot, definition Definition) {
	definition = cloneDefinition(definition)
	snapshot.definitions[definition.DefinitionRef] = definition
	snapshot.history[definition.DefinitionRef] = append(
		snapshot.history[definition.DefinitionRef],
		RevisionRecord{Definition: cloneDefinition(definition)},
	)
}

func markPublished(entries []RevisionRecord, revision int64) {
	for i := range entries {
		if entries[i].Definition.Revision == revision {
			entries[i].Published = true
		}
	}
}

func nextRevision(current int64) (int64, error) {
	if current == int64(^uint64(0)>>1) {
		return 0, errors.New("revision cannot be incremented beyond int64")
	}
	return current + 1, nil
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
