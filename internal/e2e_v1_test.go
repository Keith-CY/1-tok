//go:build integration

package internal_test

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/chenyu/1-tok/internal/bootstrap"
	"github.com/chenyu/1-tok/internal/gateway"
	iamserver "github.com/chenyu/1-tok/internal/services/iam"
	"github.com/chenyu/1-tok/internal/identity"
)

func TestV1BusinessFlowE2E(t *testing.T) {
	dsn := os.Getenv("ONE_TOK_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ONE_TOK_TEST_DATABASE_URL not set")
	}
	t.Setenv("DATABASE_URL", dsn)
	t.Setenv("NATS_URL", "")

	app, cleanup, err := bootstrap.LoadPlatformApp()
	if err != nil { t.Fatal(err) }
	defer cleanup()

	iam := iamserver.NewServerWithOptions(iamserver.Options{Store: identity.NewMemoryStore()})
	gw, err := gateway.NewServerWithOptionsE(gateway.Options{App: app})
	if err != nil { t.Fatal(err) }

	_ = signup(t, iam, `{"email":"v1b@test.com","password":"correct horse battery staple 123","name":"B","organizationName":"Buyers","organizationKind":"buyer"}`)
	_ = signup(t, iam, `{"email":"v1p@test.com","password":"correct horse battery staple 123","name":"P","organizationName":"Providers","organizationKind":"provider"}`)

	get := func(path string) *httptest.ResponseRecorder { return gwRequest(t, gw, "GET", path, nil) }
	post := func(path string, payload any) *httptest.ResponseRecorder { return gwRequest(t, gw, "POST", path, payload) }
	patch := func(path string, payload any) *httptest.ResponseRecorder { return gwRequest(t, gw, "PATCH", path, payload) }

	extract := func(resp *httptest.ResponseRecorder, keys ...string) string {
		var m map[string]any
		if err := json.Unmarshal(resp.Body.Bytes(), &m); err != nil {
			t.Fatalf("json parse: %v (body=%s)", err, resp.Body.String())
		}
		cur := m
		for i, k := range keys {
			if i == len(keys)-1 {
				return fmt.Sprintf("%v", cur[k])
			}
			cur = cur[k].(map[string]any)
		}
		return ""
	}

	expect := func(resp *httptest.ResponseRecorder, code int) {
		t.Helper()
		if resp.Code != code {
			t.Fatalf("[line %d] want %d got %d: %s", 0, code, resp.Code, resp.Body.String())
		}
	}

	// 1-7: Read-only endpoints
	expect(get("/api/v1/stats"), 200)
	expect(get("/api/v1/leaderboard"), 200)
	expect(get("/api/v1/listings?q=agent&sort=price_asc"), 200)
	expect(get("/api/v1/providers?capability=carrier"), 200)
	expect(get("/api/v1/providers/provider_1"), 200)
	expect(get("/api/v1/listings/listing_1"), 200)
	expect(get("/api/v1/system"), 200)

	// 8: Create RFQ
	r := post("/api/v1/rfqs", map[string]any{
		"buyerOrgId": "org_1", "title": "E2E", "category": "agent-ops",
		"scope": "test", "budgetCents": 100000,
		"responseDeadlineAt": time.Now().Add(72 * time.Hour).Format(time.RFC3339),
	})
	expect(r, 201)
	rfqID := extract(r, "rfq", "id")

	// 9-10: RFQ read
	expect(get("/api/v1/rfqs/"+rfqID), 200)
	expect(get("/api/v1/rfqs?status=open"), 200)

	// 11: RFQ messages
	expect(post("/api/v1/rfqs/"+rfqID+"/messages", map[string]any{"author": "buyer", "body": "hi"}), 201)
	expect(get("/api/v1/rfqs/"+rfqID+"/messages"), 200)

	// 12: Bid
	r = post("/api/v1/rfqs/"+rfqID+"/bids", map[string]any{
		"providerOrgId": "org_2", "message": "bid", "quoteCents": 90000,
		"milestones": []map[string]any{
			{"id": "ms_1", "title": "Setup", "basePriceCents": 18000, "budgetCents": 18000},
			{"id": "ms_2", "title": "Execute", "basePriceCents": 54000, "budgetCents": 54000},
			{"id": "ms_3", "title": "Deliver", "basePriceCents": 18000, "budgetCents": 18000},
		},
	})
	expect(r, 201)
	bidID := extract(r, "bid", "id")
	expect(get("/api/v1/rfqs/"+rfqID+"/bids"), 200)

	// 13: Award
	r = post("/api/v1/rfqs/"+rfqID+"/award", map[string]any{"bidId": bidID, "fundingMode": "prepaid"})
	expect(r, 200)
	orderID := extract(r, "order", "id")

	// 14-18: Order reads
	expect(get("/api/v1/orders/"+orderID), 200)
	expect(get("/api/v1/orders?status=running"), 200)
	expect(get("/api/v1/orders/"+orderID+"/budget"), 200)
	expect(get("/api/v1/orders/"+orderID+"/timeline"), 200)

	// 19: Order messages
	expect(post("/api/v1/messages", map[string]any{"orderId": orderID, "author": "b", "body": "?"}), 201)
	expect(get("/api/v1/orders/"+orderID+"/messages"), 200)

	// 20: Usage charge
	expect(post("/api/v1/orders/"+orderID+"/milestones/ms_1/usage", map[string]any{"kind": "token", "amountCents": 5000}), 200)

	// 21: Settle ms_1
	expect(post("/api/v1/orders/"+orderID+"/milestones/ms_1/settle", map[string]any{"milestoneId": "ms_1", "summary": "done"}), 200)

	// 22-27: Carrier (mocked in-memory)
	bindResp := post("/api/v1/orders/"+orderID+"/milestones/ms_2/bind-carrier", map[string]any{"carrierId": "c1", "capabilities": []string{"gpu"}})
	expect(bindResp, 201)
	bindID := extract(bindResp, "binding", "id")

	expect(get("/api/v1/orders/"+orderID+"/milestones/ms_2/bind-carrier"), 200)

	jobResp := post("/api/v1/orders/"+orderID+"/milestones/ms_2/jobs", map[string]any{"bindingId": bindID, "input": "{}"})
	expect(jobResp, 201)
	jobID := extract(jobResp, "job", "id")

	expect(patch("/api/v1/jobs/"+jobID+"/start", nil), 200)
	expect(post("/api/v1/jobs/"+jobID+"/progress", map[string]any{"step": 5, "total": 10, "message": "mid"}), 200)
	expect(post("/api/v1/jobs/"+jobID+"/heartbeat", nil), 200)
	expect(patch("/api/v1/jobs/"+jobID+"/complete", map[string]any{"output": "result"}), 200)
	expect(get("/api/v1/jobs/"+jobID), 200)
	expect(get("/api/v1/orders/"+orderID+"/milestones/ms_2/jobs"), 200)

	// 28: Evidence
	expect(post("/api/v1/jobs/"+jobID+"/evidence", map[string]any{
		"summary": "done", "artifacts": []map[string]any{{"name": "log", "type": "log", "url": "http://test/log"}},
	}), 201)
	expect(get("/api/v1/jobs/"+jobID+"/evidence"), 200)

	// 29: Settle ms_2, ms_3 → order complete
	expect(post("/api/v1/orders/"+orderID+"/milestones/ms_2/settle", map[string]any{"milestoneId": "ms_2", "summary": "ok"}), 200)
	expect(post("/api/v1/orders/"+orderID+"/milestones/ms_3/settle", map[string]any{"milestoneId": "ms_3", "summary": "ok"}), 200)

	// 30: Rating
	expect(post("/api/v1/orders/"+orderID+"/rating", map[string]any{"score": 5, "comment": "great"}), 201)
	expect(get("/api/v1/orders/"+orderID+"/rating"), 200)

	// 31: Dispute (new order)
	r = post("/api/v1/rfqs", map[string]any{"buyerOrgId": "org_1", "title": "D", "category": "ai", "scope": "d", "budgetCents": 20000, "responseDeadlineAt": time.Now().Add(72 * time.Hour).Format(time.RFC3339)})
	r2ID := extract(r, "rfq", "id")
	r = post("/api/v1/rfqs/"+r2ID+"/bids", map[string]any{"providerOrgId": "org_2", "message": "b", "quoteCents": 20000, "milestones": []map[string]any{{"id": "ms_1", "title": "W", "basePriceCents": 20000, "budgetCents": 20000}}})
	b2ID := extract(r, "bid", "id")
	r = post("/api/v1/rfqs/"+r2ID+"/award", map[string]any{"bidId": b2ID, "fundingMode": "prepaid"})
	o2ID := extract(r, "order", "id")

	// Settle ms_1 before disputing
	expect(post("/api/v1/orders/"+o2ID+"/milestones/ms_1/settle", map[string]any{"milestoneId": "ms_1", "summary": "done"}), 200)

	expect(post("/api/v1/orders/"+o2ID+"/disputes", map[string]any{"milestoneId": "ms_1", "reason": "bad", "refundCents": 5000}), 201)
	expect(get("/api/v1/disputes?status=open"), 200)

	// 32: Provider application
	expect(post("/api/v1/provider-applications", map[string]any{"orgId": "org_new", "name": "New", "capabilities": []string{"gpu"}}), 201)
	expect(get("/api/v1/provider-applications?status=pending"), 200)

	// 33: Webhooks
	expect(post("/api/v1/webhooks", map[string]any{"target": "org_1", "url": "http://test/hook"}), 201)
	expect(get("/api/v1/webhooks"), 200)

	// 34: Batch status
	expect(post("/api/v1/orders/batch-status", map[string]any{"orderIds": []string{orderID, o2ID}}), 200)

	// 35: CSV export
	expect(get("/api/v1/export/orders"), 200)
	expect(get("/api/v1/export/disputes"), 200)

	// 36: Top-up
	expect(post("/api/v1/orders/"+o2ID+"/top-up", map[string]any{"milestoneId": "ms_1", "additionalCents": 5000}), 200)

	// 37: Provider revenue
	expect(get("/api/v1/providers/provider_1/revenue"), 200)

	// 38: Validation errors
	expect(post("/api/v1/rfqs", map[string]any{"title": "", "budgetCents": 0}), 400)
	expect(post("/api/v1/orders/"+orderID+"/rating", map[string]any{"score": 10}), 400)

	t.Logf("=== V1 E2E: 38 test cases PASSED — all business lines covered ===")
}
