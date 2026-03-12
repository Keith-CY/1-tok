package postgres

import (
	"database/sql"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/chenyu/1-tok/internal/core"
	"github.com/chenyu/1-tok/internal/platform"
)

//go:embed schema.sql
var schema string

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
	_, err := db.Exec(schema)
	return err
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

type OrderRepository struct {
	db *sql.DB
}

func NewOrderRepository(db *sql.DB) *OrderRepository {
	return &OrderRepository{db: db}
}

func (r *OrderRepository) NextID() (string, error) {
	return nextID(r.db, "order_seq", "ord")
}

func (r *OrderRepository) Save(order *core.Order) error {
	payload, err := json.Marshal(order)
	if err != nil {
		return err
	}

	_, err = r.db.Exec(`
		INSERT INTO orders (
			id, buyer_org_id, provider_org_id, funding_mode, status, payload, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
		ON CONFLICT (id) DO UPDATE SET
			buyer_org_id = EXCLUDED.buyer_org_id,
			provider_org_id = EXCLUDED.provider_org_id,
			funding_mode = EXCLUDED.funding_mode,
			status = EXCLUDED.status,
			payload = EXCLUDED.payload,
			updated_at = NOW()
	`, order.ID, order.BuyerOrgID, order.ProviderOrgID, string(order.FundingMode), string(order.Status), payload)
	return err
}

func (r *OrderRepository) Get(id string) (*core.Order, error) {
	var payload []byte
	err := r.db.QueryRow(`SELECT payload FROM orders WHERE id = $1`, id).Scan(&payload)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errors.New("order not found")
	}
	if err != nil {
		return nil, err
	}

	return decodeOrder(payload)
}

func (r *OrderRepository) List() ([]*core.Order, error) {
	rows, err := r.db.Query(`SELECT payload FROM orders ORDER BY created_at ASC, id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	orders := make([]*core.Order, 0)
	for rows.Next() {
		var payload []byte
		if err := rows.Scan(&payload); err != nil {
			return nil, err
		}

		order, err := decodeOrder(payload)
		if err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}

	return orders, rows.Err()
}

type MessageRepository struct {
	db *sql.DB
}

func NewMessageRepository(db *sql.DB) *MessageRepository {
	return &MessageRepository{db: db}
}

func (r *MessageRepository) NextID() (string, error) {
	return nextID(r.db, "message_seq", "msg")
}

func (r *MessageRepository) Save(message platform.Message) error {
	_, err := r.db.Exec(`
		INSERT INTO messages (id, order_id, author, body, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`, message.ID, message.OrderID, message.Author, message.Body, message.CreatedAt)
	return err
}

type DisputeRepository struct {
	db *sql.DB
}

func NewDisputeRepository(db *sql.DB) *DisputeRepository {
	return &DisputeRepository{db: db}
}

func (r *DisputeRepository) NextID() (string, error) {
	return nextID(r.db, "dispute_seq", "disp")
}

func (r *DisputeRepository) Save(dispute platform.Dispute) error {
	_, err := r.db.Exec(`
		INSERT INTO disputes (id, order_id, milestone_id, reason, refund_cents, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, dispute.ID, dispute.OrderID, dispute.MilestoneID, dispute.Reason, dispute.RefundCents, dispute.CreatedAt)
	return err
}

func nextID(db *sql.DB, sequenceName, prefix string) (string, error) {
	return nextIDScanner(db, sequenceName, prefix)
}

type nextIDQueryer interface {
	QueryRow(query string, args ...any) *sql.Row
}

func nextIDScanner(db nextIDQueryer, sequenceName, prefix string) (string, error) {
	var value int64
	if err := db.QueryRow(fmt.Sprintf(`SELECT nextval('%s')`, sequenceName)).Scan(&value); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s_%d", prefix, value), nil
}

func decodeOrder(payload []byte) (*core.Order, error) {
	var order core.Order
	if err := json.Unmarshal(payload, &order); err != nil {
		return nil, err
	}
	return &order, nil
}

type ProviderRepository struct {
	db *sql.DB
}

func NewProviderRepository(db *sql.DB) *ProviderRepository {
	return &ProviderRepository{db: db}
}

func (r *ProviderRepository) List() ([]platform.ProviderProfile, error) {
	rows, err := r.db.Query(`
		SELECT id, name, capabilities, reputation_tier
		FROM providers
		ORDER BY id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	providers := make([]platform.ProviderProfile, 0)
	for rows.Next() {
		var provider platform.ProviderProfile
		var capabilities []byte
		if err := rows.Scan(&provider.ID, &provider.Name, &capabilities, &provider.ReputationTier); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(capabilities, &provider.Capabilities); err != nil {
			return nil, err
		}
		providers = append(providers, provider)
	}

	return providers, rows.Err()
}

func (r *ProviderRepository) Upsert(provider platform.ProviderProfile) error {
	capabilities, err := json.Marshal(provider.Capabilities)
	if err != nil {
		return err
	}

	_, err = r.db.Exec(`
		INSERT INTO providers (id, name, capabilities, reputation_tier, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW())
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			capabilities = EXCLUDED.capabilities,
			reputation_tier = EXCLUDED.reputation_tier,
			updated_at = NOW()
	`, provider.ID, provider.Name, capabilities, provider.ReputationTier)
	return err
}

type ListingRepository struct {
	db *sql.DB
}

func NewListingRepository(db *sql.DB) *ListingRepository {
	return &ListingRepository{db: db}
}

func (r *ListingRepository) List() ([]platform.Listing, error) {
	rows, err := r.db.Query(`
		SELECT id, provider_org_id, title, category, base_price_cents, tags
		FROM listings
		ORDER BY id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	listings := make([]platform.Listing, 0)
	for rows.Next() {
		var listing platform.Listing
		var tags []byte
		if err := rows.Scan(
			&listing.ID,
			&listing.ProviderOrgID,
			&listing.Title,
			&listing.Category,
			&listing.BasePriceCents,
			&tags,
		); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(tags, &listing.Tags); err != nil {
			return nil, err
		}
		listings = append(listings, listing)
	}

	return listings, rows.Err()
}

func (r *ListingRepository) Upsert(listing platform.Listing) error {
	tags, err := json.Marshal(listing.Tags)
	if err != nil {
		return err
	}

	_, err = r.db.Exec(`
		INSERT INTO listings (id, provider_org_id, title, category, base_price_cents, tags, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
		ON CONFLICT (id) DO UPDATE SET
			provider_org_id = EXCLUDED.provider_org_id,
			title = EXCLUDED.title,
			category = EXCLUDED.category,
			base_price_cents = EXCLUDED.base_price_cents,
			tags = EXCLUDED.tags,
			updated_at = NOW()
	`, listing.ID, listing.ProviderOrgID, listing.Title, listing.Category, listing.BasePriceCents, tags)
	return err
}

type RFQRepository struct {
	db *sql.DB
}

func NewRFQRepository(db *sql.DB) *RFQRepository {
	return &RFQRepository{db: db}
}

func (r *RFQRepository) NextID() (string, error) {
	return nextID(r.db, "rfq_seq", "rfq")
}

func (r *RFQRepository) Save(rfq platform.RFQ) error {
	_, err := r.db.Exec(`
		INSERT INTO rfqs (
			id, buyer_org_id, title, category, scope, budget_cents, status, response_deadline_at, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (id) DO UPDATE SET
			buyer_org_id = EXCLUDED.buyer_org_id,
			title = EXCLUDED.title,
			category = EXCLUDED.category,
			scope = EXCLUDED.scope,
			budget_cents = EXCLUDED.budget_cents,
			status = EXCLUDED.status,
			response_deadline_at = EXCLUDED.response_deadline_at,
			updated_at = EXCLUDED.updated_at
	`, rfq.ID, rfq.BuyerOrgID, rfq.Title, rfq.Category, rfq.Scope, rfq.BudgetCents, string(rfq.Status), rfq.ResponseDeadlineAt, rfq.CreatedAt, rfq.UpdatedAt)
	return err
}

func (r *RFQRepository) List() ([]platform.RFQ, error) {
	rows, err := r.db.Query(`
		SELECT id, buyer_org_id, title, category, scope, budget_cents, status, response_deadline_at, created_at, updated_at
		FROM rfqs
		ORDER BY created_at ASC, id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	rfqs := make([]platform.RFQ, 0)
	for rows.Next() {
		var rfq platform.RFQ
		var status string
		if err := rows.Scan(
			&rfq.ID,
			&rfq.BuyerOrgID,
			&rfq.Title,
			&rfq.Category,
			&rfq.Scope,
			&rfq.BudgetCents,
			&status,
			&rfq.ResponseDeadlineAt,
			&rfq.CreatedAt,
			&rfq.UpdatedAt,
		); err != nil {
			return nil, err
		}
		rfq.Status = platform.RFQStatus(status)
		rfqs = append(rfqs, rfq)
	}

	return rfqs, rows.Err()
}
