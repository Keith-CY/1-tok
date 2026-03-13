package settlement

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/chenyu/1-tok/internal/runtimeconfig"
	postgresstore "github.com/chenyu/1-tok/internal/store/postgres"
)

type FundingRecordKind string

const (
	FundingRecordKindInvoice    FundingRecordKind = "invoice"
	FundingRecordKindWithdrawal FundingRecordKind = "withdrawal"
)

type FundingRecord struct {
	ID            string            `json:"id"`
	Kind          FundingRecordKind `json:"kind"`
	OrderID       string            `json:"orderId,omitempty"`
	MilestoneID   string            `json:"milestoneId,omitempty"`
	BuyerOrgID    string            `json:"buyerOrgId,omitempty"`
	ProviderOrgID string            `json:"providerOrgId,omitempty"`
	Asset         string            `json:"asset"`
	Amount        string            `json:"amount"`
	Invoice       string            `json:"invoice,omitempty"`
	ExternalID    string            `json:"externalId,omitempty"`
	State         string            `json:"state"`
	Destination   map[string]string `json:"destination,omitempty"`
	CreatedAt     time.Time         `json:"createdAt"`
	UpdatedAt     time.Time         `json:"updatedAt"`
}

type FundingRecordFilter struct {
	Kind          FundingRecordKind
	OrderID       string
	ProviderOrgID string
}

type FundingRecordRepository interface {
	NextID() (string, error)
	Save(record FundingRecord) error
	UpdateInvoiceState(invoice, state string) error
	UpdateExternalState(externalID, state string) error
	List(filter FundingRecordFilter) ([]FundingRecord, error)
}

type memoryFundingRecordRepository struct {
	mu   sync.Mutex
	seq  int
	data map[string]FundingRecord
}

func NewMemoryFundingRecordRepository() FundingRecordRepository {
	return &memoryFundingRecordRepository{
		data: map[string]FundingRecord{},
	}
}

func (r *memoryFundingRecordRepository) NextID() (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.seq++
	return fmt.Sprintf("fund_%d", r.seq), nil
}

func (r *memoryFundingRecordRepository) Save(record FundingRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.data[record.ID] = record
	return nil
}

func (r *memoryFundingRecordRepository) UpdateInvoiceState(invoice, state string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, record := range r.data {
		if record.Invoice == invoice {
			record.State = state
			record.UpdatedAt = time.Now().UTC()
			r.data[id] = record
		}
	}
	return nil
}

func (r *memoryFundingRecordRepository) UpdateExternalState(externalID, state string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, record := range r.data {
		if record.ExternalID == externalID {
			record.State = state
			record.UpdatedAt = time.Now().UTC()
			r.data[id] = record
		}
	}
	return nil
}

func (r *memoryFundingRecordRepository) List(filter FundingRecordFilter) ([]FundingRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	records := make([]FundingRecord, 0, len(r.data))
	for _, record := range r.data {
		if filter.Kind != "" && record.Kind != filter.Kind {
			continue
		}
		if filter.OrderID != "" && record.OrderID != filter.OrderID {
			continue
		}
		if filter.ProviderOrgID != "" && record.ProviderOrgID != filter.ProviderOrgID {
			continue
		}
		records = append(records, record)
	}

	return records, nil
}

type postgresFundingRecordRepository struct {
	db *sql.DB
}

func newPostgresFundingRecordRepository(db *sql.DB) FundingRecordRepository {
	return &postgresFundingRecordRepository{db: db}
}

func (r *postgresFundingRecordRepository) NextID() (string, error) {
	var next int64
	if err := r.db.QueryRow(`SELECT nextval('settlement_funding_record_seq')`).Scan(&next); err != nil {
		return "", err
	}
	return fmt.Sprintf("fund_%d", next), nil
}

func (r *postgresFundingRecordRepository) Save(record FundingRecord) error {
	destination, err := json.Marshal(record.Destination)
	if err != nil {
		return err
	}

	_, err = r.db.Exec(`
		INSERT INTO settlement_funding_records (
			id, kind, order_id, milestone_id, buyer_org_id, provider_org_id, asset, amount,
			invoice, external_id, state, destination, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		ON CONFLICT (id) DO UPDATE SET
			kind = EXCLUDED.kind,
			order_id = EXCLUDED.order_id,
			milestone_id = EXCLUDED.milestone_id,
			buyer_org_id = EXCLUDED.buyer_org_id,
			provider_org_id = EXCLUDED.provider_org_id,
			asset = EXCLUDED.asset,
			amount = EXCLUDED.amount,
			invoice = EXCLUDED.invoice,
			external_id = EXCLUDED.external_id,
			state = EXCLUDED.state,
			destination = EXCLUDED.destination,
			updated_at = EXCLUDED.updated_at
	`, record.ID, string(record.Kind), nullIfEmpty(record.OrderID), nullIfEmpty(record.MilestoneID), nullIfEmpty(record.BuyerOrgID), nullIfEmpty(record.ProviderOrgID), record.Asset, record.Amount, nullIfEmpty(record.Invoice), nullIfEmpty(record.ExternalID), record.State, destination, record.CreatedAt, record.UpdatedAt)
	return err
}

func (r *postgresFundingRecordRepository) UpdateInvoiceState(invoice, state string) error {
	_, err := r.db.Exec(`
		UPDATE settlement_funding_records
		SET state = $2, updated_at = NOW()
		WHERE invoice = $1
	`, invoice, state)
	return err
}

func (r *postgresFundingRecordRepository) UpdateExternalState(externalID, state string) error {
	_, err := r.db.Exec(`
		UPDATE settlement_funding_records
		SET state = $2, updated_at = NOW()
		WHERE external_id = $1
	`, externalID, state)
	return err
}

func (r *postgresFundingRecordRepository) List(filter FundingRecordFilter) ([]FundingRecord, error) {
	rows, err := r.db.Query(`
		SELECT id, kind, COALESCE(order_id, ''), COALESCE(milestone_id, ''), COALESCE(buyer_org_id, ''),
		       COALESCE(provider_org_id, ''), asset, amount, COALESCE(invoice, ''), COALESCE(external_id, ''),
		       state, destination, created_at, updated_at
		FROM settlement_funding_records
		WHERE ($1 = '' OR kind = $1)
		  AND ($2 = '' OR order_id = $2)
		  AND ($3 = '' OR provider_org_id = $3)
		ORDER BY created_at ASC, id ASC
	`, string(filter.Kind), filter.OrderID, filter.ProviderOrgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records := make([]FundingRecord, 0)
	for rows.Next() {
		var record FundingRecord
		var kind string
		var destination []byte
		if err := rows.Scan(
			&record.ID,
			&kind,
			&record.OrderID,
			&record.MilestoneID,
			&record.BuyerOrgID,
			&record.ProviderOrgID,
			&record.Asset,
			&record.Amount,
			&record.Invoice,
			&record.ExternalID,
			&record.State,
			&destination,
			&record.CreatedAt,
			&record.UpdatedAt,
		); err != nil {
			return nil, err
		}
		record.Kind = FundingRecordKind(kind)
		if len(destination) > 0 {
			if err := json.Unmarshal(destination, &record.Destination); err != nil {
				return nil, err
			}
		}
		records = append(records, record)
	}

	return records, rows.Err()
}

// loadFundingRecordRepository returns a FundingRecordRepository.
// Deprecated: Use loadFundingRecordRepositoryE for explicit error handling.
func loadFundingRecordRepository() FundingRecordRepository {
	repo, err := loadFundingRecordRepositoryE()
	if err != nil {
		panic(fmt.Sprintf("settlement funding store: %v", err))
	}
	return repo
}

// loadFundingRecordRepositoryE returns a FundingRecordRepository or an error.
// When persistence is not required and the configured store fails to open,
// it falls back to an in-memory implementation.
func loadFundingRecordRepositoryE() (FundingRecordRepository, error) {
	repository, err := loadConfiguredFundingRecordRepository()
	if err != nil {
		if runtimeconfig.RequirePersistence() {
			return nil, err
		}
		log.Printf("settlement funding store: falling back to memory: %v", err)
		return NewMemoryFundingRecordRepository(), nil
	}

	return repository, nil
}

func loadConfiguredFundingRecordRepository() (FundingRecordRepository, error) {
	dsn := strings.TrimSpace(os.Getenv("SETTLEMENT_DATABASE_URL"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}
	if dsn == "" {
		return nil, errors.New("SETTLEMENT_DATABASE_URL or DATABASE_URL is required")
	}

	db, err := postgresstore.Open(dsn)
	if err != nil {
		return nil, fmt.Errorf("open settlement funding store: %w", err)
	}

	if runtimeconfig.RequireBootstrappedDatabase() {
		if err := VerifyFundingRecordStore(db); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("verify settlement funding store: %w", err)
		}
	} else {
		if err := MigrateFundingRecordStore(db); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("migrate settlement funding store: %w", err)
		}
	}

	return newPostgresFundingRecordRepository(db), nil
}

func MigrateFundingRecordStore(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE SEQUENCE IF NOT EXISTS settlement_funding_record_seq START 1;
		CREATE TABLE IF NOT EXISTS settlement_funding_records (
			id TEXT PRIMARY KEY,
			kind TEXT NOT NULL,
			order_id TEXT,
			milestone_id TEXT,
			buyer_org_id TEXT,
			provider_org_id TEXT,
			asset TEXT NOT NULL,
			amount TEXT NOT NULL,
			invoice TEXT,
			external_id TEXT,
			state TEXT NOT NULL,
			destination JSONB NOT NULL DEFAULT '{}'::jsonb,
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL
		);
		CREATE UNIQUE INDEX IF NOT EXISTS settlement_funding_records_invoice_uidx
			ON settlement_funding_records (invoice) WHERE invoice IS NOT NULL;
		CREATE UNIQUE INDEX IF NOT EXISTS settlement_funding_records_external_id_uidx
			ON settlement_funding_records (external_id) WHERE external_id IS NOT NULL;
	`)
	return err
}

func VerifyFundingRecordStore(db *sql.DB) error {
	for _, relation := range []string{"settlement_funding_record_seq", "settlement_funding_records"} {
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

func nullIfEmpty(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}
