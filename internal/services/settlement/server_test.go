package settlement

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	fiberclient "github.com/chenyu/1-tok/internal/integrations/fiber"
	iamclient "github.com/chenyu/1-tok/internal/integrations/iam"
	"github.com/chenyu/1-tok/internal/serviceauth"
)

type stubFiberClient struct {
	createInput         fiberclient.CreateInvoiceInput
	createResult        fiberclient.CreateInvoiceResult
	statusInvoice       string
	statusResult        fiberclient.InvoiceStatusResult
	quoteInput          fiberclient.QuotePayoutInput
	quoteResult         fiberclient.QuotePayoutResult
	requestPayoutInput  fiberclient.RequestPayoutInput
	requestPayoutResult fiberclient.RequestPayoutResult
	settledFeedInput    fiberclient.SettledFeedInput
	settledFeedResult   fiberclient.SettledFeedResult
	withdrawalsUserID   string
	withdrawalsResult   fiberclient.WithdrawalStatusResult
}

type stubIAMClient struct {
	token string
	actor iamclient.Actor
}

func (s *stubIAMClient) GetActor(_ context.Context, bearerToken string) (iamclient.Actor, error) {
	s.token = bearerToken
	return s.actor, nil
}

func (s *stubFiberClient) CreateInvoice(_ context.Context, input fiberclient.CreateInvoiceInput) (fiberclient.CreateInvoiceResult, error) {
	s.createInput = input
	return s.createResult, nil
}

func (s *stubFiberClient) GetInvoiceStatus(_ context.Context, invoice string) (fiberclient.InvoiceStatusResult, error) {
	s.statusInvoice = invoice
	return s.statusResult, nil
}

func (s *stubFiberClient) QuotePayout(_ context.Context, input fiberclient.QuotePayoutInput) (fiberclient.QuotePayoutResult, error) {
	s.quoteInput = input
	return s.quoteResult, nil
}

func (s *stubFiberClient) RequestPayout(_ context.Context, input fiberclient.RequestPayoutInput) (fiberclient.RequestPayoutResult, error) {
	s.requestPayoutInput = input
	return s.requestPayoutResult, nil
}

func (s *stubFiberClient) ListSettledFeed(_ context.Context, input fiberclient.SettledFeedInput) (fiberclient.SettledFeedResult, error) {
	s.settledFeedInput = input
	return s.settledFeedResult, nil
}

func (s *stubFiberClient) ListWithdrawalStatuses(_ context.Context, userID string) (fiberclient.WithdrawalStatusResult, error) {
	s.withdrawalsUserID = userID
	return s.withdrawalsResult, nil
}

func TestCreateInvoiceUsesFiberClient(t *testing.T) {
	stub := &stubFiberClient{
		createResult: fiberclient.CreateInvoiceResult{Invoice: "inv_123"},
	}
	funding := NewMemoryFundingRecordRepository()
	server := NewServerWithOptions(Options{
		Upstream: "http://127.0.0.1:8080",
		Fiber:    stub,
		Funding:  funding,
	})

	body := map[string]any{
		"orderId":       "ord_1",
		"milestoneId":   "ms_1",
		"buyerOrgId":    "buyer_1",
		"providerOrgId": "provider_1",
		"asset":         "CKB",
		"amount":        "12.5",
		"memo":          "prefund milestone",
	}
	payload, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/invoices", bytes.NewReader(payload))
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", res.Code, res.Body.String())
	}

	if stub.createInput.PostID != "ord_1:ms_1" {
		t.Fatalf("expected post id ord_1:ms_1, got %q", stub.createInput.PostID)
	}
	if stub.createInput.FromUserID != "buyer_1" {
		t.Fatalf("expected from user buyer_1, got %q", stub.createInput.FromUserID)
	}
	if stub.createInput.ToUserID != "provider_1" {
		t.Fatalf("expected to user provider_1, got %q", stub.createInput.ToUserID)
	}
	if stub.createInput.Asset != "CKB" || stub.createInput.Amount != "12.5" {
		t.Fatalf("unexpected invoice input: %+v", stub.createInput)
	}
	if stub.createInput.Message != "prefund milestone" {
		t.Fatalf("expected message to be forwarded, got %q", stub.createInput.Message)
	}

	var response struct {
		Invoice string `json:"invoice"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Invoice != "inv_123" {
		t.Fatalf("expected invoice inv_123, got %q", response.Invoice)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/v1/funding-records", nil)
	listRes := httptest.NewRecorder()
	server.ServeHTTP(listRes, listReq)
	if listRes.Code != http.StatusOK {
		t.Fatalf("expected 200 from funding list, got %d body=%s", listRes.Code, listRes.Body.String())
	}

	var listResponse struct {
		Records []FundingRecord `json:"records"`
	}
	if err := json.Unmarshal(listRes.Body.Bytes(), &listResponse); err != nil {
		t.Fatalf("decode funding records: %v", err)
	}
	if len(listResponse.Records) != 1 {
		t.Fatalf("expected one funding record, got %+v", listResponse.Records)
	}
	if listResponse.Records[0].Kind != FundingRecordKindInvoice || listResponse.Records[0].Invoice != "inv_123" || listResponse.Records[0].State != "UNPAID" {
		t.Fatalf("unexpected funding record: %+v", listResponse.Records[0])
	}
}

func TestNewServerRequiresPersistentFundingStoreWhenConfigured(t *testing.T) {
	t.Setenv("ONE_TOK_REQUIRE_PERSISTENCE", "true")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("SETTLEMENT_DATABASE_URL", "")

	defer func() {
		if recovered := recover(); recovered == nil {
			t.Fatalf("expected NewServerWithOptions to panic when persistence is required and no database is configured")
		}
	}()

	_ = NewServerWithOptions(Options{
		Upstream: "http://upstream.internal",
		Fiber:    &stubFiberClient{},
	})
}

func TestNewServerRequiresExternalDependenciesWhenConfigured(t *testing.T) {
	t.Setenv("ONE_TOK_REQUIRE_EXTERNALS", "true")
	t.Setenv("IAM_UPSTREAM", "")
	t.Setenv("SETTLEMENT_SERVICE_TOKEN", "")
	t.Setenv("FIBER_RPC_URL", "")
	t.Setenv("FIBER_APP_ID", "")
	t.Setenv("FIBER_HMAC_SECRET", "")

	defer func() {
		if recovered := recover(); recovered == nil {
			t.Fatalf("expected NewServerWithOptions to panic when external dependencies are required and config is missing")
		}
	}()

	_ = NewServerWithOptions(Options{
		Upstream: "http://upstream.internal",
		Funding:  NewMemoryFundingRecordRepository(),
	})
}

func TestCreateInvoiceRejectsMissingServiceTokenWhenConfigured(t *testing.T) {
	t.Setenv("SETTLEMENT_SERVICE_TOKEN", "settlement-shared-token")

	stub := &stubFiberClient{
		createResult: fiberclient.CreateInvoiceResult{Invoice: "inv_123"},
	}
	server := NewServerWithOptions(Options{
		Upstream: "http://127.0.0.1:8080",
		Fiber:    stub,
		Funding:  NewMemoryFundingRecordRepository(),
	})

	body := map[string]any{
		"orderId":       "ord_1",
		"milestoneId":   "ms_1",
		"buyerOrgId":    "buyer_1",
		"providerOrgId": "provider_1",
		"asset":         "CKB",
		"amount":        "12.5",
	}
	payload, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/invoices", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)

	if res.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d body=%s", res.Code, res.Body.String())
	}
}

func TestCreateInvoiceAcceptsRotatedServiceTokenFromEnv(t *testing.T) {
	t.Setenv("SETTLEMENT_SERVICE_TOKEN", "")
	t.Setenv("SETTLEMENT_SERVICE_TOKENS", "current-token,next-token")

	stub := &stubFiberClient{
		createResult: fiberclient.CreateInvoiceResult{Invoice: "inv_123"},
	}
	server := NewServerWithOptions(Options{
		Upstream: "http://127.0.0.1:8080",
		Fiber:    stub,
		Funding:  NewMemoryFundingRecordRepository(),
	})

	body := map[string]any{
		"orderId":       "ord_1",
		"milestoneId":   "ms_1",
		"buyerOrgId":    "buyer_1",
		"providerOrgId": "provider_1",
		"asset":         "CKB",
		"amount":        "12.5",
	}
	payload, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/invoices", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(serviceauth.HeaderName, "next-token")
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("expected rotated settlement token to be accepted, got %d body=%s", res.Code, res.Body.String())
	}
}

func TestGetInvoiceStatusUsesFiberClient(t *testing.T) {
	stub := &stubFiberClient{
		createResult: fiberclient.CreateInvoiceResult{Invoice: "inv_123"},
		statusResult: fiberclient.InvoiceStatusResult{State: "SETTLED"},
	}
	funding := NewMemoryFundingRecordRepository()
	server := NewServerWithOptions(Options{
		Upstream: "http://127.0.0.1:8080",
		Fiber:    stub,
		Funding:  funding,
	})

	createBody := map[string]any{
		"orderId":       "ord_1",
		"milestoneId":   "ms_1",
		"buyerOrgId":    "buyer_1",
		"providerOrgId": "provider_1",
		"asset":         "CKB",
		"amount":        "12.5",
	}
	createPayload, _ := json.Marshal(createBody)
	createReq := httptest.NewRequest(http.MethodPost, "/v1/invoices", bytes.NewReader(createPayload))
	createRes := httptest.NewRecorder()
	server.ServeHTTP(createRes, createReq)
	if createRes.Code != http.StatusCreated {
		t.Fatalf("expected 201 from invoice creation, got %d body=%s", createRes.Code, createRes.Body.String())
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/invoices/inv_123", nil)
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", res.Code, res.Body.String())
	}
	if stub.statusInvoice != "inv_123" {
		t.Fatalf("expected invoice inv_123, got %q", stub.statusInvoice)
	}

	var response struct {
		Invoice string `json:"invoice"`
		State   string `json:"state"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Invoice != "inv_123" || response.State != "SETTLED" {
		t.Fatalf("unexpected response: %+v", response)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/v1/funding-records", nil)
	listRes := httptest.NewRecorder()
	server.ServeHTTP(listRes, listReq)
	if listRes.Code != http.StatusOK {
		t.Fatalf("expected 200 from funding list, got %d body=%s", listRes.Code, listRes.Body.String())
	}

	var listResponse struct {
		Records []FundingRecord `json:"records"`
	}
	if err := json.Unmarshal(listRes.Body.Bytes(), &listResponse); err != nil {
		t.Fatalf("decode funding records: %v", err)
	}
	if len(listResponse.Records) != 1 || listResponse.Records[0].State != "SETTLED" {
		t.Fatalf("expected settled funding record, got %+v", listResponse.Records)
	}
}

func TestOrderRoutesStillProxyToGateway(t *testing.T) {
	var receivedPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"proxied":true}`))
	}))
	defer upstream.Close()

	server := NewServerWithOptions(Options{
		Upstream: upstream.URL,
		Fiber:    &stubFiberClient{},
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/orders/ord_1/milestones/ms_1/settle", bytes.NewReader([]byte(`{"summary":"done"}`)))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", res.Code, res.Body.String())
	}
	if receivedPath != "/api/v1/orders/ord_1/milestones/ms_1/settle" {
		t.Fatalf("expected proxied path, got %q", receivedPath)
	}
}

func TestQuoteWithdrawalUsesFiberClient(t *testing.T) {
	stub := &stubFiberClient{
		quoteResult: fiberclient.QuotePayoutResult{
			Asset:             "CKB",
			Amount:            "61",
			MinimumAmount:     "61",
			AvailableBalance:  "124",
			LockedBalance:     "0",
			NetworkFee:        "0.00001",
			ReceiveAmount:     "60.99999",
			DestinationValid:  true,
			ValidationMessage: nil,
		},
	}
	server := NewServerWithOptions(Options{
		Upstream: "http://127.0.0.1:8080",
		Fiber:    stub,
	})

	body := map[string]any{
		"providerOrgId": "provider_1",
		"asset":         "CKB",
		"amount":        "61",
		"destination": map[string]any{
			"kind":    "CKB_ADDRESS",
			"address": "ckt1qyqfth8m4fevfzh5hhd088s78qcdjjp8cehs7z8jhw",
		},
	}
	payload, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/withdrawals/quote", bytes.NewReader(payload))
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", res.Code, res.Body.String())
	}
	if stub.quoteInput.UserID != "provider_1" || stub.quoteInput.Asset != "CKB" || stub.quoteInput.Amount != "61" {
		t.Fatalf("unexpected quote input: %+v", stub.quoteInput)
	}
	if stub.quoteInput.Destination.Kind != "CKB_ADDRESS" || stub.quoteInput.Destination.Address == "" {
		t.Fatalf("unexpected destination: %+v", stub.quoteInput.Destination)
	}
}

func TestRequestWithdrawalUsesFiberClient(t *testing.T) {
	stub := &stubFiberClient{
		requestPayoutResult: fiberclient.RequestPayoutResult{
			ID:    "wd_123",
			State: "PENDING",
		},
	}
	funding := NewMemoryFundingRecordRepository()
	server := NewServerWithOptions(Options{
		Upstream: "http://127.0.0.1:8080",
		Fiber:    stub,
		Funding:  funding,
	})

	body := map[string]any{
		"providerOrgId": "provider_1",
		"asset":         "USDI",
		"amount":        "10",
		"destination": map[string]any{
			"kind":           "PAYMENT_REQUEST",
			"paymentRequest": "fiber:invoice:example",
		},
	}
	payload, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/withdrawals", bytes.NewReader(payload))
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", res.Code, res.Body.String())
	}
	if stub.requestPayoutInput.UserID != "provider_1" || stub.requestPayoutInput.Asset != "USDI" || stub.requestPayoutInput.Amount != "10" {
		t.Fatalf("unexpected request input: %+v", stub.requestPayoutInput)
	}
	if stub.requestPayoutInput.Destination.Kind != "PAYMENT_REQUEST" || stub.requestPayoutInput.Destination.PaymentRequest == "" {
		t.Fatalf("unexpected destination: %+v", stub.requestPayoutInput.Destination)
	}

	var response struct {
		ID    string `json:"id"`
		State string `json:"state"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.ID != "wd_123" || response.State != "PENDING" {
		t.Fatalf("unexpected response: %+v", response)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/v1/funding-records", nil)
	listRes := httptest.NewRecorder()
	server.ServeHTTP(listRes, listReq)
	if listRes.Code != http.StatusOK {
		t.Fatalf("expected 200 from funding list, got %d body=%s", listRes.Code, listRes.Body.String())
	}

	var listResponse struct {
		Records []FundingRecord `json:"records"`
	}
	if err := json.Unmarshal(listRes.Body.Bytes(), &listResponse); err != nil {
		t.Fatalf("decode funding records: %v", err)
	}
	if len(listResponse.Records) != 1 {
		t.Fatalf("expected one funding record, got %+v", listResponse.Records)
	}
	record := listResponse.Records[0]
	if record.Kind != FundingRecordKindWithdrawal || record.ExternalID != "wd_123" || record.State != "PENDING" {
		t.Fatalf("unexpected withdrawal funding record: %+v", record)
	}
}

func TestSettledFeedUpdatesFundingRecordState(t *testing.T) {
	stub := &stubFiberClient{
		createResult: fiberclient.CreateInvoiceResult{Invoice: "inv_123"},
		settledFeedResult: fiberclient.SettledFeedResult{
			Items: []fiberclient.SettledFeedItem{
				{
					TipIntentID: "tip_1",
					PostID:      "ord_1:ms_1",
					Invoice:     "inv_123",
					Amount:      "12.5",
					Asset:       "CKB",
					FromUserID:  "buyer_1",
					ToUserID:    "provider_1",
					SettledAt:   "2026-03-12T00:00:00Z",
				},
			},
		},
	}
	funding := NewMemoryFundingRecordRepository()
	server := NewServerWithOptions(Options{
		Upstream: "http://127.0.0.1:8080",
		Fiber:    stub,
		Funding:  funding,
	})

	createBody := map[string]any{
		"orderId":       "ord_1",
		"milestoneId":   "ms_1",
		"buyerOrgId":    "buyer_1",
		"providerOrgId": "provider_1",
		"asset":         "CKB",
		"amount":        "12.5",
	}
	createPayload, _ := json.Marshal(createBody)
	createReq := httptest.NewRequest(http.MethodPost, "/v1/invoices", bytes.NewReader(createPayload))
	createRes := httptest.NewRecorder()
	server.ServeHTTP(createRes, createReq)
	if createRes.Code != http.StatusCreated {
		t.Fatalf("expected 201 from invoice creation, got %d body=%s", createRes.Code, createRes.Body.String())
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/settled-feed?limit=20", nil)
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", res.Code, res.Body.String())
	}
	if stub.settledFeedInput.Limit != 20 {
		t.Fatalf("expected settled feed limit 20, got %+v", stub.settledFeedInput)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/v1/funding-records", nil)
	listRes := httptest.NewRecorder()
	server.ServeHTTP(listRes, listReq)
	if listRes.Code != http.StatusOK {
		t.Fatalf("expected 200 from funding list, got %d body=%s", listRes.Code, listRes.Body.String())
	}

	var listResponse struct {
		Records []FundingRecord `json:"records"`
	}
	if err := json.Unmarshal(listRes.Body.Bytes(), &listResponse); err != nil {
		t.Fatalf("decode funding records: %v", err)
	}
	if len(listResponse.Records) != 1 || listResponse.Records[0].State != "SETTLED" {
		t.Fatalf("expected settled funding record after feed sync, got %+v", listResponse.Records)
	}
}

func TestSettledFeedRejectsMissingServiceTokenWhenConfigured(t *testing.T) {
	t.Setenv("SETTLEMENT_SERVICE_TOKEN", "settlement-shared-token")

	stub := &stubFiberClient{
		settledFeedResult: fiberclient.SettledFeedResult{
			Items: []fiberclient.SettledFeedItem{{Invoice: "inv_123", SettledAt: "2026-03-12T00:00:00Z"}},
		},
	}
	server := NewServerWithOptions(Options{
		Upstream: "http://127.0.0.1:8080",
		Fiber:    stub,
		Funding:  NewMemoryFundingRecordRepository(),
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/settled-feed?limit=20", nil)
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)

	if res.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d body=%s", res.Code, res.Body.String())
	}
}

func TestWithdrawalStatusSyncUpdatesFundingRecordState(t *testing.T) {
	stub := &stubFiberClient{
		requestPayoutResult: fiberclient.RequestPayoutResult{
			ID:    "wd_123",
			State: "PENDING",
		},
		withdrawalsResult: fiberclient.WithdrawalStatusResult{
			Withdrawals: []fiberclient.WithdrawalStatusItem{
				{
					ID:    "wd_123",
					State: "PROCESSING",
				},
			},
		},
	}
	funding := NewMemoryFundingRecordRepository()
	server := NewServerWithOptions(Options{
		Upstream: "http://127.0.0.1:8080",
		Fiber:    stub,
		Funding:  funding,
	})

	body := map[string]any{
		"providerOrgId": "provider_1",
		"asset":         "USDI",
		"amount":        "10",
		"destination": map[string]any{
			"kind":           "PAYMENT_REQUEST",
			"paymentRequest": "fiber:invoice:example",
		},
	}
	payload, _ := json.Marshal(body)
	createReq := httptest.NewRequest(http.MethodPost, "/v1/withdrawals", bytes.NewReader(payload))
	createRes := httptest.NewRecorder()
	server.ServeHTTP(createRes, createReq)
	if createRes.Code != http.StatusCreated {
		t.Fatalf("expected 201 from withdrawal creation, got %d body=%s", createRes.Code, createRes.Body.String())
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/withdrawals/status?providerOrgId=provider_1", nil)
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", res.Code, res.Body.String())
	}
	if stub.withdrawalsUserID != "provider_1" {
		t.Fatalf("expected provider_1 withdrawal sync, got %q", stub.withdrawalsUserID)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/v1/funding-records", nil)
	listRes := httptest.NewRecorder()
	server.ServeHTTP(listRes, listReq)
	if listRes.Code != http.StatusOK {
		t.Fatalf("expected 200 from funding list, got %d body=%s", listRes.Code, listRes.Body.String())
	}

	var listResponse struct {
		Records []FundingRecord `json:"records"`
	}
	if err := json.Unmarshal(listRes.Body.Bytes(), &listResponse); err != nil {
		t.Fatalf("decode funding records: %v", err)
	}
	if len(listResponse.Records) != 1 || listResponse.Records[0].State != "PROCESSING" {
		t.Fatalf("expected processing withdrawal record after status sync, got %+v", listResponse.Records)
	}
}

func TestFundingRecordsAreScopedToAuthenticatedProvider(t *testing.T) {
	funding := NewMemoryFundingRecordRepository()
	now := time.Now().UTC()
	if err := funding.Save(FundingRecord{
		ID:            "fund_1",
		Kind:          FundingRecordKindInvoice,
		OrderID:       "ord_1",
		ProviderOrgID: "provider_auth_1",
		Asset:         "CKB",
		Amount:        "10",
		State:         "SETTLED",
		CreatedAt:     now,
		UpdatedAt:     now,
	}); err != nil {
		t.Fatalf("seed fund_1: %v", err)
	}
	if err := funding.Save(FundingRecord{
		ID:            "fund_2",
		Kind:          FundingRecordKindInvoice,
		OrderID:       "ord_2",
		ProviderOrgID: "provider_other",
		Asset:         "CKB",
		Amount:        "12",
		State:         "SETTLED",
		CreatedAt:     now,
		UpdatedAt:     now,
	}); err != nil {
		t.Fatalf("seed fund_2: %v", err)
	}

	server := NewServerWithOptions(Options{
		Upstream: "http://127.0.0.1:8080",
		Fiber:    &stubFiberClient{},
		Funding:  funding,
		Auth: &stubIAMClient{
			actor: iamclient.Actor{
				UserID: "usr_1",
				Memberships: []iamclient.ActorMembership{
					{
						OrganizationID:   "provider_auth_1",
						OrganizationKind: "provider",
						Role:             "finance_viewer",
					},
				},
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/funding-records", nil)
	req.Header.Set("Authorization", "Bearer provider-token")
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", res.Code, res.Body.String())
	}

	var response struct {
		Records []FundingRecord `json:"records"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Records) != 1 || response.Records[0].ProviderOrgID != "provider_auth_1" {
		t.Fatalf("expected scoped funding records, got %+v", response.Records)
	}
}

func TestWithdrawalsUseAuthenticatedProviderMembership(t *testing.T) {
	stub := &stubFiberClient{
		requestPayoutResult: fiberclient.RequestPayoutResult{
			ID:    "wd_auth",
			State: "PENDING",
		},
		withdrawalsResult: fiberclient.WithdrawalStatusResult{
			Withdrawals: []fiberclient.WithdrawalStatusItem{
				{ID: "wd_auth", State: "PROCESSING"},
			},
		},
	}
	server := NewServerWithOptions(Options{
		Upstream: "http://127.0.0.1:8080",
		Fiber:    stub,
		Funding:  NewMemoryFundingRecordRepository(),
		Auth: &stubIAMClient{
			actor: iamclient.Actor{
				UserID: "usr_1",
				Memberships: []iamclient.ActorMembership{
					{
						OrganizationID:   "provider_auth_1",
						OrganizationKind: "provider",
						Role:             "finance_viewer",
					},
				},
			},
		},
	})

	body := map[string]any{
		"asset":  "USDI",
		"amount": "10",
		"destination": map[string]any{
			"kind":           "PAYMENT_REQUEST",
			"paymentRequest": "fiber:invoice:example",
		},
	}
	payload, _ := json.Marshal(body)
	createReq := httptest.NewRequest(http.MethodPost, "/v1/withdrawals", bytes.NewReader(payload))
	createReq.Header.Set("Authorization", "Bearer provider-token")
	createRes := httptest.NewRecorder()
	server.ServeHTTP(createRes, createReq)

	if createRes.Code != http.StatusCreated {
		t.Fatalf("expected 201 from withdrawal creation, got %d body=%s", createRes.Code, createRes.Body.String())
	}
	if stub.requestPayoutInput.UserID != "provider_auth_1" {
		t.Fatalf("expected authenticated provider org, got %+v", stub.requestPayoutInput)
	}

	statusReq := httptest.NewRequest(http.MethodGet, "/v1/withdrawals/status", nil)
	statusReq.Header.Set("Authorization", "Bearer provider-token")
	statusRes := httptest.NewRecorder()
	server.ServeHTTP(statusRes, statusReq)

	if statusRes.Code != http.StatusOK {
		t.Fatalf("expected 200 from status sync, got %d body=%s", statusRes.Code, statusRes.Body.String())
	}
	if stub.withdrawalsUserID != "provider_auth_1" {
		t.Fatalf("expected authenticated provider for status sync, got %q", stub.withdrawalsUserID)
	}
}
