package postgres

import "testing"

func TestValidSequenceName(t *testing.T) {
	valid := []string{
		"order_seq",
		"rfq_seq",
		"bid_seq",
		"message_seq",
		"dispute_seq",
		"user_seq",
		"organization_seq",
		"iam_session_seq",
		"settlement_funding_record_seq",
	}
	for _, name := range valid {
		if !validSequenceName(name) {
			t.Errorf("expected %q to be valid", name)
		}
	}

	invalid := []string{
		"",
		"ORDER_SEQ",
		"order seq",
		"order-seq",
		"'; DROP TABLE users; --",
		"order_seq\x00",
		"order.seq",
	}
	for _, name := range invalid {
		if validSequenceName(name) {
			t.Errorf("expected %q to be invalid", name)
		}
	}
}
