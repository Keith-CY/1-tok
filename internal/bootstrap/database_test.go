package bootstrap

import (
	"database/sql"
	"reflect"
	"testing"
)

func TestRunDatabaseBootstrapExecutesCoreAndFundingSteps(t *testing.T) {
	db := &sql.DB{}
	steps := make([]string, 0, 4)

	err := runDatabaseBootstrap("postgres://example", databaseBootstrapOptions{
		open: func(dsn string) (*sql.DB, error) {
			steps = append(steps, "open:"+dsn)
			return db, nil
		},
		migrateCore: func(candidate *sql.DB) error {
			if candidate != db {
				t.Fatalf("expected core migrate to receive opened db")
			}
			steps = append(steps, "migrate-core")
			return nil
		},
		seedCatalog: func(candidate *sql.DB) error {
			if candidate != db {
				t.Fatalf("expected seed to receive opened db")
			}
			steps = append(steps, "seed-catalog")
			return nil
		},
		migrateFunding: func(candidate *sql.DB) error {
			if candidate != db {
				t.Fatalf("expected funding migrate to receive opened db")
			}
			steps = append(steps, "migrate-funding")
			return nil
		},
		closeDB: func(candidate *sql.DB) error {
			if candidate != db {
				t.Fatalf("expected close to receive opened db")
			}
			steps = append(steps, "close")
			return nil
		},
	})
	if err != nil {
		t.Fatalf("bootstrap database: %v", err)
	}

	expected := []string{
		"open:postgres://example",
		"migrate-core",
		"seed-catalog",
		"migrate-funding",
		"close",
	}
	if !reflect.DeepEqual(steps, expected) {
		t.Fatalf("unexpected bootstrap steps: got %v want %v", steps, expected)
	}
}

func TestBootstrapDatabaseRejectsMissingDSN(t *testing.T) {
	if err := BootstrapDatabase(""); err == nil {
		t.Fatalf("expected missing DATABASE_URL to fail")
	}
}
