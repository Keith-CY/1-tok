package bootstrap

import (
	"database/sql"
	"errors"
	"os"
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

func TestBootstrapDatabase_WithPostgres(t *testing.T) {
	dsn := os.Getenv("ONE_TOK_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ONE_TOK_TEST_DATABASE_URL not set")
	}
	if err := BootstrapDatabase(dsn); err != nil {
		t.Fatal(err)
	}
}

func TestBootstrapDatabase_NoDSN(t *testing.T) {
	err := BootstrapDatabase("")
	if err == nil {
		t.Error("expected error without DSN")
	}
}

func TestRunDatabaseBootstrap_AllSteps(t *testing.T) {
	migrateCalled := false
	seedCalled := false
	fundingCalled := false

	err := runDatabaseBootstrap("postgres://dummy", databaseBootstrapOptions{
		open: func(dsn string) (*sql.DB, error) {
			// Return a fake DB — we don't actually connect
			return nil, nil
		},
		migrateCore: func(db *sql.DB) error {
			migrateCalled = true
			return nil
		},
		seedCatalog: func(db *sql.DB) error {
			seedCalled = true
			return nil
		},
		migrateFunding: func(db *sql.DB) error {
			fundingCalled = true
			return nil
		},
		closeDB: func(db *sql.DB) error {
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !migrateCalled || !seedCalled || !fundingCalled {
		t.Errorf("migrate=%v seed=%v funding=%v", migrateCalled, seedCalled, fundingCalled)
	}
}

func TestRunDatabaseBootstrap_OpenError(t *testing.T) {
	err := runDatabaseBootstrap("postgres://dummy", databaseBootstrapOptions{
		open: func(dsn string) (*sql.DB, error) {
			return nil, errors.New("connect failed")
		},
		closeDB: func(db *sql.DB) error { return nil },
	})
	if err == nil {
		t.Error("expected error from open failure")
	}
}

func TestRunDatabaseBootstrap_MigrateError(t *testing.T) {
	err := runDatabaseBootstrap("postgres://dummy", databaseBootstrapOptions{
		open: func(dsn string) (*sql.DB, error) { return nil, nil },
		migrateCore: func(db *sql.DB) error { return errors.New("migrate failed") },
		closeDB:     func(db *sql.DB) error { return nil },
	})
	if err == nil {
		t.Error("expected error from migrate failure")
	}
}

func TestRunDatabaseBootstrap_SeedError(t *testing.T) {
	err := runDatabaseBootstrap("postgres://dummy", databaseBootstrapOptions{
		open:        func(dsn string) (*sql.DB, error) { return nil, nil },
		migrateCore: func(db *sql.DB) error { return nil },
		seedCatalog: func(db *sql.DB) error { return errors.New("seed failed") },
		closeDB:     func(db *sql.DB) error { return nil },
	})
	if err == nil {
		t.Error("expected error from seed failure")
	}
}

func TestRunDatabaseBootstrap_FundingError(t *testing.T) {
	err := runDatabaseBootstrap("postgres://dummy", databaseBootstrapOptions{
		open:           func(dsn string) (*sql.DB, error) { return nil, nil },
		migrateCore:    func(db *sql.DB) error { return nil },
		seedCatalog:    func(db *sql.DB) error { return nil },
		migrateFunding: func(db *sql.DB) error { return errors.New("funding failed") },
		closeDB:        func(db *sql.DB) error { return nil },
	})
	if err == nil {
		t.Error("expected error from funding failure")
	}
}
