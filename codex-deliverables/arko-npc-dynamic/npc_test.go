package npc

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
)

const validDefinition = `{
  "contract_version": "1.0.0",
  "definition_ref": "demo:npc-one",
  "revision": 1,
  "model_ref": "demo:model",
  "name": "NPC demonstração",
  "location": {"map_ref": "demo:map", "x": 0, "y": -3},
  "services": ["quest"],
  "active": false
}`

func TestReloadDemonstrationFile(t *testing.T) {
	var catalog Catalog
	if err := catalog.ReloadFile("examples/DEMONSTRAÇÃO/catalog.json"); err != nil {
		t.Fatalf("ReloadFile() error = %v", err)
	}
	if got := catalog.Len(); got != 1 {
		t.Fatalf("Len() = %d, want 1", got)
	}
}

func TestReloadJSONSchemaShaped(t *testing.T) {
	var catalog Catalog
	input := `[` + validDefinition + `,` + strings.Replace(validDefinition,
		"demo:npc-one", "demo:npc-two", 1) + `]`
	if err := catalog.ReloadJSON([]byte(input)); err != nil {
		t.Fatalf("ReloadJSON() error = %v", err)
	}
	if got := catalog.Len(); got != 2 {
		t.Fatalf("Len() = %d, want 2", got)
	}

	definitions := catalog.List()
	if definitions[0].DefinitionRef != "demo:npc-one" || definitions[1].DefinitionRef != "demo:npc-two" {
		t.Fatalf("List() is not sorted by definition_ref: %#v", definitions)
	}
	got, ok := catalog.Lookup("demo:npc-one")
	if !ok {
		t.Fatal("Lookup() did not find definition")
	}
	if got.ContractVersion != ContractVersion || got.Revision != 1 || got.Location.X != 0 ||
		got.Location.Y != -3 || got.Active || len(got.Services) != 1 || got.Services[0] != ServiceQuest {
		t.Fatalf("Lookup() returned unexpected definition: %#v", got)
	}

	got.Services[0] = ServiceShop
	again, _ := catalog.Lookup("demo:npc-one")
	if again.Services[0] != ServiceQuest {
		t.Fatal("Lookup() exposed mutable catalog state")
	}
}

func TestReloadJSONAcceptsMathematicalJSONIntegers(t *testing.T) {
	input := strings.Replace(validDefinition, `"revision": 1`, `"revision": 1.0`, 1)
	input = strings.Replace(input, `"x": 0`, `"x": 0e4`, 1)
	input = strings.Replace(input, `"services": ["quest"]`, `"services": []`, 1)
	var catalog Catalog
	if err := catalog.ReloadJSON([]byte(`[` + input + `]`)); err != nil {
		t.Fatalf("ReloadJSON() rejected schema integers written with decimal/exponent notation: %v", err)
	}
	definition, ok := catalog.Lookup("demo:npc-one")
	if !ok || definition.Services == nil || len(definition.Services) != 0 {
		t.Fatalf("empty schema services array was not preserved: %#v", definition)
	}
}

func TestReloadJSONRejectsInvalidInput(t *testing.T) {
	longName := strings.Repeat("x", 65)
	cases := map[string]string{
		"not an array":            `{}`,
		"null catalog":            `null`,
		"unknown top-level field": strings.Replace(validDefinition, `"active": false`, `"active": false, "extra": 1`, 1),
		"unknown nested field":    strings.Replace(validDefinition, `"y": -3`, `"y": -3, "extra": 1`, 1),
		"missing required":        strings.Replace(validDefinition, `  "name": "NPC demonstração",`+"\n", "", 1),
		"wrong contract version":  strings.Replace(validDefinition, `"1.0.0"`, `"2.0.0"`, 1),
		"empty definition ref":    strings.Replace(validDefinition, `"demo:npc-one"`, `""`, 1),
		"long name":               strings.Replace(validDefinition, `"NPC demonstração"`, `"`+longName+`"`, 1),
		"invalid revision":        strings.Replace(validDefinition, `"revision": 1`, `"revision": 0`, 1),
		"fractional revision":     strings.Replace(validDefinition, `"revision": 1`, `"revision": 1.5`, 1),
		"float coordinate":        strings.Replace(validDefinition, `"x": 0`, `"x": 0.5`, 1),
		"invalid service":         strings.Replace(validDefinition, `"quest"`, `"dance"`, 1),
		"duplicate services":      strings.Replace(validDefinition, `"quest"`, `"quest", "quest"`, 1),
		"null services":           strings.Replace(validDefinition, `"services": ["quest"]`, `"services": null`, 1),
		"null active":             strings.Replace(validDefinition, `"active": false`, `"active": null`, 1),
		"trailing JSON":           validDefinition + `{}`,
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			var catalog Catalog
			if err := catalog.ReloadJSON([]byte(input)); err == nil {
				t.Fatal("ReloadJSON() unexpectedly succeeded")
			}
		})
	}
}

func TestReloadJSONRejectsDuplicateDefinitionRefs(t *testing.T) {
	duplicate := `[` + validDefinition + `,` + validDefinition + `]`
	var catalog Catalog
	if err := catalog.ReloadJSON([]byte(duplicate)); err == nil {
		t.Fatal("ReloadJSON() accepted duplicate definition_ref")
	}
}

func TestFailedReloadLeavesPreviousSnapshotIntact(t *testing.T) {
	var catalog Catalog
	if err := catalog.ReloadJSON([]byte(`[` + validDefinition + `]`)); err != nil {
		t.Fatalf("initial ReloadJSON() error = %v", err)
	}
	if err := catalog.ReloadJSON([]byte(`[` + validDefinition + `,` + validDefinition + `]`)); err == nil {
		t.Fatal("duplicate reload unexpectedly succeeded")
	}
	if got := catalog.Len(); got != 1 {
		t.Fatalf("Len() after failed reload = %d, want 1", got)
	}
	if _, ok := catalog.Lookup("demo:npc-one"); !ok {
		t.Fatal("failed reload removed existing definition")
	}
	if _, ok := catalog.Lookup("demo:npc-two"); ok {
		t.Fatal("failed reload partially installed a definition")
	}

	if err := catalog.ReloadJSON([]byte(`[{"contract_version":"1.0.0"}]`)); err == nil {
		t.Fatal("invalid reload unexpectedly succeeded")
	}
	if got := catalog.Len(); got != 1 {
		t.Fatalf("Len() after invalid reload = %d, want 1", got)
	}
}

func TestReplaceRejectsDuplicateIDsAndInvalidDefinition(t *testing.T) {
	first := testDefinition("demo:duplicate", 1)
	second := testDefinition("demo:duplicate", 2)
	var catalog Catalog
	if err := catalog.Replace([]Definition{first, second}); err == nil {
		t.Fatal("Replace() accepted duplicate definition_ref")
	}
	if got := catalog.Len(); got != 0 {
		t.Fatalf("Len() after rejected initial replacement = %d, want 0", got)
	}

	first.Services = nil
	if err := catalog.Replace([]Definition{first}); err == nil {
		t.Fatal("Replace() accepted nil services")
	}
}

func TestCRUDDuplicationEditsActivationPublicationAndRollback(t *testing.T) {
	var catalog Catalog
	created := testDefinition("demo:catalog-a", 1)
	created.Services = []Service{ServiceShop, ServiceQuest, ServiceCrafting}
	if err := catalog.Create(created); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := catalog.Create(created); err == nil {
		t.Fatal("Create() accepted a duplicate definition_ref")
	}

	if err := catalog.Duplicate("demo:catalog-a", "demo:catalog-copy"); err != nil {
		t.Fatalf("Duplicate() error = %v", err)
	}
	duplicate, ok := catalog.Lookup("demo:catalog-copy")
	if !ok || duplicate.Revision != 1 || duplicate.Active ||
		len(duplicate.Services) != 3 || duplicate.Services[1] != ServiceQuest {
		t.Fatalf("Duplicate() did not preserve metadata: %#v, found=%v", duplicate, ok)
	}

	updated, _ := catalog.Lookup("demo:catalog-a")
	updated.Location.X = 42
	updated.Location.Y = -17
	updated.ModelRef = "demo:alternate-model"
	if err := catalog.Update("demo:catalog-a", updated); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if err := catalog.Activate("demo:catalog-a"); err != nil {
		t.Fatalf("Activate() error = %v", err)
	}
	if err := catalog.Activate("demo:catalog-a"); err != nil {
		t.Fatalf("idempotent Activate() error = %v", err)
	}
	if err := catalog.Publish("demo:catalog-a"); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	published, ok := catalog.Published("demo:catalog-a")
	if !ok || published.Revision != 3 || !published.Active ||
		published.Location.X != 42 || published.ModelRef != "demo:alternate-model" {
		t.Fatalf("Published() returned unexpected definition: %#v, found=%v", published, ok)
	}
	if err := catalog.Deactivate("demo:catalog-a"); err != nil {
		t.Fatalf("Deactivate() error = %v", err)
	}
	if err := catalog.Rollback("demo:catalog-a", 2); err != nil {
		t.Fatalf("Rollback() error = %v", err)
	}
	rolledBack, ok := catalog.Lookup("demo:catalog-a")
	if !ok || rolledBack.Revision != 5 || rolledBack.Active ||
		rolledBack.Location.X != 42 || rolledBack.ModelRef != "demo:alternate-model" {
		t.Fatalf("Rollback() did not create restored revision: %#v, found=%v", rolledBack, ok)
	}
	published, ok = catalog.Published("demo:catalog-a")
	if !ok || published.Revision != 5 {
		t.Fatalf("Rollback() did not publish restored revision: %#v, found=%v", published, ok)
	}
	history := catalog.History("demo:catalog-a")
	if len(history) != 5 {
		t.Fatalf("History() returned %d revisions, want 5", len(history))
	}
	for i, entry := range history {
		if entry.Definition.Revision != int64(i+1) {
			t.Fatalf("history[%d] revision = %d, want %d", i, entry.Definition.Revision, i+1)
		}
	}
	if !history[2].Published || !history[4].Published {
		t.Fatalf("published revision history markers missing: %#v", history)
	}
	history[0].Definition.Services[0] = ServiceArena
	originalHistory := catalog.History("demo:catalog-a")
	if originalHistory[0].Definition.Services[0] != ServiceShop {
		t.Fatal("History() exposed mutable catalog state")
	}
}

func TestRemovalRequiresReferenceCheckAndFailsClosed(t *testing.T) {
	var catalog Catalog
	if err := catalog.Create(testDefinition("demo:remove-me", 1)); err != nil {
		t.Fatal(err)
	}
	if err := catalog.Remove("demo:remove-me", nil); err == nil {
		t.Fatal("Remove() without a checker unexpectedly succeeded")
	}
	var nilChecker ReferenceCheckFunc
	if err := catalog.Remove("demo:remove-me", nilChecker); err == nil ||
		err.Error() != "reference checker function is nil" {
		t.Fatalf("Remove() with typed-nil checker error = %v, want explicit nil-function error", err)
	}
	if _, ok := catalog.Lookup("demo:remove-me"); !ok {
		t.Fatal("typed-nil checker removal changed the catalog")
	}
	checkFailed := errors.New("reference store unavailable")
	if err := catalog.Remove("demo:remove-me", ReferenceCheckFunc(func(string) (bool, error) {
		return false, checkFailed
	})); !errors.Is(err, checkFailed) {
		t.Fatalf("Remove() error = %v, want wrapped reference check error", err)
	}
	if err := catalog.Remove("demo:remove-me", ReferenceCheckFunc(func(string) (bool, error) {
		return true, nil
	})); err == nil {
		t.Fatal("Remove() deleted a referenced definition")
	}
	if _, ok := catalog.Lookup("demo:remove-me"); !ok {
		t.Fatal("failed removal changed the catalog")
	}
	if err := catalog.Publish("demo:remove-me"); err != nil {
		t.Fatal(err)
	}
	if err := catalog.Remove("demo:remove-me", ReferenceCheckFunc(func(string) (bool, error) {
		return false, nil
	})); err == nil {
		t.Fatal("Remove() deleted a published definition")
	}
	if err := catalog.Unpublish("demo:remove-me"); err != nil {
		t.Fatal(err)
	}
	if err := catalog.Remove("demo:remove-me", ReferenceCheckFunc(func(string) (bool, error) {
		return false, nil
	})); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	if _, ok := catalog.Lookup("demo:remove-me"); ok {
		t.Fatal("Remove() left the definition in the catalog")
	}
}

func TestInvalidJSONMutationIsAtomic(t *testing.T) {
	var catalog Catalog
	if err := catalog.CreateJSON([]byte(validDefinition)); err != nil {
		t.Fatalf("CreateJSON() error = %v", err)
	}
	before, _ := catalog.Lookup("demo:npc-one")
	invalid := strings.Replace(validDefinition, `"model_ref": "demo:model"`,
		`"model_ref": "demo:model", "unexpected": true`, 1)
	if err := catalog.UpdateJSON("demo:npc-one", []byte(invalid)); err == nil {
		t.Fatal("UpdateJSON() accepted unknown property")
	}
	if err := catalog.UpdateJSON("demo:npc-one", []byte(validDefinition+`{}`)); err == nil {
		t.Fatal("UpdateJSON() accepted trailing JSON")
	}
	after, _ := catalog.Lookup("demo:npc-one")
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("invalid JSON changed the definition: before=%#v after=%#v", before, after)
	}
	if got := len(catalog.History("demo:npc-one")); got != 1 {
		t.Fatalf("invalid JSON changed history length to %d", got)
	}
	if err := catalog.CreateJSON([]byte(`{"contract_version":"1.0.0"}`)); err == nil {
		t.Fatal("CreateJSON() accepted incomplete definition")
	}
	if got := catalog.Len(); got != 1 {
		t.Fatalf("failed CreateJSON() changed catalog length to %d", got)
	}
}

func TestConcurrentCatalogMutations(t *testing.T) {
	var catalog Catalog
	if err := catalog.Create(testDefinition("demo:concurrent", 1)); err != nil {
		t.Fatal(err)
	}
	const workers = 8
	const iterations = 50
	var group sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		group.Add(1)
		go func(worker int) {
			defer group.Done()
			for i := 0; i < iterations; i++ {
				ref := fmt.Sprintf("demo:concurrent-%d-%d", worker, i)
				if err := catalog.Duplicate("demo:concurrent", ref); err != nil {
					t.Errorf("Duplicate() error = %v", err)
					return
				}
				if err := catalog.Activate(ref); err != nil {
					t.Errorf("Activate() error = %v", err)
					return
				}
			}
		}(worker)
	}
	group.Wait()
	if got, want := catalog.Len(), 1+workers*iterations; got != want {
		t.Fatalf("Len() = %d, want %d", got, want)
	}
}

func TestConcurrentReadsAndReloads(t *testing.T) {
	var catalog Catalog
	if err := catalog.Replace(testGeneration(1)); err != nil {
		t.Fatal(err)
	}

	const readerCount = 8
	const reloadCount = 150
	var workers sync.WaitGroup
	for reader := 0; reader < readerCount; reader++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for i := 0; i < reloadCount*2; i++ {
				definitions := catalog.List()
				if len(definitions) != 2 {
					t.Errorf("List() returned %d definitions, want 2", len(definitions))
					return
				}
				if definitions[0].Revision != definitions[1].Revision {
					t.Errorf("List() mixed snapshots: revisions %d and %d",
						definitions[0].Revision, definitions[1].Revision)
					return
				}
				definition, ok := catalog.Lookup("demo:npc-a")
				if !ok || (definition.Revision != 1 && definition.Revision != 2) {
					t.Errorf("Lookup() returned incomplete snapshot: %#v, found=%v", definition, ok)
					return
				}
			}
		}()
	}
	for reload := 0; reload < 2; reload++ {
		workers.Add(1)
		go func(offset int) {
			defer workers.Done()
			for i := 0; i < reloadCount; i++ {
				revision := int64((i+offset)%2 + 1)
				if err := catalog.Replace(testGeneration(revision)); err != nil {
					t.Errorf("Replace() error = %v", err)
					return
				}
			}
		}(reload)
	}
	workers.Wait()
}

func testGeneration(revision int64) []Definition {
	return []Definition{
		testDefinition("demo:npc-a", revision),
		testDefinition("demo:npc-b", revision),
	}
}

func testDefinition(ref string, revision int64) Definition {
	return Definition{
		ContractVersion: ContractVersion,
		DefinitionRef:   ref,
		Revision:        revision,
		ModelRef:        "demo:model",
		Name:            fmt.Sprintf("NPC %s", ref),
		Location:        Location{MapRef: "demo:map"},
		Services:        []Service{ServiceQuest},
	}
}
