package release

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	fiberclient "github.com/chenyu/1-tok/internal/integrations/fiber"
)

type FNNAdapterSmokeConfig struct {
	BaseURL        string
	AppID          string
	HMACSecret     string
	IncludePayment bool
}

type FNNAdapterSmokeSummary struct {
	Invoice          string `json:"invoice"`
	Status           string `json:"status"`
	QuoteValid       bool   `json:"quoteValid"`
	ValidationReason string `json:"validationReason,omitempty"`
	WithdrawalID     string `json:"withdrawalId,omitempty"`
}

func FNNAdapterSmokeConfigFromEnv() FNNAdapterSmokeConfig {
	return FNNAdapterSmokeConfig{
		BaseURL:        envOrDefault("RELEASE_FNN_ADAPTER_BASE_URL", "http://127.0.0.1:8091"),
		AppID:          envOrDefault("RELEASE_FNN_ADAPTER_APP_ID", "app_local"),
		HMACSecret:     envOrDefault("RELEASE_FNN_ADAPTER_HMAC_SECRET", "secret_local"),
		IncludePayment: envBoolDefaultFalse("RELEASE_FNN_ADAPTER_INCLUDE_PAYMENT"),
	}
}

func RunFNNAdapterSmoke(ctx context.Context, cfg FNNAdapterSmokeConfig) (FNNAdapterSmokeSummary, error) {
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	if baseURL == "" {
		return FNNAdapterSmokeSummary{}, errors.New("adapter base url is required")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/healthz", nil)
	if err != nil {
		return FNNAdapterSmokeSummary{}, err
	}
	res, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		return FNNAdapterSmokeSummary{}, fmt.Errorf("adapter health: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return FNNAdapterSmokeSummary{}, fmt.Errorf("adapter health returned %d", res.StatusCode)
	}

	client := fiberclient.NewClient(baseURL, cfg.AppID, cfg.HMACSecret)
	invoiceAmount := "12"
	createResult, err := client.CreateInvoice(ctx, fiberclient.CreateInvoiceInput{
		PostID:     "fnn_adapter_smoke:ms_1",
		FromUserID: "buyer_smoke",
		ToUserID:   "provider_smoke",
		Asset:      "CKB",
		Amount:     invoiceAmount,
		Message:    "fnn adapter smoke",
	})
	if err != nil {
		return FNNAdapterSmokeSummary{}, fmt.Errorf("create invoice: %w", err)
	}
	if strings.TrimSpace(createResult.Invoice) == "" {
		return FNNAdapterSmokeSummary{}, errors.New("create invoice returned empty invoice")
	}

	statusResult, err := client.GetInvoiceStatus(ctx, createResult.Invoice)
	if err != nil {
		return FNNAdapterSmokeSummary{}, fmt.Errorf("get invoice status: %w", err)
	}
	if strings.TrimSpace(statusResult.State) == "" {
		return FNNAdapterSmokeSummary{}, errors.New("get invoice status returned empty state")
	}

	quoteResult, err := client.QuotePayout(ctx, fiberclient.QuotePayoutInput{
		UserID: "provider_smoke",
		Asset:  "CKB",
		Amount: invoiceAmount,
		Destination: fiberclient.WithdrawalDestination{
			Kind:           "PAYMENT_REQUEST",
			PaymentRequest: createResult.Invoice,
		},
	})
	if err != nil {
		return FNNAdapterSmokeSummary{}, fmt.Errorf("quote payout: %w", err)
	}

	summary := FNNAdapterSmokeSummary{
		Invoice:    createResult.Invoice,
		Status:     statusResult.State,
		QuoteValid: quoteResult.DestinationValid,
	}
	if quoteResult.ValidationMessage != nil {
		summary.ValidationReason = *quoteResult.ValidationMessage
	}
	if cfg.IncludePayment {
		if !summary.QuoteValid {
			return FNNAdapterSmokeSummary{}, errors.New("quote payout is invalid; refusing to request payment")
		}
		payoutResult, err := client.RequestPayout(ctx, fiberclient.RequestPayoutInput{
			UserID: "provider_smoke",
			Asset:  "CKB",
			Amount: invoiceAmount,
			Destination: fiberclient.WithdrawalDestination{
				Kind:           "PAYMENT_REQUEST",
				PaymentRequest: createResult.Invoice,
			},
		})
		if err != nil {
			return FNNAdapterSmokeSummary{}, fmt.Errorf("request payout: %w", err)
		}
		if strings.TrimSpace(payoutResult.ID) == "" {
			return FNNAdapterSmokeSummary{}, errors.New("request payout returned empty id")
		}
		withdrawalResult, err := client.ListWithdrawalStatuses(ctx, "provider_smoke")
		if err != nil {
			return FNNAdapterSmokeSummary{}, fmt.Errorf("list withdrawal statuses: %w", err)
		}
		found := false
		for _, withdrawal := range withdrawalResult.Withdrawals {
			if withdrawal.ID == payoutResult.ID {
				found = true
				break
			}
		}
		if !found {
			return FNNAdapterSmokeSummary{}, fmt.Errorf("requested payout %q not present in dashboard summary", payoutResult.ID)
		}
		summary.WithdrawalID = payoutResult.ID
	}
	return summary, nil
}

func writeFNNAdapterArtifact(summary FNNAdapterSmokeSummary) error {
	return WriteJSONArtifact(os.Getenv("RELEASE_FNN_ADAPTER_OUTPUT_PATH"), summary)
}

func envBoolDefaultFalse(key string) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return false
	}
	switch strings.ToLower(value) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
