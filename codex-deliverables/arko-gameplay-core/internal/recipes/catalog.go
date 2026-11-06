package recipes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"sync/atomic"

	"arko-gameplay-core/internal/core"
)

var categoryPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

type Definition struct {
	Category string      `json:"category"`
	Recipe   core.Recipe `json:"recipe"`
}

type recipeKey struct {
	id      core.RecipeID
	version uint64
}

type catalogEntry struct {
	category string
	recipe   core.Recipe
}

type snapshot struct {
	history map[recipeKey]catalogEntry
}

type Catalog struct {
	root          string
	current       atomic.Pointer[snapshot]
	reloadLock    sync.Mutex
	diagnosticsMu sync.RWMutex
	diagnostics   []error
}

func NewCatalog(root string) (*Catalog, error) {
	if root == "" {
		return nil, core.ErrInvalidRequest
	}
	catalog := &Catalog{root: root}
	catalog.current.Store(&snapshot{history: make(map[recipeKey]catalogEntry)})
	if _, err := catalog.Reload(); err != nil {
		return nil, err
	}
	return catalog, nil
}

// Reload publishes one complete immutable generation. Invalid individual files are
// returned as diagnostics and do not hide valid recipes or the previous history.
func (c *Catalog) Reload() ([]error, error) {
	if c == nil || c.root == "" {
		return nil, core.ErrInvalidRequest
	}
	c.reloadLock.Lock()
	defer c.reloadLock.Unlock()

	directories, err := os.ReadDir(c.root)
	if err != nil {
		return nil, err
	}
	current := c.current.Load()
	next := &snapshot{history: make(map[recipeKey]catalogEntry, len(current.history))}
	for key, entry := range current.history {
		next.history[key] = cloneEntry(entry)
	}
	var diagnostics []error
	seenCandidates := make(map[recipeKey]string)
	for _, directory := range directories {
		if !directory.IsDir() {
			continue
		}
		category := directory.Name()
		if !categoryPattern.MatchString(category) {
			diagnostics = append(diagnostics, fmt.Errorf("%s: invalid category directory", category))
			continue
		}
		files, readErr := os.ReadDir(filepath.Join(c.root, category))
		if readErr != nil {
			return nil, fmt.Errorf("%s: %w", category, readErr)
		}
		for _, file := range files {
			if file.IsDir() || filepath.Ext(file.Name()) != ".json" {
				continue
			}
			path := filepath.Join(c.root, category, file.Name())
			if file.Type()&os.ModeSymlink != 0 {
				diagnostics = append(diagnostics, fmt.Errorf("%s: symbolic links are not recipe files", path))
				continue
			}
			definition, decodeErr := readDefinition(path)
			if decodeErr != nil {
				diagnostics = append(diagnostics, fmt.Errorf("%s: %w", path, decodeErr))
				continue
			}
			if definition.Category != category {
				diagnostics = append(diagnostics, fmt.Errorf("%s: category does not match shard directory", path))
				continue
			}
			if err := core.ValidateRecipeDefinitionV2(definition.Recipe); err != nil {
				diagnostics = append(diagnostics, fmt.Errorf("%s: %w", path, err))
				continue
			}
			key := recipeKey{id: definition.Recipe.ID, version: definition.Recipe.Version}
			entry := catalogEntry{category: category, recipe: cloneRecipe(definition.Recipe)}
			if firstPath, duplicate := seenCandidates[key]; duplicate {
				diagnostics = append(diagnostics, fmt.Errorf("%s: duplicate recipe ID/version also declared in %s", path, firstPath))
				continue
			}
			seenCandidates[key] = path
			if previous, found := next.history[key]; found {
				if !sameEntry(previous, entry) {
					diagnostics = append(diagnostics, fmt.Errorf("%s: recipe ID/version is immutable", path))
				}
				continue
			}
			next.history[key] = entry
		}
	}
	c.current.Store(next)
	c.diagnosticsMu.Lock()
	c.diagnostics = append([]error(nil), diagnostics...)
	c.diagnosticsMu.Unlock()
	return diagnostics, nil
}

func (c *Catalog) Diagnostics() []error {
	if c == nil {
		return nil
	}
	c.diagnosticsMu.RLock()
	defer c.diagnosticsMu.RUnlock()
	return append([]error(nil), c.diagnostics...)
}

func (c *Catalog) ReadRecipe(ctx context.Context, id core.RecipeID, version uint64) (core.Recipe, bool, error) {
	if c == nil || ctx == nil {
		return core.Recipe{}, false, core.ErrInvalidRequest
	}
	if err := ctx.Err(); err != nil {
		return core.Recipe{}, false, err
	}
	entry, found := c.current.Load().history[recipeKey{id: id, version: version}]
	return cloneRecipe(entry.recipe), found, nil
}

func (c *Catalog) HistorySize() int {
	if c == nil {
		return 0
	}
	return len(c.current.Load().history)
}

func readDefinition(path string) (Definition, error) {
	file, err := os.Open(path)
	if err != nil {
		return Definition{}, err
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var definition Definition
	if err := decoder.Decode(&definition); err != nil {
		return Definition{}, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return Definition{}, errors.New("multiple JSON values")
		}
		return Definition{}, err
	}
	return definition, nil
}

func sameEntry(left, right catalogEntry) bool {
	if left.category != right.category {
		return false
	}
	leftJSON, leftErr := json.Marshal(left.recipe)
	rightJSON, rightErr := json.Marshal(right.recipe)
	return leftErr == nil && rightErr == nil && string(leftJSON) == string(rightJSON)
}

func cloneEntry(entry catalogEntry) catalogEntry {
	entry.recipe = cloneRecipe(entry.recipe)
	return entry
}

func cloneRecipe(recipe core.Recipe) core.Recipe {
	recipe.Materials = append([]core.ItemAmount(nil), recipe.Materials...)
	recipe.MaterialSelectors = append([]core.MaterialSelector(nil), recipe.MaterialSelectors...)
	if recipe.TransformSource != nil {
		source := *recipe.TransformSource
		recipe.TransformSource = &source
	}
	recipe.Requirements = append([]core.RecipeRequirement(nil), recipe.Requirements...)
	recipe.TypedRequirements = append([]core.TypedRequirement(nil), recipe.TypedRequirements...)
	for index := range recipe.TypedRequirements {
		recipe.TypedRequirements[index].Payload = append([]byte(nil), recipe.TypedRequirements[index].Payload...)
	}
	return recipe
}
