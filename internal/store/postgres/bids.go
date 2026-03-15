package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/chenyu/1-tok/internal/platform"
)

type BidRepository struct {
	db *sql.DB
}

func NewBidRepository(db *sql.DB) *BidRepository {
	return &BidRepository{db: db}
}

func (r *BidRepository) NextID() (string, error) {
	return nextID(r.db, "bid_seq", "bid")
}

func (r *BidRepository) Get(id string) (platform.Bid, error) {
	row := r.db.QueryRowContext(context.TODO(), `
		SELECT id, rfq_id, provider_org_id, message, quote_cents, status, milestones, created_at, updated_at
		FROM bids
		WHERE id = $1
	`, id)
	return scanBid(row)
}

func (r *BidRepository) Save(bid platform.Bid) error {
	milestones, err := json.Marshal(bid.Milestones)
	if err != nil {
		return err
	}

	_, err = r.db.ExecContext(context.TODO(), `
		INSERT INTO bids (
			id, rfq_id, provider_org_id, message, quote_cents, status, milestones, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (id) DO UPDATE SET
			rfq_id = EXCLUDED.rfq_id,
			provider_org_id = EXCLUDED.provider_org_id,
			message = EXCLUDED.message,
			quote_cents = EXCLUDED.quote_cents,
			status = EXCLUDED.status,
			milestones = EXCLUDED.milestones,
			updated_at = EXCLUDED.updated_at
	`, bid.ID, bid.RFQID, bid.ProviderOrgID, bid.Message, bid.QuoteCents, string(bid.Status), milestones, bid.CreatedAt, bid.UpdatedAt)
	return err
}

func (r *BidRepository) ListByRFQ(rfqID string) ([]platform.Bid, error) {
	rows, err := r.db.QueryContext(context.TODO(), `
		SELECT id, rfq_id, provider_org_id, message, quote_cents, status, milestones, created_at, updated_at
		FROM bids
		WHERE rfq_id = $1
		ORDER BY created_at ASC, id ASC
	`, rfqID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	bids := make([]platform.Bid, 0)
	for rows.Next() {
		bid, err := scanBid(rows)
		if err != nil {
			return nil, err
		}
		bids = append(bids, bid)
	}
	return bids, rows.Err()
}

func scanBid(row rowScanner) (platform.Bid, error) {
	var bid platform.Bid
	var status string
	var milestones []byte
	if err := row.Scan(
		&bid.ID,
		&bid.RFQID,
		&bid.ProviderOrgID,
		&bid.Message,
		&bid.QuoteCents,
		&status,
		&milestones,
		&bid.CreatedAt,
		&bid.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return platform.Bid{}, platform.ErrBidNotFound
		}
		return platform.Bid{}, err
	}
	bid.Status = platform.BidStatus(status)
	if err := json.Unmarshal(milestones, &bid.Milestones); err != nil {
		return platform.Bid{}, err
	}
	return bid, nil
}
