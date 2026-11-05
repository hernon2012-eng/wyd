package main

import (
	"strings"
	"testing"
)

func TestOpenDatabaseRequiresDedicatedDatabaseName(t *testing.T) {
	for _, database := range []string{"wyd", "wyd_dev", "other"} {
		t.Run(database, func(t *testing.T) {
			t.Setenv("ARKO_ADMIN_DATABASE_NAME", requiredDatabaseName)
			t.Setenv("ARKO_ADMIN_DATABASE_URL", "postgres://demo:demo@127.0.0.1:5432/"+database+"?sslmode=disable")
			db, err := openDatabase()
			if err == nil || db != nil {
				if db != nil {
					db.Close()
				}
				t.Fatalf("openDatabase() = (%v, %v), want refusal without connecting", db, err)
			}
		})
	}
}

func TestRunRejectsUnknownCommandBeforeDatabaseAccess(t *testing.T) {
	err := run([]string{"unknown"})
	if err == nil || !strings.Contains(err.Error(), "usage") {
		t.Fatalf("run(unknown) error = %v, want usage error", err)
	}
}
