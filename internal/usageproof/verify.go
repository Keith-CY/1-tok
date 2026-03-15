// Package usageproof provides HMAC-based verification for usage charge reports.
// Carriers sign each usage report to prevent inflation attacks.
package usageproof

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	// ErrMissingProof is returned when a proof is required but not provided.
	ErrMissingProof = errors.New("usage proof is required")
	// ErrInvalidSignature is returned when the proof signature doesn't match.
	ErrInvalidSignature = errors.New("invalid usage proof signature")
	// ErrExpiredProof is returned when the proof timestamp is too old.
	ErrExpiredProof = errors.New("usage proof has expired")
)

// MaxProofAge is the maximum age of a usage proof before it's considered expired.
const MaxProofAge = 5 * time.Minute

// Proof represents a signed usage charge report from a Carrier.
type Proof struct {
	ExecutionID string `json:"executionId"`
	MilestoneID string `json:"milestoneId"`
	Kind        string `json:"kind"`
	AmountCents int64  `json:"amountCents"`
	Timestamp   string `json:"timestamp"`
	Signature   string `json:"signature"`
}

// Sign creates an HMAC-SHA256 signature for the proof payload.
func Sign(secret string, p Proof) string {
	payload := fmt.Sprintf("%s|%s|%s|%d|%s",
		p.ExecutionID, p.MilestoneID, p.Kind, p.AmountCents, p.Timestamp)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

// Verify checks the proof signature and timestamp.
// If secret is empty, verification is skipped (backward compatible).
func Verify(secret string, p Proof) error {
	if strings.TrimSpace(secret) == "" {
		return nil // No secret configured — skip verification
	}

	if strings.TrimSpace(p.Signature) == "" {
		return ErrMissingProof
	}

	expected := Sign(secret, p)
	if !hmac.Equal([]byte(expected), []byte(p.Signature)) {
		return ErrInvalidSignature
	}

	ts, err := time.Parse(time.RFC3339, p.Timestamp)
	if err != nil {
		return fmt.Errorf("invalid proof timestamp: %w", err)
	}
	if time.Since(ts) > MaxProofAge {
		return ErrExpiredProof
	}

	return nil
}
