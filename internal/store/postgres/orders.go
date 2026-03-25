package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/chenyu/1-tok/internal/core"
	"github.com/chenyu/1-tok/internal/platform"
)

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

	_, err = r.db.ExecContext(context.TODO(), `
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
	err := r.db.QueryRowContext(context.TODO(), `SELECT payload FROM orders WHERE id = $1`, id).Scan(&payload)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, core.ErrOrderNotFound
	}
	if err != nil {
		return nil, err
	}

	return decodeOrder(payload)
}

func (r *OrderRepository) List() ([]*core.Order, error) {
	rows, err := r.db.QueryContext(context.TODO(), `SELECT payload FROM orders ORDER BY created_at ASC, id ASC`)
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

func (r *OrderRepository) ListByFilter(filter platform.OrderListFilter) ([]*core.Order, error) {
	conditions := make([]string, 0, 3)
	args := make([]any, 0, 1+len(filter.Statuses))
	nextArg := 1

	if strings.TrimSpace(filter.BuyerOrgID) != "" {
		conditions = append(conditions, fmt.Sprintf("buyer_org_id = $%d", nextArg))
		args = append(args, filter.BuyerOrgID)
		nextArg++
	}
	if strings.TrimSpace(string(filter.FundingMode)) != "" {
		conditions = append(conditions, fmt.Sprintf("funding_mode = $%d", nextArg))
		args = append(args, filter.FundingMode)
		nextArg++
	}
	if len(filter.Statuses) > 0 {
		placeholders := make([]string, 0, len(filter.Statuses))
		for _, status := range filter.Statuses {
			if status == "" {
				continue
			}
			placeholders = append(placeholders, fmt.Sprintf("$%d", nextArg))
			args = append(args, string(status))
			nextArg++
		}
		if len(placeholders) > 0 {
			conditions = append(conditions, fmt.Sprintf("status IN (%s)", strings.Join(placeholders, ",")))
		}
	}

	query := "SELECT payload FROM orders"
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY created_at ASC, id ASC"

	rows, err := r.db.QueryContext(context.TODO(), query, args...)
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
