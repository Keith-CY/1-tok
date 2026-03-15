package carrier

import (
	"testing"
	"time"
)

func TestSignCallback_Deterministic(t *testing.T) {
	event := CallbackEvent{
		Type: "job.completed", JobID: "job_1", BindingID: "bind_1",
		Timestamp: "2026-03-15T00:00:00Z",
	}
	sig1 := SignCallback("secret", event)
	sig2 := SignCallback("secret", event)
	if sig1 != sig2 {
		t.Error("expected deterministic signature")
	}
}

func TestVerifyCallback_Success(t *testing.T) {
	event := CallbackEvent{
		Type: "job.completed", JobID: "job_1", BindingID: "bind_1",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
	event.Signature = SignCallback("secret", event)
	if err := VerifyCallback("secret", event); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyCallback_NoSecret(t *testing.T) {
	if err := VerifyCallback("", CallbackEvent{}); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyCallback_InvalidSignature(t *testing.T) {
	event := CallbackEvent{
		Type: "job.completed", JobID: "job_1", BindingID: "bind_1",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Signature: "invalid",
	}
	if err := VerifyCallback("secret", event); err != ErrInvalidCallbackSignature {
		t.Fatalf("expected ErrInvalidCallbackSignature, got %v", err)
	}
}

func TestVerifyCallback_Expired(t *testing.T) {
	event := CallbackEvent{
		Type: "job.completed", JobID: "job_1", BindingID: "bind_1",
		Timestamp: time.Now().Add(-10 * time.Minute).UTC().Format(time.RFC3339),
	}
	event.Signature = SignCallback("secret", event)
	if err := VerifyCallback("secret", event); err != ErrExpiredCallback {
		t.Fatalf("expected ErrExpiredCallback, got %v", err)
	}
}

func TestVerifyCallback_BadTimestamp(t *testing.T) {
	event := CallbackEvent{
		Type: "job.completed", JobID: "job_1", BindingID: "bind_1",
		Timestamp: "not-a-timestamp",
	}
	event.Signature = SignCallback("secret", event)
	if err := VerifyCallback("secret", event); err == nil {
		t.Error("expected error for bad timestamp")
	}
}

func TestSignCallback_DifferentSecrets(t *testing.T) {
	event := CallbackEvent{Type: "job.completed", JobID: "job_1", BindingID: "bind_1", Timestamp: "2026-03-15T00:00:00Z"}
	sig1 := SignCallback("secret1", event)
	sig2 := SignCallback("secret2", event)
	if sig1 == sig2 {
		t.Error("different secrets should produce different signatures")
	}
}
