package services

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
)

const validCatalog = `{
  "catalog_version":"1.0.0",
  "descriptors":[
    {"contract_version":"1.0.0","service_ref":"demo.shop","revision":1,"service_type":"shop","references":["demo.vendor"],"demo_label":"DEMONSTRAÇÃO","demo_only":true},
    {"contract_version":"1.0.0","service_ref":"demo.crafting","revision":2,"service_type":"crafting","references":[],"demo_label":"DEMONSTRAÇÃO","demo_only":true}
  ]
}`

func TestReloadAndSortedDefensiveReads(t *testing.T) {
	registry := NewRegistry()
	if err := registry.ReloadJSON(strings.NewReader(validCatalog)); err != nil {
		t.Fatal(err)
	}
	if got := registry.CatalogVersion(); got != CatalogVersion {
		t.Fatalf("CatalogVersion() = %q", got)
	}

	descriptors := registry.List()
	if len(descriptors) != 2 || descriptors[0].ServiceRef != "demo.crafting" || descriptors[1].ServiceRef != "demo.shop" {
		t.Fatalf("List() is not sorted: %#v", descriptors)
	}
	descriptors[1].References[0] = "changed"
	descriptors[0].ServiceRef = "changed"
	found, err := registry.Lookup("demo.shop")
	if err != nil {
		t.Fatal(err)
	}
	if found.References[0] != "demo.vendor" {
		t.Fatalf("Lookup returned aliased reference slice: %#v", found)
	}

	crafting, err := registry.ListByType(ServiceCrafting)
	if err != nil {
		t.Fatal(err)
	}
	if len(crafting) != 1 || crafting[0].ServiceRef != "demo.crafting" {
		t.Fatalf("ListByType() = %#v", crafting)
	}
}

func TestFailedReloadPreservesPublishedSnapshot(t *testing.T) {
	registry := NewRegistry()
	if err := registry.ReloadJSON(strings.NewReader(validCatalog)); err != nil {
		t.Fatal(err)
	}
	before := registry.List()

	for _, invalid := range []string{
		`{"catalog_version":"1.0.0","descriptors":[{"contract_version":"1.0.0","service_ref":"demo.unknown","revision":1,"service_type":"bank","references":[],"demo_label":"DEMONSTRAÇÃO","demo_only":true}]}`,
		`{"catalog_version":"1.0.0","descriptors":[{"contract_version":"1.0.0","service_ref":"Bad Ref","revision":1,"service_type":"shop","references":[],"demo_label":"DEMONSTRAÇÃO","demo_only":true}]}`,
		`{"catalog_version":"1.0.0","descriptors":[{"contract_version":"1.0.0","service_ref":"demo.same","revision":1,"service_type":"shop","references":[],"demo_label":"DEMONSTRAÇÃO","demo_only":true},{"contract_version":"1.0.0","service_ref":"demo.same","revision":2,"service_type":"quest","references":[],"demo_label":"DEMONSTRAÇÃO","demo_only":true}]}`,
		`{"catalog_version":"1.0.0","descriptors":[{"contract_version":"1.0.0","service_ref":"demo.one","revision":1,"service_type":"shop","references":["demo.ref","demo.ref"],"demo_label":"DEMONSTRAÇÃO","demo_only":true}]}`,
		`{"catalog_version":"1.0.0","descriptors":[{"contract_version":"1.0.0","service_ref":"demo.one","revision":1,"service_type":"shop","references":["Bad Ref"],"demo_label":"DEMONSTRAÇÃO","demo_only":true}]}`,
		`{"catalog_version":"1.0.0","descriptors":[{"contract_version":"1.0.0","service_ref":"demo.one","revision":1,"service_type":"shop","references":["demo.one"],"demo_label":"DEMONSTRAÇÃO","demo_only":true}]}`,
		`{"catalog_version":"1.0.0","descriptors":[{"contract_version":"1.0.0","service_ref":"demo.one","revision":1,"service_type":"shop","references":[],"demo_label":"DEMONSTRAÇÃO","demo_only":false}]}`,
		`{"catalog_version":"1.0.0","descriptors":[{"contract_version":"1.0.0","service_ref":"demo.one","revision":0,"service_type":"shop","references":[],"demo_label":"DEMONSTRAÇÃO","demo_only":true}]}`,
		`{"catalog_version":"1.0.0","descriptors":[{"contract_version":"1.0.0","service_ref":"demo.one","revision":1,"service_type":"shop","references":[],"demo_label":"not-a-demo","demo_only":true}]}`,
		`{"catalog_version":"1.0.0","descriptors":[]}`,
		`{"catalog_version":"1.0.0","descriptors":[]} {}`,
		`{"catalog_version":"9.0.0","descriptors":[]}`,
		`{"catalog_version":"1.0.0","descriptors":[{"contract_version":"1.0.0","service_ref":"demo.one","revision":1,"service_type":"shop","references":[],"demo_label":"DEMONSTRAÇÃO","demo_only":true,"extra":true}]}`,
	} {
		if err := registry.ReloadJSON(strings.NewReader(invalid)); err == nil {
			t.Errorf("ReloadJSON accepted invalid input: %s", invalid)
		}
		if got := registry.List(); !equalDescriptors(got, before) {
			t.Fatalf("failed reload changed active snapshot: got %#v, want %#v", got, before)
		}
	}
}

func TestQueriesRejectUnknownTypeAndReportMissingRef(t *testing.T) {
	registry := NewRegistry()
	if _, err := registry.ListByType(ServiceType("bank")); !errors.Is(err, ErrUnknownServiceType) {
		t.Fatalf("ListByType error = %v, want ErrUnknownServiceType", err)
	}
	if _, err := registry.Lookup("missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Lookup error = %v, want ErrNotFound", err)
	}
}

func TestAllKnownServiceTypesAccepted(t *testing.T) {
	types := []ServiceType{
		ServiceShop,
		ServiceTeleport,
		ServiceCrafting,
		ServiceProduction,
		ServiceQuest,
		ServiceArena,
		ServiceAdministration,
	}
	for _, serviceType := range types {
		if !IsKnownServiceType(serviceType) {
			t.Errorf("service type %q was not recognized", serviceType)
		}
	}
}

func TestConcurrentReadsAndReloads(t *testing.T) {
	registry := NewRegistry()
	if err := registry.ReloadJSON(strings.NewReader(validCatalog)); err != nil {
		t.Fatal(err)
	}
	alternateCatalog := strings.ReplaceAll(validCatalog, "demo.shop", "demo.store")
	const workers = 12
	const iterations = 100

	var wait sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		wait.Add(1)
		go func(worker int) {
			defer wait.Done()
			for i := 0; i < iterations; i++ {
				if worker%3 == 0 {
					data := validCatalog
					if i%2 == 0 {
						data = alternateCatalog
					}
					if err := registry.ReloadJSON(strings.NewReader(data)); err != nil {
						t.Errorf("concurrent reload failed: %v", err)
						return
					}
					continue
				}
				listed := registry.List()
				if len(listed) != 2 {
					t.Errorf("List() returned %d descriptors", len(listed))
					return
				}
				for j := 1; j < len(listed); j++ {
					if listed[j-1].ServiceRef > listed[j].ServiceRef {
						t.Errorf("List() is not sorted: %#v", listed)
						return
					}
				}
				if _, err := registry.ListByType(ServiceShop); err != nil {
					t.Errorf("ListByType() failed: %v", err)
					return
				}
			}
		}(worker)
	}
	wait.Wait()
}

func TestRegistryCanBeUsedAsZeroValue(t *testing.T) {
	var registry Registry
	if got := registry.List(); len(got) != 0 {
		t.Fatalf("zero-value List() = %#v", got)
	}
	if err := registry.ReloadJSON(strings.NewReader(validCatalog)); err != nil {
		t.Fatal(err)
	}
	if got := registry.List(); len(got) != 2 {
		t.Fatalf("List() after reload has %d descriptors", len(got))
	}
}

func TestConcurrentReloadFailureDoesNotPublish(t *testing.T) {
	registry := NewRegistry()
	if err := registry.ReloadJSON(strings.NewReader(validCatalog)); err != nil {
		t.Fatal(err)
	}
	invalid := `{"catalog_version":"1.0.0","descriptors":[{"contract_version":"1.0.0","service_ref":"demo.bad","revision":1,"service_type":"not-known","references":[],"demo_label":"DEMONSTRAÇÃO","demo_only":true}]}`
	if err := registry.ReloadJSON(strings.NewReader(invalid)); err == nil {
		t.Fatal("invalid reload unexpectedly succeeded")
	}
	if got := registry.CatalogVersion(); got != CatalogVersion {
		t.Fatalf("CatalogVersion() after failed reload = %q", got)
	}
	if _, err := registry.Lookup("demo.shop"); err != nil {
		t.Fatal(fmt.Errorf("active descriptor missing after failed reload: %w", err))
	}
}

func equalDescriptors(a, b []Descriptor) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].ContractVersion != b[i].ContractVersion ||
			a[i].ServiceRef != b[i].ServiceRef ||
			a[i].Revision != b[i].Revision ||
			a[i].ServiceType != b[i].ServiceType ||
			a[i].DemoLabel != b[i].DemoLabel ||
			a[i].DemoOnly != b[i].DemoOnly ||
			len(a[i].References) != len(b[i].References) {
			return false
		}
		for j := range a[i].References {
			if a[i].References[j] != b[i].References[j] {
				return false
			}
		}
	}
	return true
}
