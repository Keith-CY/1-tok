package nats

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

type Publisher struct {
	conn *nats.Conn
	js   jetstream.JetStream
}

func Connect(url string) (*Publisher, error) {
	conn, err := nats.Connect(url,
		nats.MaxReconnects(-1),              // unlimited reconnect attempts
		nats.ReconnectWait(2*time.Second),   // wait 2s between attempts
		nats.ReconnectBufSize(16*1024*1024), // buffer up to 16MB while disconnected
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			if err != nil {
				log.Printf("nats: disconnected: %v", err)
			}
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			log.Printf("nats: reconnected to %s", nc.ConnectedUrl())
		}),
	)
	if err != nil {
		return nil, err
	}

	js, err := jetstream.New(conn)
	if err != nil {
		conn.Close()
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:      "MARKET_EVENTS",
		Subjects:  []string{"market.>"},
		Storage:   jetstream.FileStorage,
		Retention: jetstream.LimitsPolicy,
		MaxAge:    7 * 24 * time.Hour,
	})
	if err != nil {
		conn.Close()
		return nil, err
	}

	return &Publisher{conn: conn, js: js}, nil
}

func (p *Publisher) Publish(subject string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = p.js.Publish(ctx, subject, body)
	return err
}

func (p *Publisher) Close() {
	if p.conn != nil {
		p.conn.Close()
	}
}
