package postgres

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/chenyu/1-tok/internal/core"
	"github.com/chenyu/1-tok/internal/platform"
)

type DisputeRepository struct {
	db *sql.DB
}

func NewDisputeRepository(db *sql.DB) *DisputeRepository {
	return &DisputeRepository{db: db}
}

func (r *DisputeRepository) NextID() (string, error) {
	return nextID(r.db, "dispute_seq", "disp")
}

func (r *DisputeRepository) Get(id string) (platform.Dispute, error) {
	row := r.db.QueryRow(`
		SELECT id, order_id, milestone_id, reason, refund_cents, status, resolution, resolved_by, resolved_at, created_at
		FROM disputes
		WHERE id = $1
	`, id)
	return scanDispute(row)
}

func (r *DisputeRepository) Save(dispute platform.Dispute) error {
	_, err := r.db.Exec(`
		INSERT INTO disputes (
			id, order_id, milestone_id, reason, refund_cents, status, resolution, resolved_by, resolved_at, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (id) DO UPDATE SET
			order_id = EXCLUDED.order_id,
			milestone_id = EXCLUDED.milestone_id,
			reason = EXCLUDED.reason,
			refund_cents = EXCLUDED.refund_cents,
			status = EXCLUDED.status,
			resolution = EXCLUDED.resolution,
			resolved_by = EXCLUDED.resolved_by,
			resolved_at = EXCLUDED.resolved_at
	`, dispute.ID, dispute.OrderID, dispute.MilestoneID, dispute.Reason, dispute.RefundCents, normalizeDisputeStatus(dispute.Status), dispute.Resolution, dispute.ResolvedBy, dispute.ResolvedAt, dispute.CreatedAt)
	return err
}

func (r *DisputeRepository) List() ([]platform.Dispute, error) {
	rows, err := r.db.Query(`
		SELECT id, order_id, milestone_id, reason, refund_cents, status, resolution, resolved_by, resolved_at, created_at
		FROM disputes
		ORDER BY created_at ASC, id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	disputes := make([]platform.Dispute, 0)
	for rows.Next() {
		dispute, err := scanDispute(rows)
		if err != nil {
			return nil, err
		}
		disputes = append(disputes, dispute)
	}

	return disputes, rows.Err()
}

func scanDispute(row rowScanner) (platform.Dispute, error) {
	var dispute platform.Dispute
	var status string
	if err := row.Scan(
		&dispute.ID,
		&dispute.OrderID,
		&dispute.MilestoneID,
		&dispute.Reason,
		&dispute.RefundCents,
		&status,
		&dispute.Resolution,
		&dispute.ResolvedBy,
		&dispute.ResolvedAt,
		&dispute.CreatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return platform.Dispute{}, platform.ErrDisputeNotFound
		}
		return platform.Dispute{}, err
	}
	dispute.Status = core.DisputeStatus(status)
	if dispute.Status == "" {
		dispute.Status = core.DisputeStatusOpen
	}
	return dispute, nil
}

func normalizeDisputeStatus(status core.DisputeStatus) core.DisputeStatus {
	if status == "" {
		return core.DisputeStatusOpen
	}
	return status
}

func nextID(db *sql.DB, sequenceName, prefix string) (string, error) {
	return nextIDScanner(db, sequenceName, prefix)
}

type nextIDQueryer interface {
	QueryRow(query string, args ...any) *sql.Row
}

func nextIDScanner(db nextIDQueryer, sequenceName, prefix string) (string, error) {
	if !validSequenceName(sequenceName) {
		return "", fmt.Errorf("invalid sequence name: %q", sequenceName)
	}
	var value int64
	if err := db.QueryRow(fmt.Sprintf(`SELECT nextval('%s')`, sequenceName)).Scan(&value); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s_%d", prefix, value), nil
}

// validSequenceName checks that a sequence name contains only lowercase letters,
// digits, and underscores. This prevents SQL injection when the name is
// interpolated into a query via fmt.Sprintf.
func validSequenceName(name string) bool {
	if len(name) == 0 || len(name) > 63 {
		return false
	}
	for _, c := range name {
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_') {
			return false
		}
	}
	return true
}

func decodeOrder(payload []byte) (*core.Order, error) {
	var order core.Order
	if err := json.Unmarshal(payload, &order); err != nil {
		return nil, err
	}
	return &order, nil
}
