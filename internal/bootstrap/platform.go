package bootstrap

import (
	"database/sql"
	"errors"
	"os"

	natsevents "github.com/chenyu/1-tok/internal/events/nats"
	"github.com/chenyu/1-tok/internal/platform"
	"github.com/chenyu/1-tok/internal/runtimeconfig"
	postgresstore "github.com/chenyu/1-tok/internal/store/postgres"
)

func LoadPlatformApp() (*platform.App, func() error, error) {
	publisher, publisherCleanup, err := loadPublisher()
	if err != nil {
		return nil, nil, err
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		if runtimeconfig.RequirePersistence() {
			_ = publisherCleanup()
			return nil, nil, errors.New("DATABASE_URL is required when ONE_TOK_REQUIRE_PERSISTENCE=true")
		}
		app := platform.NewAppWithMemory()
		app.SetPublisher(publisher)
		return app, publisherCleanup, nil
	}

	db, err := postgresstore.Open(dsn)
	if err != nil {
		_ = publisherCleanup()
		return nil, nil, err
	}

	if runtimeconfig.RequireBootstrappedDatabase() {
		if err := postgresstore.VerifyCoreSchema(db); err != nil {
			_ = db.Close()
			_ = publisherCleanup()
			return nil, nil, err
		}
	} else {
		if err := postgresstore.Migrate(db); err != nil {
			_ = db.Close()
			_ = publisherCleanup()
			return nil, nil, err
		}

		if err := postgresstore.SeedCatalog(db); err != nil {
			_ = db.Close()
			_ = publisherCleanup()
			return nil, nil, err
		}
	}

	app := platform.NewAppWithStorage(
		postgresstore.NewOrderRepository(db),
		postgresstore.NewProviderRepository(db),
		postgresstore.NewListingRepository(db),
		postgresstore.NewRFQRepository(db),
		postgresstore.NewBidRepository(db),
		postgresstore.NewMessageRepository(db),
		postgresstore.NewDisputeRepository(db),
	)
	app.SetPublisher(publisher)

	return app, func() error {
		if err := closeDB(db); err != nil {
			return err
		}
		return publisherCleanup()
	}, nil
}

func closeDB(db *sql.DB) error {
	return db.Close()
}

func loadPublisher() (platform.EventPublisher, func() error, error) {
	url := os.Getenv("NATS_URL")
	if url == "" {
		return nil, func() error { return nil }, nil
	}

	publisher, err := natsevents.Connect(url)
	if err != nil {
		return nil, nil, err
	}

	return publisher, func() error {
		publisher.Close()
		return nil
	}, nil
}
