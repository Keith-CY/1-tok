package bootstrap

import (
	"database/sql"
	"os"

	"github.com/chenyu/1-tok/internal/platform"
	postgresstore "github.com/chenyu/1-tok/internal/store/postgres"
)

func LoadPlatformApp() (*platform.App, func() error, error) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return platform.NewAppWithMemory(), func() error { return nil }, nil
	}

	db, err := postgresstore.Open(dsn)
	if err != nil {
		return nil, nil, err
	}

	if err := postgresstore.Migrate(db); err != nil {
		_ = db.Close()
		return nil, nil, err
	}

	app := platform.NewAppWithStorage(
		postgresstore.NewOrderRepository(db),
		postgresstore.NewMessageRepository(db),
		postgresstore.NewDisputeRepository(db),
	)

	return app, func() error { return closeDB(db) }, nil
}

func closeDB(db *sql.DB) error {
	return db.Close()
}
