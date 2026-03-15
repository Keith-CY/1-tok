package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/chenyu/1-tok/internal/platform"
)

type RFQRepository struct {
	db *sql.DB
}

func NewRFQRepository(db *sql.DB) *RFQRepository {
	return &RFQRepository{db: db}
}

func (r *RFQRepository) NextID() (string, error) {
	return nextID(r.db, "rfq_seq", "rfq")
}

func (r *RFQRepository) Get(id string) (platform.RFQ, error) {
	row := r.db.QueryRowContext(context.TODO(), `
		SELECT id, buyer_org_id, title, category, scope, budget_cents, COALESCE(default_milestones, '[]'::jsonb), status, COALESCE(awarded_bid_id, ''), COALESCE(awarded_provider_org_id, ''), COALESCE(order_id, ''), response_deadline_at, created_at, updated_at
		FROM rfqs
		WHERE id = $1
	`, id)
	return scanRFQ(row)
}

func (r *RFQRepository) Save(rfq platform.RFQ) error {
	milestonesJSON, err := json.Marshal(rfq.DefaultMilestones)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(context.TODO(), `
		INSERT INTO rfqs (
			id, buyer_org_id, title, category, scope, budget_cents, default_milestones, status, awarded_bid_id, awarded_provider_org_id, order_id, response_deadline_at, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		ON CONFLICT (id) DO UPDATE SET
			buyer_org_id = EXCLUDED.buyer_org_id,
			title = EXCLUDED.title,
			category = EXCLUDED.category,
			scope = EXCLUDED.scope,
			budget_cents = EXCLUDED.budget_cents,
			default_milestones = EXCLUDED.default_milestones,
			status = EXCLUDED.status,
			awarded_bid_id = EXCLUDED.awarded_bid_id,
			awarded_provider_org_id = EXCLUDED.awarded_provider_org_id,
			order_id = EXCLUDED.order_id,
			response_deadline_at = EXCLUDED.response_deadline_at,
			updated_at = EXCLUDED.updated_at
	`, rfq.ID, rfq.BuyerOrgID, rfq.Title, rfq.Category, rfq.Scope, rfq.BudgetCents, milestonesJSON, string(rfq.Status), rfq.AwardedBidID, rfq.AwardedProviderOrgID, rfq.OrderID, rfq.ResponseDeadlineAt, rfq.CreatedAt, rfq.UpdatedAt)
	return err
}

func (r *RFQRepository) List() ([]platform.RFQ, error) {
	rows, err := r.db.QueryContext(context.TODO(), `
		SELECT id, buyer_org_id, title, category, scope, budget_cents, COALESCE(default_milestones, '[]'::jsonb), status, COALESCE(awarded_bid_id, ''), COALESCE(awarded_provider_org_id, ''), COALESCE(order_id, ''), response_deadline_at, created_at, updated_at
		FROM rfqs
		ORDER BY created_at ASC, id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	rfqs := make([]platform.RFQ, 0)
	for rows.Next() {
		rfq, err := scanRFQ(rows)
		if err != nil {
			return nil, err
		}
		rfqs = append(rfqs, rfq)
	}

	return rfqs, rows.Err()
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanRFQ(row rowScanner) (platform.RFQ, error) {
	var rfq platform.RFQ
	var status string
	var milestonesJSON []byte
	if err := row.Scan(
		&rfq.ID,
		&rfq.BuyerOrgID,
		&rfq.Title,
		&rfq.Category,
		&rfq.Scope,
		&rfq.BudgetCents,
		&milestonesJSON,
		&status,
		&rfq.AwardedBidID,
		&rfq.AwardedProviderOrgID,
		&rfq.OrderID,
		&rfq.ResponseDeadlineAt,
		&rfq.CreatedAt,
		&rfq.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return platform.RFQ{}, platform.ErrRFQNotFound
		}
		return platform.RFQ{}, err
	}
	rfq.Status = platform.RFQStatus(status)
	if len(milestonesJSON) > 0 {
		if err := json.Unmarshal(milestonesJSON, &rfq.DefaultMilestones); err != nil {
			return platform.RFQ{}, err
		}
	}
	return rfq, nil
}
