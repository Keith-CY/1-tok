package settlement

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	fiberclient "github.com/chenyu/1-tok/internal/integrations/fiber"
)

type stubFiberClient struct {
	createInput  fiberclient.CreateInvoiceInput
	createResult fiberclient.CreateInvoiceResult
	statusInvoice string
	statusResult  fiberclient.InvoiceStatusResult
}

func (s *stubFiberClient) CreateInvoice(_ context.Context, input fiberclient.CreateInvoiceInput) (fiberclient.CreateInvoiceResult, error) {
	s.createInput = input
	return s.createResult, nil
}

func (s *stubFiberClient) GetInvoiceStatus(_ context.Context, invoice string) (fiberclient.InvoiceStatusResult, error) {
	s.statusInvoice = invoice
	return s.statusResult, nil
}

func TestCreateInvoiceUsesFiberClient(t *testing.T) {
	stub := &stubFiberClient{
		createResult: fiberclient.CreateInvoiceResult{Invoice: "inv_123"},
	}
	server := NewServerWithOptions(Options{
		Upstream: "http://127.0.0.1:8080",
		Fiber:    stub,
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
}

func TestGetInvoiceStatusUsesFiberClient(t *testing.T) {
	stub := &stubFiberClient{
		statusResult: fiberclient.InvoiceStatusResult{State: "SETTLED"},
	}
	server := NewServerWithOptions(Options{
		Upstream: "http://127.0.0.1:8080",
		Fiber:    stub,
	})

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
