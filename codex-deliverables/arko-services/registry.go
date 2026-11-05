package services

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
	"sync/atomic"
)

const CatalogVersion = "1.0.0"

type ServiceType string

const (
	ServiceShop           ServiceType = "shop"
	ServiceTeleport       ServiceType = "teleport"
	ServiceCrafting       ServiceType = "crafting"
	ServiceProduction     ServiceType = "production"
	ServiceQuest          ServiceType = "quest"
	ServiceArena          ServiceType = "arena"
	ServiceAdministration ServiceType = "administration"
)

var (
	ErrUnknownServiceType = errors.New("unknown service type")
	ErrNotFound           = errors.New("service descriptor not found")
	refPattern            = regexp.MustCompile(`^[a-z][a-z0-9]*([._/-][a-z0-9]+)*$`)
	knownServiceTypes     = map[ServiceType]struct{}{
		ServiceShop:           {},
		ServiceTeleport:       {},
		ServiceCrafting:       {},
		ServiceProduction:     {},
		ServiceQuest:          {},
		ServiceArena:          {},
		ServiceAdministration: {},
	}
)

// Descriptor records versioned contract metadata only. It does not configure or
// execute the referenced game service.
type Descriptor struct {
	ContractVersion string      `json:"contract_version"`
	ServiceRef      string      `json:"service_ref"`
	Revision        uint64      `json:"revision"`
	ServiceType     ServiceType `json:"service_type"`
	References      []string    `json:"references"`
	DemoLabel       string      `json:"demo_label"`
	DemoOnly        bool        `json:"demo_only"`
}

// Catalog is a versioned set of service descriptors.
type Catalog struct {
	CatalogVersion string       `json:"catalog_version"`
	Descriptors    []Descriptor `json:"descriptors"`
}

type snapshot struct {
	version     string
	descriptors []Descriptor
	byRef       map[string]int
}

// Registry publishes complete validated snapshots atomically. Its zero value is
// safe for reads and reloads.
type Registry struct {
	current atomic.Pointer[snapshot]
}

func NewRegistry() *Registry {
	return &Registry{}
}

// ReloadJSON decodes and validates a complete catalog before publishing it.
// On any error, the previously published snapshot remains unchanged.
func (r *Registry) ReloadJSON(reader io.Reader) error {
	if reader == nil {
		return errors.New("catalog reader is nil")
	}

	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	var catalog Catalog
	if err := decoder.Decode(&catalog); err != nil {
		return fmt.Errorf("decode catalog: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("decode catalog: multiple JSON values")
		}
		return fmt.Errorf("decode catalog trailer: %w", err)
	}

	next, err := buildSnapshot(catalog)
	if err != nil {
		return err
	}
	r.current.Store(next)
	return nil
}

// ReloadBytes is a convenience wrapper for callers that already have catalog bytes.
func (r *Registry) ReloadBytes(data []byte) error {
	return r.ReloadJSON(bytes.NewReader(data))
}

// CatalogVersion returns the version of the active snapshot, or an empty string
// before the first successful load.
func (r *Registry) CatalogVersion() string {
	current := r.current.Load()
	if current == nil {
		return ""
	}
	return current.version
}

// List returns defensive copies sorted by service_ref, then service_type.
func (r *Registry) List() []Descriptor {
	current := r.current.Load()
	if current == nil {
		return []Descriptor{}
	}
	return cloneDescriptors(current.descriptors)
}

// ListByType returns defensive copies sorted by service_ref.
func (r *Registry) ListByType(serviceType ServiceType) ([]Descriptor, error) {
	if !IsKnownServiceType(serviceType) {
		return nil, fmt.Errorf("%w: %q", ErrUnknownServiceType, serviceType)
	}
	current := r.current.Load()
	if current == nil {
		return []Descriptor{}, nil
	}

	result := make([]Descriptor, 0)
	for _, descriptor := range current.descriptors {
		if descriptor.ServiceType == serviceType {
			result = append(result, cloneDescriptor(descriptor))
		}
	}
	return result, nil
}

// Lookup returns a defensive copy of the descriptor for ref.
func (r *Registry) Lookup(ref string) (Descriptor, error) {
	current := r.current.Load()
	if current == nil {
		return Descriptor{}, fmt.Errorf("%w: %q", ErrNotFound, ref)
	}
	index, ok := current.byRef[ref]
	if !ok {
		return Descriptor{}, fmt.Errorf("%w: %q", ErrNotFound, ref)
	}
	return cloneDescriptor(current.descriptors[index]), nil
}

func IsKnownServiceType(serviceType ServiceType) bool {
	_, ok := knownServiceTypes[serviceType]
	return ok
}

func buildSnapshot(catalog Catalog) (*snapshot, error) {
	if catalog.CatalogVersion != CatalogVersion {
		return nil, fmt.Errorf("catalog_version must be %q", CatalogVersion)
	}
	if len(catalog.Descriptors) == 0 {
		return nil, errors.New("catalog must contain at least one descriptor")
	}

	descriptors := cloneDescriptors(catalog.Descriptors)
	byRef := make(map[string]int, len(descriptors))
	for i := range descriptors {
		descriptor := &descriptors[i]
		if descriptor.ContractVersion != CatalogVersion {
			return nil, fmt.Errorf("descriptor %q: contract_version must be %q", descriptor.ServiceRef, CatalogVersion)
		}
		if !validRef(descriptor.ServiceRef) {
			return nil, fmt.Errorf("descriptor %q: malformed service_ref", descriptor.ServiceRef)
		}
		if descriptor.Revision == 0 {
			return nil, fmt.Errorf("descriptor %q: revision must be positive", descriptor.ServiceRef)
		}
		if !IsKnownServiceType(descriptor.ServiceType) {
			return nil, fmt.Errorf("descriptor %q: %w %q", descriptor.ServiceRef, ErrUnknownServiceType, descriptor.ServiceType)
		}
		if !descriptor.DemoOnly {
			return nil, fmt.Errorf("descriptor %q: demo_only must be true", descriptor.ServiceRef)
		}
		if descriptor.DemoLabel != "DEMONSTRAÇÃO" {
			return nil, fmt.Errorf("descriptor %q: demo_label must be DEMONSTRAÇÃO", descriptor.ServiceRef)
		}
		if _, exists := byRef[descriptor.ServiceRef]; exists {
			return nil, fmt.Errorf("duplicate service_ref %q", descriptor.ServiceRef)
		}
		byRef[descriptor.ServiceRef] = i

		seenReferences := make(map[string]struct{}, len(descriptor.References))
		for _, ref := range descriptor.References {
			if !validRef(ref) {
				return nil, fmt.Errorf("descriptor %q: malformed reference %q", descriptor.ServiceRef, ref)
			}
			if ref == descriptor.ServiceRef {
				return nil, fmt.Errorf("descriptor %q: self-reference is not allowed", descriptor.ServiceRef)
			}
			if _, exists := seenReferences[ref]; exists {
				return nil, fmt.Errorf("descriptor %q: duplicate reference %q", descriptor.ServiceRef, ref)
			}
			seenReferences[ref] = struct{}{}
		}
	}

	sort.Slice(descriptors, func(i, j int) bool {
		if descriptors[i].ServiceRef == descriptors[j].ServiceRef {
			return descriptors[i].ServiceType < descriptors[j].ServiceType
		}
		return descriptors[i].ServiceRef < descriptors[j].ServiceRef
	})
	for i := range descriptors {
		byRef[descriptors[i].ServiceRef] = i
	}
	return &snapshot{
		version:     catalog.CatalogVersion,
		descriptors: descriptors,
		byRef:       byRef,
	}, nil
}

func validRef(ref string) bool {
	return len(ref) <= 128 && ref == strings.TrimSpace(ref) && refPattern.MatchString(ref)
}

func cloneDescriptor(descriptor Descriptor) Descriptor {
	descriptor.References = append([]string(nil), descriptor.References...)
	return descriptor
}

func cloneDescriptors(descriptors []Descriptor) []Descriptor {
	result := make([]Descriptor, len(descriptors))
	for i, descriptor := range descriptors {
		result[i] = cloneDescriptor(descriptor)
	}
	return result
}
