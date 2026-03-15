package usageproof

import (
	"testing"
	"time"
)

func TestSign_Deterministic(t *testing.T) {
	p := Proof{
		ExecutionID: "exec_1",
		MilestoneID: "ms_1",
		Kind:        "token",
		AmountCents: 500,
		Timestamp:   "2026-03-15T00:00:00Z",
	}
	sig1 := Sign("secret", p)
	sig2 := Sign("secret", p)
	if sig1 != sig2 {
		t.Error("expected deterministic signature")
	}
	if sig1 == "" {
		t.Error("expected non-empty signature")
	}
}

func TestSign_DifferentSecret(t *testing.T) {
	p := Proof{
		ExecutionID: "exec_1",
		MilestoneID: "ms_1",
		Kind:        "token",
		AmountCents: 500,
		Timestamp:   "2026-03-15T00:00:00Z",
	}
	sig1 := Sign("secret1", p)
	sig2 := Sign("secret2", p)
	if sig1 == sig2 {
		t.Error("different secrets should produce different signatures")
	}
}

func TestVerify_Success(t *testing.T) {
	p := Proof{
		ExecutionID: "exec_1",
		MilestoneID: "ms_1",
		Kind:        "token",
		AmountCents: 500,
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
	}
	p.Signature = Sign("secret", p)

	if err := Verify("secret", p); err != nil {
		t.Fatalf("expected valid proof, got %v", err)
	}
}

func TestVerify_NoSecret(t *testing.T) {
	// Empty secret = skip verification (backward compatible)
	p := Proof{Signature: ""}
	if err := Verify("", p); err != nil {
		t.Fatalf("expected nil for empty secret, got %v", err)
	}
}

func TestVerify_MissingSignature(t *testing.T) {
	p := Proof{Signature: ""}
	err := Verify("secret", p)
	if err != ErrMissingProof {
		t.Fatalf("expected ErrMissingProof, got %v", err)
	}
}

func TestVerify_InvalidSignature(t *testing.T) {
	p := Proof{
		ExecutionID: "exec_1",
		MilestoneID: "ms_1",
		Kind:        "token",
		AmountCents: 500,
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
		Signature:   "invalid_signature",
	}
	err := Verify("secret", p)
	if err != ErrInvalidSignature {
		t.Fatalf("expected ErrInvalidSignature, got %v", err)
	}
}

func TestVerify_ExpiredTimestamp(t *testing.T) {
	p := Proof{
		ExecutionID: "exec_1",
		MilestoneID: "ms_1",
		Kind:        "token",
		AmountCents: 500,
		Timestamp:   time.Now().Add(-10 * time.Minute).UTC().Format(time.RFC3339),
	}
	p.Signature = Sign("secret", p)

	err := Verify("secret", p)
	if err != ErrExpiredProof {
		t.Fatalf("expected ErrExpiredProof, got %v", err)
	}
}

func TestVerify_InvalidTimestamp(t *testing.T) {
	p := Proof{
		ExecutionID: "exec_1",
		MilestoneID: "ms_1",
		Kind:        "token",
		AmountCents: 500,
		Timestamp:   "not-a-timestamp",
		Signature:   Sign("secret", Proof{ExecutionID: "exec_1", MilestoneID: "ms_1", Kind: "token", AmountCents: 500, Timestamp: "not-a-timestamp"}),
	}

	err := Verify("secret", p)
	if err == nil {
		t.Error("expected error for invalid timestamp")
	}
}

func TestVerify_TamperedAmount(t *testing.T) {
	p := Proof{
		ExecutionID: "exec_1",
		MilestoneID: "ms_1",
		Kind:        "token",
		AmountCents: 500,
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
	}
	p.Signature = Sign("secret", p)

	// Tamper with amount
	p.AmountCents = 5000

	err := Verify("secret", p)
	if err != ErrInvalidSignature {
		t.Fatalf("expected ErrInvalidSignature for tampered amount, got %v", err)
	}
}
