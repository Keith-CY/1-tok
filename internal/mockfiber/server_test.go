package mockfiber

import (
	"context"
	"net/http/httptest"
	"testing"

	fiberclient "github.com/chenyu/1-tok/internal/integrations/fiber"
)

func TestServerCreatesInvoiceAndReturnsSettledFeedItem(t *testing.T) {
	server := httptest.NewServer(NewServer())
	defer server.Close()

	client := fiberclient.NewClient(server.URL, "app_local", "secret_local")

	createResult, err := client.CreateInvoice(context.Background(), fiberclient.CreateInvoiceInput{
		PostID:     "ord_1:ms_1",
		FromUserID: "buyer_1",
		ToUserID:   "provider_1",
		Asset:      "CKB",
		Amount:     "12.5",
		Message:    "local smoke invoice",
	})
	if err != nil {
		t.Fatalf("create invoice: %v", err)
	}
	if createResult.Invoice == "" {
		t.Fatalf("expected invoice id, got %+v", createResult)
	}

	settledFeed, err := client.ListSettledFeed(context.Background(), fiberclient.SettledFeedInput{Limit: 10})
	if err != nil {
		t.Fatalf("list settled feed: %v", err)
	}
	if len(settledFeed.Items) != 1 {
		t.Fatalf("expected one settled item, got %+v", settledFeed.Items)
	}
	item := settledFeed.Items[0]
	if item.Invoice != createResult.Invoice || item.PostID != "ord_1:ms_1" || item.FromUserID != "buyer_1" || item.ToUserID != "provider_1" {
		t.Fatalf("unexpected settled item: %+v", item)
	}
}

func TestServerQuotesAndTracksWithdrawals(t *testing.T) {
	server := httptest.NewServer(NewServer())
	defer server.Close()

	client := fiberclient.NewClient(server.URL, "app_local", "secret_local")

	quote, err := client.QuotePayout(context.Background(), fiberclient.QuotePayoutInput{
		UserID: "provider_1",
		Asset:  "USDI",
		Amount: "10",
		Destination: fiberclient.WithdrawalDestination{
			Kind:           "PAYMENT_REQUEST",
			PaymentRequest: "fiber:invoice:example",
		},
	})
	if err != nil {
		t.Fatalf("quote payout: %v", err)
	}
	if !quote.DestinationValid || quote.Asset != "USDI" || quote.Amount != "10" {
		t.Fatalf("unexpected quote result: %+v", quote)
	}

	withdrawal, err := client.RequestPayout(context.Background(), fiberclient.RequestPayoutInput{
		UserID: "provider_1",
		Asset:  "USDI",
		Amount: "10",
		Destination: fiberclient.WithdrawalDestination{
			Kind:           "PAYMENT_REQUEST",
			PaymentRequest: "fiber:invoice:example",
		},
	})
	if err != nil {
		t.Fatalf("request payout: %v", err)
	}
	if withdrawal.ID == "" || withdrawal.State != "PENDING" {
		t.Fatalf("unexpected withdrawal result: %+v", withdrawal)
	}

	statuses, err := client.ListWithdrawalStatuses(context.Background(), "provider_1")
	if err != nil {
		t.Fatalf("list withdrawal statuses: %v", err)
	}
	if len(statuses.Withdrawals) != 1 {
		t.Fatalf("expected one withdrawal, got %+v", statuses.Withdrawals)
	}
	item := statuses.Withdrawals[0]
	if item.ID != withdrawal.ID || item.UserID != "provider_1" || item.State != "PROCESSING" {
		t.Fatalf("unexpected withdrawal status: %+v", item)
	}
}
