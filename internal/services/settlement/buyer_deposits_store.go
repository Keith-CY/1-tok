package settlement

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/chenyu/1-tok/internal/runtimeconfig"
	postgresstore "github.com/chenyu/1-tok/internal/store/postgres"
)

type postgresBuyerDepositAddressRepository struct {
	db *sql.DB
}

type postgresBuyerDepositSweepRepository struct {
	db *sql.DB
}

func newPostgresBuyerDepositAddressRepository(db *sql.DB) BuyerDepositAddressRepository {
	return &postgresBuyerDepositAddressRepository{db: db}
}

func newPostgresBuyerDepositSweepRepository(db *sql.DB) BuyerDepositSweepRepository {
	return &postgresBuyerDepositSweepRepository{db: db}
}

func (r *postgresBuyerDepositAddressRepository) NextID() (string, error) {
	var next int64
	if err := r.db.QueryRow(`SELECT nextval('settlement_buyer_deposit_address_seq')`).Scan(&next); err != nil {
		return "", err
	}
	return fmt.Sprintf("depaddr_%d", next), nil
}

func (r *postgresBuyerDepositAddressRepository) NextDerivationIndex() (int, error) {
	var next int64
	if err := r.db.QueryRow(`SELECT nextval('settlement_buyer_deposit_derivation_seq') - 1`).Scan(&next); err != nil {
		return 0, err
	}
	return int(next), nil
}

func (r *postgresBuyerDepositAddressRepository) Save(record BuyerDepositAddress) error {
	_, err := r.db.Exec(`
		INSERT INTO settlement_buyer_deposit_addresses (
			id, buyer_org_id, asset, address, derivation_index, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (buyer_org_id) DO UPDATE SET
			asset = EXCLUDED.asset,
			address = EXCLUDED.address,
			derivation_index = EXCLUDED.derivation_index,
			updated_at = EXCLUDED.updated_at
	`, record.ID, record.BuyerOrgID, record.Asset, record.Address, record.DerivationIndex, record.CreatedAt, record.UpdatedAt)
	return err
}

func (r *postgresBuyerDepositAddressRepository) GetByBuyerOrgID(buyerOrgID string) (BuyerDepositAddress, error) {
	var record BuyerDepositAddress
	err := r.db.QueryRow(`
		SELECT id, buyer_org_id, asset, address, derivation_index, created_at, updated_at
		FROM settlement_buyer_deposit_addresses
		WHERE buyer_org_id = $1
	`, buyerOrgID).Scan(
		&record.ID,
		&record.BuyerOrgID,
		&record.Asset,
		&record.Address,
		&record.DerivationIndex,
		&record.CreatedAt,
		&record.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return BuyerDepositAddress{}, errors.New("buyer deposit address not found")
		}
		return BuyerDepositAddress{}, err
	}
	return record, nil
}

func (r *postgresBuyerDepositAddressRepository) List() ([]BuyerDepositAddress, error) {
	rows, err := r.db.Query(`
		SELECT id, buyer_org_id, asset, address, derivation_index, created_at, updated_at
		FROM settlement_buyer_deposit_addresses
		ORDER BY created_at ASC, id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records := make([]BuyerDepositAddress, 0)
	for rows.Next() {
		var record BuyerDepositAddress
		if err := rows.Scan(
			&record.ID,
			&record.BuyerOrgID,
			&record.Asset,
			&record.Address,
			&record.DerivationIndex,
			&record.CreatedAt,
			&record.UpdatedAt,
		); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

func (r *postgresBuyerDepositSweepRepository) NextID() (string, error) {
	var next int64
	if err := r.db.QueryRow(`SELECT nextval('settlement_buyer_deposit_sweep_seq')`).Scan(&next); err != nil {
		return "", err
	}
	return fmt.Sprintf("depsweep_%d", next), nil
}

func (r *postgresBuyerDepositSweepRepository) Save(record BuyerDepositSweepRecord) error {
	_, err := r.db.Exec(`
		INSERT INTO settlement_buyer_deposit_sweeps (
			id, buyer_org_id, asset, deposit_address, treasury_address, amount_raw, tx_hash, state, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (id) DO UPDATE SET
			asset = EXCLUDED.asset,
			deposit_address = EXCLUDED.deposit_address,
			treasury_address = EXCLUDED.treasury_address,
			amount_raw = EXCLUDED.amount_raw,
			tx_hash = EXCLUDED.tx_hash,
			state = EXCLUDED.state,
			updated_at = EXCLUDED.updated_at
	`, record.ID, record.BuyerOrgID, record.Asset, record.DepositAddress, record.TreasuryAddress, record.AmountRaw, nullIfEmpty(record.TxHash), record.State, record.CreatedAt, record.UpdatedAt)
	return err
}

func (r *postgresBuyerDepositSweepRepository) List(filter BuyerDepositSweepFilter) ([]BuyerDepositSweepRecord, error) {
	rows, err := r.db.Query(`
		SELECT id, buyer_org_id, asset, deposit_address, treasury_address, amount_raw, COALESCE(tx_hash, ''), state, created_at, updated_at
		FROM settlement_buyer_deposit_sweeps
		WHERE ($1 = '' OR buyer_org_id = $1)
		ORDER BY created_at ASC, id ASC
	`, filter.BuyerOrgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records := make([]BuyerDepositSweepRecord, 0)
	for rows.Next() {
		var record BuyerDepositSweepRecord
		if err := rows.Scan(
			&record.ID,
			&record.BuyerOrgID,
			&record.Asset,
			&record.DepositAddress,
			&record.TreasuryAddress,
			&record.AmountRaw,
			&record.TxHash,
			&record.State,
			&record.CreatedAt,
			&record.UpdatedAt,
		); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

func loadBuyerDepositRepositoriesE() (BuyerDepositAddressRepository, BuyerDepositSweepRepository, error) {
	dsn := strings.TrimSpace(os.Getenv("SETTLEMENT_DATABASE_URL"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}
	if dsn == "" {
		return nil, nil, errors.New("SETTLEMENT_DATABASE_URL or DATABASE_URL is required")
	}

	db, err := postgresstore.Open(dsn)
	if err != nil {
		return nil, nil, fmt.Errorf("open buyer deposit store: %w", err)
	}

	if runtimeconfig.RequireBootstrappedDatabase() {
		if err := VerifyBuyerDepositStore(db); err != nil {
			_ = db.Close()
			return nil, nil, fmt.Errorf("verify buyer deposit store: %w", err)
		}
	} else {
		if err := MigrateBuyerDepositStore(db); err != nil {
			_ = db.Close()
			return nil, nil, fmt.Errorf("migrate buyer deposit store: %w", err)
		}
	}

	return newPostgresBuyerDepositAddressRepository(db), newPostgresBuyerDepositSweepRepository(db), nil
}

func loadBuyerDepositRepositoriesOrMemory() (BuyerDepositAddressRepository, BuyerDepositSweepRepository, error) {
	addresses, sweeps, err := loadBuyerDepositRepositoriesE()
	if err != nil {
		if runtimeconfig.RequirePersistence() {
			return nil, nil, err
		}
		log.Printf("settlement buyer deposit store: falling back to memory: %v", err)
		return NewMemoryBuyerDepositAddressRepository(), NewMemoryBuyerDepositSweepRepository(), nil
	}
	return addresses, sweeps, nil
}

func MigrateBuyerDepositStore(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE SEQUENCE IF NOT EXISTS settlement_buyer_deposit_address_seq START 1;
		CREATE SEQUENCE IF NOT EXISTS settlement_buyer_deposit_derivation_seq START 1;
		CREATE SEQUENCE IF NOT EXISTS settlement_buyer_deposit_sweep_seq START 1;

		CREATE TABLE IF NOT EXISTS settlement_buyer_deposit_addresses (
			id TEXT PRIMARY KEY,
			buyer_org_id TEXT NOT NULL UNIQUE,
			asset TEXT NOT NULL,
			address TEXT NOT NULL UNIQUE,
			derivation_index INTEGER NOT NULL UNIQUE,
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL
		);

		CREATE TABLE IF NOT EXISTS settlement_buyer_deposit_sweeps (
			id TEXT PRIMARY KEY,
			buyer_org_id TEXT NOT NULL,
			asset TEXT NOT NULL,
			deposit_address TEXT NOT NULL,
			treasury_address TEXT NOT NULL,
			amount_raw BIGINT NOT NULL,
			tx_hash TEXT,
			state TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL
		);

		CREATE UNIQUE INDEX IF NOT EXISTS settlement_buyer_deposit_sweeps_tx_hash_uidx
			ON settlement_buyer_deposit_sweeps (tx_hash) WHERE tx_hash IS NOT NULL;
		CREATE INDEX IF NOT EXISTS idx_sbd_addresses_buyer_org_id ON settlement_buyer_deposit_addresses (buyer_org_id);
		CREATE INDEX IF NOT EXISTS idx_sbd_sweeps_buyer_org_id ON settlement_buyer_deposit_sweeps (buyer_org_id);
	`)
	return err
}

func VerifyBuyerDepositStore(db *sql.DB) error {
	for _, relation := range []string{
		"settlement_buyer_deposit_address_seq",
		"settlement_buyer_deposit_derivation_seq",
		"settlement_buyer_deposit_sweep_seq",
		"settlement_buyer_deposit_addresses",
		"settlement_buyer_deposit_sweeps",
	} {
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
