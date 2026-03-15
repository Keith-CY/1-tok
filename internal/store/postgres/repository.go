package postgres

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/chenyu/1-tok/internal/platform"
)

//go:embed schema.sql
var schema string

const schemaMigrationLockKey int64 = 10241001

func Open(dsn string) (*sql.DB, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}

	return db, nil
}

func Migrate(db *sql.DB) error {
	conn, err := db.Conn(context.Background())
	if err != nil {
		return err
	}
	defer conn.Close()

	if _, err := conn.ExecContext(context.Background(), `SELECT pg_advisory_lock($1)`, schemaMigrationLockKey); err != nil {
		return err
	}
	defer func() {
		_, _ = conn.ExecContext(context.Background(), `SELECT pg_advisory_unlock($1)`, schemaMigrationLockKey)
	}()

	_, err = conn.ExecContext(context.Background(), schema)
	return err
}

func VerifyCoreSchema(db *sql.DB) error {
	return verifyRelations(db, []string{
		"order_seq",
		"rfq_seq",
		"bid_seq",
		"message_seq",
		"dispute_seq",
		"user_seq",
		"organization_seq",
		"iam_session_seq",
		"users",
		"organizations",
		"memberships",
		"iam_sessions",
		"providers",
		"listings",
		"orders",
		"rfqs",
		"bids",
		"messages",
		"disputes",
	})
}

func SeedCatalog(db *sql.DB) error {
	providers := NewProviderRepository(db)
	for _, provider := range platform.DefaultProviderProfiles() {
		if err := providers.Upsert(provider); err != nil {
			return err
		}
	}

	listings := NewListingRepository(db)
	for _, listing := range platform.DefaultListings() {
		if err := listings.Upsert(listing); err != nil {
			return err
		}
	}

	return nil
}

func verifyRelations(db *sql.DB, relations []string) error {
	for _, relation := range relations {
		var existing sql.NullString
		if err := db.QueryRow(`SELECT to_regclass($1)`, relation).Scan(&existing); err != nil {
			return err
		}
		if !existing.Valid {
			return fmt.Errorf("missing bootstrapped relation %q", relation)
		}
	}
	return nil
}
