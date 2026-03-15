package nats

import (
	"os"
	"testing"
)

func TestPublisher_Close_Nil(t *testing.T) {
	p := &Publisher{}
	p.Close()
}

func TestConnect_InvalidURL(t *testing.T) {
	_, err := Connect("nats://127.0.0.1:1")
	if err == nil {
		t.Error("expected error for unreachable NATS")
	}
}

func TestConnect_Success(t *testing.T) {
	url := os.Getenv("ONE_TOK_TEST_NATS_URL")
	if url == "" {
		t.Skip("ONE_TOK_TEST_NATS_URL not set")
	}

	pub, err := Connect(url)
	if err != nil {
		t.Fatal(err)
	}
	defer pub.Close()

	// Publish a test event
	if err := pub.Publish("market.test.event", map[string]any{"test": true}); err != nil {
		t.Fatal(err)
	}
}

func TestPublish_InvalidPayload(t *testing.T) {
	url := os.Getenv("ONE_TOK_TEST_NATS_URL")
	if url == "" {
		t.Skip("ONE_TOK_TEST_NATS_URL not set")
	}

	pub, err := Connect(url)
	if err != nil {
		t.Fatal(err)
	}
	defer pub.Close()

	// Publish with channel (not serializable)
	err = pub.Publish("market.test.bad", make(chan int))
	if err == nil {
		t.Error("expected error for non-serializable payload")
	}
}
