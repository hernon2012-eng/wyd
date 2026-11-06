package recipes

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"arko-gameplay-core/internal/core"
)

func testCatalog(t *testing.T) *Catalog {
	t.Helper()
	catalog, err := NewCatalog(filepath.Join("testdata", "catalog"))
	if err != nil {
		t.Fatal(err)
	}
	return catalog
}

func TestReloadIsolatesMalformedMissingVersionAndConflictingRecipes(t *testing.T) {
	catalog := testCatalog(t)
	diagnostics, err := catalog.Reload()
	if err != nil || len(diagnostics) < 5 {
		t.Fatalf("Reload() diagnostics=%v err=%v, want malformed, missing-version, output-mode, immutable-version, and duplicate reports", diagnostics, err)
	}
	duplicateDiagnostics := 0
	for _, diagnostic := range diagnostics {
		if strings.Contains(diagnostic.Error(), "duplicate recipe ID/version") && strings.Contains(diagnostic.Error(), "06-identical-duplicate.json") {
			duplicateDiagnostics++
		}
	}
	if duplicateDiagnostics != 1 {
		t.Fatalf("identical duplicate should produce one per-file diagnostic, got %d: %v", duplicateDiagnostics, diagnostics)
	}
	if catalog.HistorySize() != 2 {
		t.Fatalf("history size = %d, want valid versions 1 and 2 only", catalog.HistorySize())
	}
	version1, found, err := catalog.ReadRecipe(context.Background(), "demo:alloy", 1)
	if err != nil || !found || version1.Name != "DEMONSTRAÇÃO - alloy base" {
		t.Fatalf("version 1 = %#v found=%v err=%v", version1, found, err)
	}
	version2, found, err := catalog.ReadRecipe(context.Background(), "demo:alloy", 2)
	if err != nil || !found || version2.Product.ItemID != "demo:refined-alloy" || len(version2.TypedRequirements) != 1 {
		t.Fatalf("version 2 = %#v found=%v err=%v", version2, found, err)
	}
	if _, found, err := catalog.ReadRecipe(context.Background(), "demo:no-version", 1); err != nil || found {
		t.Fatalf("missing-version recipe found=%v err=%v", found, err)
	}
	if _, found, err := catalog.ReadRecipe(context.Background(), "demo:no-output-mode", 1); err != nil || found {
		t.Fatalf("missing-output-mode recipe found=%v err=%v", found, err)
	}
}

func TestCatalogReturnsImmutableRecipeCopies(t *testing.T) {
	catalog := testCatalog(t)
	recipe, found, err := catalog.ReadRecipe(context.Background(), "demo:alloy", 2)
	if err != nil || !found {
		t.Fatalf("ReadRecipe() found=%v err=%v", found, err)
	}
	originalPayload := append([]byte(nil), recipe.TypedRequirements[0].Payload...)
	recipe.TypedRequirements[0].Payload[0] = 'X'
	recipe.MaterialSelectors[0].Category = "changed"
	again, _, err := catalog.ReadRecipe(context.Background(), "demo:alloy", 2)
	if err != nil || string(again.TypedRequirements[0].Payload) != string(originalPayload) || again.MaterialSelectors[0].Category != "demo:ore-category" {
		t.Fatalf("catalog recipe was mutated: %#v, %v", again, err)
	}
}

func TestReloadFailurePreservesPublishedSnapshot(t *testing.T) {
	catalog := testCatalog(t)
	before, found, err := catalog.ReadRecipe(context.Background(), "demo:alloy", 1)
	if err != nil || !found {
		t.Fatalf("initial recipe found=%v err=%v", found, err)
	}
	catalog.root = filepath.Join("testdata", "missing-catalog-root")
	if _, err := catalog.Reload(); err == nil {
		t.Fatal("Reload() unexpectedly accepted missing root")
	}
	after, found, err := catalog.ReadRecipe(context.Background(), "demo:alloy", 1)
	if err != nil || !found || before.Name != after.Name || catalog.HistorySize() != 2 {
		t.Fatalf("failed reload changed snapshot: before=%#v after=%#v found=%v err=%v", before, after, found, err)
	}
}

func TestConcurrentCatalogReloadAndReads(t *testing.T) {
	catalog := testCatalog(t)
	var wait sync.WaitGroup
	failures := make(chan error, 16)
	for worker := 0; worker < 8; worker++ {
		wait.Add(1)
		go func(worker int) {
			defer wait.Done()
			for index := 0; index < 25; index++ {
				if worker%2 == 0 {
					if _, err := catalog.Reload(); err != nil {
						failures <- err
						return
					}
					continue
				}
				for _, version := range []uint64{1, 2} {
					recipe, found, err := catalog.ReadRecipe(context.Background(), "demo:alloy", version)
					if err != nil || !found || recipe.Version != version {
						failures <- errors.New("reader observed incomplete catalog snapshot")
						return
					}
				}
			}
		}(worker)
	}
	wait.Wait()
	close(failures)
	for err := range failures {
		t.Error(err)
	}
}

func TestRecipeDefinitionSchemaInputsAreCoreValidated(t *testing.T) {
	catalog := testCatalog(t)
	recipe, found, err := catalog.ReadRecipe(context.Background(), "demo:alloy", 2)
	if err != nil || !found || core.ValidateRecipeDefinitionV2(recipe) != nil {
		t.Fatalf("version 2 must be a valid core recipe: %#v, %v", recipe, err)
	}
}
