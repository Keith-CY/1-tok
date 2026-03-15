package postgres

import (
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/chenyu/1-tok/internal/core"
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
		return nil, core.ErrOrderNotFound
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
