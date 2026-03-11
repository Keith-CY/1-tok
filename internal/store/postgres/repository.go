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
