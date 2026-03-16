//go:build integration

package internal_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"context"
	"github.com/chenyu/1-tok/internal/bootstrap"
	"github.com/chenyu/1-tok/internal/gateway"
	"github.com/chenyu/1-tok/internal/integrations/iam"
)

type stubIAMClient struct {
	actor iam.Actor
}

func (s *stubIAMClient) GetActor(_ context.Context, _ string) (iam.Actor, error) {
	return s.actor, nil
}

func gwExtract(t *testing.T, resp *httptest.ResponseRecorder, keys ...string) string {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &m); err != nil {
		t.Fatalf("extract: json parse: %v (body=%s)", err, resp.Body.String())
	}
	cur := m
	for i, k := range keys {
		if i == len(keys)-1 {
			v, ok := cur[k]
			if !ok {
				t.Fatalf("extract: missing key %q", k)
			}
			return fmt.Sprintf("%v", v)
		}
		next, ok := cur[k].(map[string]any)
		if !ok {
			t.Fatalf("extract: key %q is not object", k)
		}
		cur = next
	}
	return ""
}

func gwExpect(t *testing.T, resp *httptest.ResponseRecorder, code int) {
	t.Helper()
	if resp.Code != code {
		t.Fatalf("want %d got %d: %s", code, resp.Code, resp.Body.String())
	}
}

func TestV1BusinessFlowE2E(t *testing.T) {
	dsn := os.Getenv("ONE_TOK_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ONE_TOK_TEST_DATABASE_URL not set")
	}
	t.Setenv("DATABASE_URL", dsn)
	t.Setenv("NATS_URL", "")
	t.Setenv("IAM_UPSTREAM", "")
	t.Setenv("ONE_TOK_REQUIRE_EXTERNALS", "")

	app, cleanup, err := bootstrap.LoadPlatformApp()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	gw, err := gateway.NewServerWithOptionsE(gateway.Options{App: app})
	if err != nil {
		t.Fatal(err)
	}

	// Gateway uses NoopClient (IAM_UPSTREAM forced empty above).
	// Auth enforcement tested separately in gateway/server_test.go.

	get := func(path string) *httptest.ResponseRecorder { return gwRequest(t, gw, "GET", path, nil) }
	post := func(path string, payload any) *httptest.ResponseRecorder {
		return gwRequest(t, gw, "POST", path, payload)
	}
	patch := func(path string, payload any) *httptest.ResponseRecorder {
		return gwRequest(t, gw, "PATCH", path, payload)
	}

	// 1-7: Read-only endpoints
	gwExpect(t, get("/api/v1/stats"), 200)
	gwExpect(t, get("/api/v1/leaderboard"), 200)
	gwExpect(t, get("/api/v1/listings?q=agent&sort=price_asc"), 200)
	gwExpect(t, get("/api/v1/providers?capability=carrier"), 200)
	gwExpect(t, get("/api/v1/providers/provider_1"), 200)
	gwExpect(t, get("/api/v1/listings/listing_1"), 200)
	gwExpect(t, get("/api/v1/system"), 200)

	// 8: Create RFQ
	r := post("/api/v1/rfqs", map[string]any{
		"buyerOrgId": "org_1", "title": "E2E", "category": "agent-ops",
		"scope": "test", "budgetCents": 100000,
		"responseDeadlineAt": time.Now().Add(72 * time.Hour).Format(time.RFC3339),
	})
	gwExpect(t, r, 201)
	rfqID := gwExtract(t, r, "rfq", "id")

	// 9-10: RFQ read
	gwExpect(t, get("/api/v1/rfqs/"+rfqID), 200)
	gwExpect(t, get("/api/v1/rfqs?status=open"), 200)

	// 11: RFQ messages
	gwExpect(t, post("/api/v1/rfqs/"+rfqID+"/messages", map[string]any{"author": "buyer", "body": "hi"}), 201)
	gwExpect(t, get("/api/v1/rfqs/"+rfqID+"/messages"), 200)

	// 12: Bid
	r = post("/api/v1/rfqs/"+rfqID+"/bids", map[string]any{
		"providerOrgId": "org_2", "message": "bid", "quoteCents": 90000,
		"milestones": []map[string]any{
			{"id": "ms_1", "title": "Setup", "basePriceCents": 18000, "budgetCents": 18000},
			{"id": "ms_2", "title": "Execute", "basePriceCents": 54000, "budgetCents": 54000},
			{"id": "ms_3", "title": "Deliver", "basePriceCents": 18000, "budgetCents": 18000},
		},
	})
	gwExpect(t, r, 201)
	bidID := gwExtract(t, r, "bid", "id")
	gwExpect(t, get("/api/v1/rfqs/"+rfqID+"/bids"), 200)

	// 13: Award
	r = post("/api/v1/rfqs/"+rfqID+"/award", map[string]any{"bidId": bidID, "fundingMode": "prepaid"})
	gwExpect(t, r, 200)
	orderID := gwExtract(t, r, "order", "id")

	// 14-18: Order reads
	gwExpect(t, get("/api/v1/orders/"+orderID), 200)
	gwExpect(t, get("/api/v1/orders?status=running"), 200)
	gwExpect(t, get("/api/v1/orders/"+orderID+"/budget"), 200)
	gwExpect(t, get("/api/v1/orders/"+orderID+"/timeline"), 200)

	// 19: Order messages
	gwExpect(t, post("/api/v1/messages", map[string]any{"orderId": orderID, "author": "b", "body": "?"}), 201)
	gwExpect(t, get("/api/v1/orders/"+orderID+"/messages"), 200)

	// 20: Usage charge
	gwExpect(t, post("/api/v1/orders/"+orderID+"/milestones/ms_1/usage", map[string]any{"kind": "token", "amountCents": 5000}), 200)

	// 21: Settle ms_1
	gwExpect(t, post("/api/v1/orders/"+orderID+"/milestones/ms_1/settle", map[string]any{"milestoneId": "ms_1", "summary": "done"}), 200)

	// 22-27: Carrier (mocked in-memory)
	bindResp := post("/api/v1/orders/"+orderID+"/milestones/ms_2/bind-carrier", map[string]any{"carrierId": "c1", "capabilities": []string{"gpu"}})
	gwExpect(t, bindResp, 201)
	bindID := gwExtract(t, bindResp, "binding", "id")

	gwExpect(t, get("/api/v1/orders/"+orderID+"/milestones/ms_2/bind-carrier"), 200)

	jobResp := post("/api/v1/orders/"+orderID+"/milestones/ms_2/jobs", map[string]any{"bindingId": bindID, "input": "{}"})
	gwExpect(t, jobResp, 201)
	jobID := gwExtract(t, jobResp, "job", "id")

	gwExpect(t, patch("/api/v1/jobs/"+jobID+"/start", nil), 200)
	gwExpect(t, post("/api/v1/jobs/"+jobID+"/progress", map[string]any{"step": 5, "total": 10, "message": "mid"}), 200)
	gwExpect(t, post("/api/v1/jobs/"+jobID+"/heartbeat", nil), 200)
	gwExpect(t, patch("/api/v1/jobs/"+jobID+"/complete", map[string]any{"output": "result"}), 200)
	gwExpect(t, get("/api/v1/jobs/"+jobID), 200)
	gwExpect(t, get("/api/v1/orders/"+orderID+"/milestones/ms_2/jobs"), 200)

	// 28: Evidence
	gwExpect(t, post("/api/v1/jobs/"+jobID+"/evidence", map[string]any{
		"summary": "done", "artifacts": []map[string]any{{"name": "log", "type": "log", "url": "http://test/log"}},
	}), 201)
	gwExpect(t, get("/api/v1/jobs/"+jobID+"/evidence"), 200)

	// 29: Settle ms_2, ms_3 → order complete
	gwExpect(t, post("/api/v1/orders/"+orderID+"/milestones/ms_2/settle", map[string]any{"milestoneId": "ms_2", "summary": "ok"}), 200)
	gwExpect(t, post("/api/v1/orders/"+orderID+"/milestones/ms_3/settle", map[string]any{"milestoneId": "ms_3", "summary": "ok"}), 200)

	// 30: Rating
	gwExpect(t, post("/api/v1/orders/"+orderID+"/rating", map[string]any{"score": 5, "comment": "great"}), 201)
	gwExpect(t, get("/api/v1/orders/"+orderID+"/rating"), 200)

	// 31: Dispute (new order)
	r = post("/api/v1/rfqs", map[string]any{"buyerOrgId": "org_1", "title": "D", "category": "ai", "scope": "d", "budgetCents": 20000, "responseDeadlineAt": time.Now().Add(72 * time.Hour).Format(time.RFC3339)})
	r2ID := gwExtract(t, r, "rfq", "id")
	r = post("/api/v1/rfqs/"+r2ID+"/bids", map[string]any{"providerOrgId": "org_2", "message": "b", "quoteCents": 20000, "milestones": []map[string]any{{"id": "ms_1", "title": "W", "basePriceCents": 20000, "budgetCents": 20000}}})
	b2ID := gwExtract(t, r, "bid", "id")
	r = post("/api/v1/rfqs/"+r2ID+"/award", map[string]any{"bidId": b2ID, "fundingMode": "prepaid"})
	o2ID := gwExtract(t, r, "order", "id")

	// Settle ms_1 before disputing
	gwExpect(t, post("/api/v1/orders/"+o2ID+"/milestones/ms_1/settle", map[string]any{"milestoneId": "ms_1", "summary": "done"}), 200)

	gwExpect(t, post("/api/v1/orders/"+o2ID+"/disputes", map[string]any{"milestoneId": "ms_1", "reason": "bad", "refundCents": 5000}), 201)
	gwExpect(t, get("/api/v1/disputes?status=open"), 200)

	// 32: Provider application
	gwExpect(t, post("/api/v1/provider-applications", map[string]any{"orgId": "org_new", "name": "New", "capabilities": []string{"gpu"}}), 201)
	gwExpect(t, get("/api/v1/provider-applications?status=pending"), 200)

	// 33: Webhooks
	gwExpect(t, post("/api/v1/webhooks", map[string]any{"target": "org_1", "url": "http://test/hook"}), 201)
	gwExpect(t, get("/api/v1/webhooks"), 200)

	// 34: Batch status
	gwExpect(t, post("/api/v1/orders/batch-status", map[string]any{"orderIds": []string{orderID, o2ID}}), 200)

	// 35: CSV export
	gwExpect(t, get("/api/v1/export/orders"), 200)
	gwExpect(t, get("/api/v1/export/disputes"), 200)

	// 36: Top-up
	gwExpect(t, post("/api/v1/orders/"+o2ID+"/top-up", map[string]any{"milestoneId": "ms_1", "additionalCents": 5000}), 200)

	// 37: Provider revenue
	gwExpect(t, get("/api/v1/providers/provider_1/revenue"), 200)

	// 38: Validation errors
	gwExpect(t, post("/api/v1/rfqs", map[string]any{"title": "", "budgetCents": 0}), 400)
	gwExpect(t, post("/api/v1/orders/"+orderID+"/rating", map[string]any{"score": 10}), 400)

	t.Logf("=== V1 E2E: 38 test cases PASSED — all business lines covered ===")
}

func TestV1BusinessFlowE2E_DisputeLifecycle(t *testing.T) {
	dsn := os.Getenv("ONE_TOK_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ONE_TOK_TEST_DATABASE_URL not set")
	}
	t.Setenv("DATABASE_URL", dsn)
	t.Setenv("NATS_URL", "")
	t.Setenv("IAM_UPSTREAM", "")
	t.Setenv("ONE_TOK_REQUIRE_EXTERNALS", "")

	app, cleanup, err := bootstrap.LoadPlatformApp()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	gw, err := gateway.NewServerWithOptionsE(gateway.Options{App: app})
	if err != nil {
		t.Fatal(err)
	}

	post := func(path string, payload any) *httptest.ResponseRecorder {
		return gwRequest(t, gw, "POST", path, payload)
	}
	get := func(path string) *httptest.ResponseRecorder { return gwRequest(t, gw, "GET", path, nil) }

	r := post("/api/v1/rfqs", map[string]any{
		"buyerOrgId": "org_1", "title": "E2E Dispute", "category": "agent-ops",
		"scope": "test", "budgetCents": 100000,
		"responseDeadlineAt": time.Now().Add(72 * time.Hour).Format(time.RFC3339),
	})
	gwExpect(t, r, 201)
	rfqID := gwExtract(t, r, "rfq", "id")

	r = post("/api/v1/rfqs/"+rfqID+"/bids", map[string]any{
		"providerOrgId": "org_2", "message": "bid", "quoteCents": 90000,
		"milestones": []map[string]any{
			{"id": "ms_1", "title": "Setup", "basePriceCents": 30000, "budgetCents": 30000},
			{"id": "ms_2", "title": "Exec", "basePriceCents": 30000, "budgetCents": 30000},
			{"id": "ms_3", "title": "Done", "basePriceCents": 30000, "budgetCents": 30000},
		},
	})
	gwExpect(t, r, 201)
	bidID := gwExtract(t, r, "bid", "id")

	r = post("/api/v1/rfqs/"+rfqID+"/award", map[string]any{"bidId": bidID, "fundingMode": "prepaid"})
	gwExpect(t, r, 200)
	orderID := gwExtract(t, r, "order", "id")

	gwExpect(t, post("/api/v1/orders/"+orderID+"/milestones/ms_1/settle", map[string]any{"milestoneId": "ms_1", "summary": "ok"}), 200)
	r = post("/api/v1/orders/"+orderID+"/disputes", map[string]any{"milestoneId": "ms_1", "reason": "bad", "refundCents": 10000})
	gwExpect(t, r, 201)

	dResp := get("/api/v1/disputes?status=open")
	gwExpect(t, dResp, 200)
	type disputeItem struct {
		ID string `json:"id"`
	}
	type disputesResp struct {
		Disputes []disputeItem `json:"disputes"`
	}
	var list disputesResp
	if err := json.Unmarshal(dResp.Body.Bytes(), &list); err != nil {
		t.Fatalf("unmarshal disputes: %v", err)
	}
	if len(list.Disputes) == 0 {
		t.Fatal("no open dispute found")
	}
	disputeID := list.Disputes[0].ID
	if disputeID == "" {
		t.Fatal("dispute id empty")
	}

	r = post("/api/v1/disputes/"+disputeID+"/resolve", map[string]any{"resolution": "approved", "resolvedBy": "ops_admin"})
	gwExpect(t, r, 200)

	dResp = get("/api/v1/disputes?status=resolved")
	gwExpect(t, dResp, 200)
	if !strings.Contains(dResp.Body.String(), disputeID) {
		t.Fatalf("expected resolved dispute in response, body=%s", dResp.Body.String())
	}
}

func TestV1BusinessFlowE2E_AuthAndStateGuards(t *testing.T) {
	dsn := os.Getenv("ONE_TOK_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ONE_TOK_TEST_DATABASE_URL not set")
	}
	t.Setenv("DATABASE_URL", dsn)
	t.Setenv("NATS_URL", "")
	t.Setenv("IAM_UPSTREAM", "")
	t.Setenv("ONE_TOK_REQUIRE_EXTERNALS", "")

	app, cleanup, err := bootstrap.LoadPlatformApp()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	actor := iam.Actor{
		UserID:      "u_buyer",
		Memberships: []iam.ActorMembership{{OrganizationID: "org_b", OrganizationKind: "buyer", Role: "org_owner"}},
	}
	gw, err := gateway.NewServerWithOptionsE(gateway.Options{
		App: app,
		IAM: &stubIAMClient{actor: actor},
	})
	if err != nil {
		t.Fatal(err)
	}

	post := func(path string, payload any) *httptest.ResponseRecorder {
		return gwRequest(t, gw, "POST", path, payload)
	}
	get := func(path string) *httptest.ResponseRecorder { return gwRequest(t, gw, "GET", path, nil) }
	assert := func(resp *httptest.ResponseRecorder, code int) {
		t.Helper()
		if resp.Code != code {
			t.Fatalf("want %d got %d: %s", code, resp.Code, resp.Body.String())
		}
	}

	// 401: ops resolve without token when IAM enabled
	assert(post("/api/v1/disputes/disp_fake/resolve", map[string]any{"resolution": "x", "resolvedBy": "ops"}), http.StatusUnauthorized)

	// 403: non-ops org trying to read disputes list
	req := httptest.NewRequest("GET", "/api/v1/disputes", nil)
	req.Header.Set("Authorization", "Bearer x")
	rec := httptest.NewRecorder()
	gw.ServeHTTP(rec, req)
	assert(rec, http.StatusForbidden)

	// 404: nonexistent order
	assert(get("/api/v1/orders/order_not_exists"), http.StatusNotFound)

	// 409: illegal state transition (complete job before start)
	bindResp := post("/api/v1/orders/ord_1/milestones/ms_1/bind-carrier", map[string]any{"carrierId": "c1", "capabilities": []string{"gpu"}})
	assert(bindResp, http.StatusCreated)
	var bindData map[string]any
	json.Unmarshal(bindResp.Body.Bytes(), &bindData)
	jobResp := post("/api/v1/orders/ord_1/milestones/ms_1/jobs", map[string]any{"bindingId": bindData["binding"].(map[string]any)["id"].(string), "input": "{}"})
	assert(jobResp, http.StatusCreated)
	var jobData map[string]any
	json.Unmarshal(jobResp.Body.Bytes(), &jobData)
	jobID := jobData["job"].(map[string]any)["id"].(string)
	assert(post("/api/v1/jobs/"+jobID+"/complete", map[string]any{"output": "oops"}), http.StatusConflict)
}

func TestV1BusinessFlowE2E_ProviderApplicationLifecycle(t *testing.T) {
	dsn := os.Getenv("ONE_TOK_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ONE_TOK_TEST_DATABASE_URL not set")
	}
	t.Setenv("DATABASE_URL", dsn)
	t.Setenv("NATS_URL", "")
	t.Setenv("IAM_UPSTREAM", "")
	t.Setenv("ONE_TOK_REQUIRE_EXTERNALS", "")

	app, cleanup, err := bootstrap.LoadPlatformApp()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	gw, err := gateway.NewServerWithOptionsE(gateway.Options{App: app})
	if err != nil {
		t.Fatal(err)
	}

	post := func(path string, payload any) *httptest.ResponseRecorder {
		return gwRequest(t, gw, "POST", path, payload)
	}
	get := func(path string) *httptest.ResponseRecorder { return gwRequest(t, gw, "GET", path, nil) }

	// submit provider application
	submitResp := post("/api/v1/provider-applications", map[string]any{
		"orgId":        "org_app_1",
		"name":         "Test Provider",
		"capabilities": []string{"gpu", "io"},
	})
	gwExpect(t, submitResp, http.StatusCreated)
	var payload struct {
		Application struct {
			ID string `json:"id"`
		} `json:"application"`
	}
	if err := json.Unmarshal(submitResp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal submit: %v", err)
	}
	appID := payload.Application.ID
	if appID == "" {
		t.Fatal("provider application id is empty")
	}

	// approve
	reviewResp := post("/api/v1/provider-applications/"+appID+"/review", map[string]any{
		"reviewedBy": "ops_admin",
		"note":       "approve for e2e",
		"approve":    true,
	})
	gwExpect(t, reviewResp, http.StatusOK)

	var reviewedResp map[string]any
	if err := json.Unmarshal(reviewResp.Body.Bytes(), &reviewedResp); err != nil {
		t.Fatalf("unmarshal review: %v", err)
	}
	appObj, ok := reviewedResp["application"].(map[string]any)
	if !ok {
		t.Fatalf("missing application in review response: %v", reviewedResp)
	}
	if appObj["status"].(string) != "approved" {
		t.Fatalf("expected approved status, got %v", appObj["status"])
	}

	approved := get("/api/v1/provider-applications?status=approved")
	gwExpect(t, approved, http.StatusOK)
	if !strings.Contains(approved.Body.String(), appID) {
		t.Fatalf("approved list should contain app id %s: %s", appID, approved.Body.String())
	}
}
