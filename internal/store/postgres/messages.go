package postgres

import (
	"database/sql"

	"github.com/chenyu/1-tok/internal/platform"
)

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
		INSERT INTO messages (id, order_id, rfq_id, author, body, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, message.ID, sql.NullString{String: message.OrderID, Valid: message.OrderID != ""}, sql.NullString{String: message.RFQID, Valid: message.RFQID != ""}, message.Author, message.Body, message.CreatedAt)
	return err
}

func (r *MessageRepository) ListByRFQ(rfqID string) ([]platform.Message, error) {
	rows, err := r.db.Query(`SELECT id, COALESCE(order_id, ''), COALESCE(rfq_id, ''), author, body, created_at FROM messages WHERE rfq_id = $1 ORDER BY created_at ASC`, rfqID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var messages []platform.Message
	for rows.Next() {
		var m platform.Message
		if err := rows.Scan(&m.ID, &m.OrderID, &m.RFQID, &m.Author, &m.Body, &m.CreatedAt); err != nil {
			return nil, err
		}
		messages = append(messages, m)
	}
	return messages, rows.Err()
}

func (r *MessageRepository) ListByOrder(orderID string) ([]platform.Message, error) {
	rows, err := r.db.Query(`SELECT id, COALESCE(order_id, ''), COALESCE(rfq_id, ''), author, body, created_at FROM messages WHERE order_id = $1 ORDER BY created_at ASC`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var messages []platform.Message
	for rows.Next() {
		var m platform.Message
		if err := rows.Scan(&m.ID, &m.OrderID, &m.RFQID, &m.Author, &m.Body, &m.CreatedAt); err != nil {
			return nil, err
		}
		messages = append(messages, m)
	}
	return messages, rows.Err()
}
