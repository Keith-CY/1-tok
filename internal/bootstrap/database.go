package bootstrap

import (
	"database/sql"
	"errors"

	"github.com/chenyu/1-tok/internal/services/settlement"
	postgresstore "github.com/chenyu/1-tok/internal/store/postgres"
)

type databaseBootstrapOptions struct {
	open           func(dsn string) (*sql.DB, error)
	migrateCore    func(db *sql.DB) error
	seedCatalog    func(db *sql.DB) error
	migrateFunding func(db *sql.DB) error
	closeDB        func(db *sql.DB) error
}

func BootstrapDatabase(dsn string) error {
	return runDatabaseBootstrap(dsn, databaseBootstrapOptions{
		open:           postgresstore.Open,
		migrateCore:    postgresstore.Migrate,
		seedCatalog:    postgresstore.SeedCatalog,
		migrateFunding: settlement.MigrateFundingRecordStore,
		closeDB:        closeDB,
	})
}

func runDatabaseBootstrap(dsn string, options databaseBootstrapOptions) (err error) {
	if dsn == "" {
		return errors.New("DATABASE_URL is required")
	}

	db, err := options.open(dsn)
	if err != nil {
		return err
	}
	defer func() {
		closeErr := options.closeDB(db)
		if err == nil {
			err = closeErr
		}
	}()

	if err := options.migrateCore(db); err != nil {
		return err
	}
	if err := options.seedCatalog(db); err != nil {
		return err
	}
	if err := options.migrateFunding(db); err != nil {
		return err
	}

	return nil
}
