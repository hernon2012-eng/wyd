package identity

import (
	"embed"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// Migration describes a schema migration available to the host application.
// The package exposes these files but never runs them.
type Migration struct {
	Version int
	Name    string
	SQL     string
}

// Migrations returns the versioned SQL migrations for explicit application by
// the embedding application.
func Migrations() ([]Migration, error) {
	entries, err := migrationFiles.ReadDir("migrations")
	if err != nil {
		return nil, fmt.Errorf("read identity migrations: %w", err)
	}

	migrations := make([]Migration, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		versionText, name, ok := strings.Cut(entry.Name(), "_")
		if !ok {
			return nil, fmt.Errorf("invalid identity migration filename %q", entry.Name())
		}
		version, err := strconv.Atoi(versionText)
		if err != nil || version <= 0 || name == "" {
			return nil, fmt.Errorf("invalid identity migration filename %q", entry.Name())
		}
		content, err := migrationFiles.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return nil, fmt.Errorf("read identity migration %q: %w", entry.Name(), err)
		}
		migrations = append(migrations, Migration{
			Version: version,
			Name:    strings.TrimSuffix(name, ".sql"),
			SQL:     string(content),
		})
	}
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})
	return migrations, nil
}
