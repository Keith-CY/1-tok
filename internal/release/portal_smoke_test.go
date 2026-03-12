package release

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/chenyu/1-tok/internal/serviceauth"
)

type portalUser struct {
	email        string
	password     string
	kind         string
	sessionToken string
}

func TestRunPortalSmokeExercisesPortalForms(t *testing.T) {
	usersByEmail := map[string]portalUser{}
	usersByToken := map[string]portalUser{}
	var rfqID string
	var rfqTitle string
	var bidID string
	var orderID string
	var disputeID string
	var settledOrderID string
	var executionToken string

	iam := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/signup":
			var payload struct {
				Email            string `json:"email"`
				Password         string `json:"password"`
				OrganizationKind string `json:"organizationKind"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode signup payload: %v", err)
			}
			user := portalUser{
				email:        payload.Email,
				password:     payload.Password,
				kind:         payload.OrganizationKind,
				sessionToken: "sess_" + payload.OrganizationKind,
			}
			usersByEmail[payload.Email] = user
			usersByToken[user.sessionToken] = user
			_ = json.NewEncoder(w).Encode(map[string]any{
				"session": map[string]any{
					"token": user.sessionToken,
				},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/v1/sessions":
			var payload struct {
				Email    string `json:"email"`
				Password string `json:"password"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode session payload: %v", err)
			}
			user, ok := usersByEmail[payload.Email]
			if !ok || user.password != payload.Password {
				t.Fatalf("unexpected login payload: %+v", payload)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"session": map[string]any{
					"token": user.sessionToken,
				},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/me":
			user := userFromBearer(t, r, usersByToken)
			role := map[string]string{
				"buyer":    "procurement",
				"provider": "sales",
				"ops":      "ops_reviewer",
			}[user.kind]
			_ = json.NewEncoder(w).Encode(map[string]any{
				"user": map[string]any{
					"id":    "usr_" + user.kind,
					"email": user.email,
					"name":  strings.Title(user.kind) + " User",
				},
				"memberships": []map[string]any{
					{
						"role": role,
						"organization": map[string]any{
							"id":   user.kind + "_org_1",
							"name": strings.Title(user.kind) + " Org",
							"kind": user.kind,
						},
					},
				},
			})
		default:
			t.Fatalf("unexpected iam request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer iam.Close()

	web := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/login":
			_, _ = w.Write([]byte("login"))
		case r.Method == http.MethodPost && r.URL.Path == "/auth/login":
			if err := r.ParseForm(); err != nil {
				t.Fatalf("parse login form: %v", err)
			}
			user, ok := usersByEmail[r.Form.Get("email")]
			if !ok || user.password != r.Form.Get("password") {
				t.Fatalf("unexpected auth form: %+v", r.Form)
			}
			http.SetCookie(w, &http.Cookie{Name: "one_tok_session", Value: user.sessionToken, Path: "/"})
			http.Redirect(w, r, r.Form.Get("next"), http.StatusSeeOther)
		case r.Method == http.MethodGet && r.URL.Path == "/buyer":
			assertPortalCookie(t, r, "sess_buyer")
			_, _ = w.Write([]byte("Buyer portal / orchestration budget"))
		case r.Method == http.MethodGet && r.URL.Path == "/provider":
			assertPortalCookie(t, r, "sess_provider")
			_, _ = w.Write([]byte("Provider portal / delivery + payouts"))
		case r.Method == http.MethodGet && r.URL.Path == "/ops":
			assertPortalCookie(t, r, "sess_ops")
			_, _ = w.Write([]byte("Ops portal / treasury + governance"))
		case r.Method == http.MethodPost && r.URL.Path == "/buyer/rfqs":
			assertPortalCookie(t, r, "sess_buyer")
			if err := r.ParseForm(); err != nil {
				t.Fatalf("parse buyer rfq form: %v", err)
			}
			rfqID = "rfq_web_1"
			rfqTitle = r.Form.Get("title")
			http.Redirect(w, r, "/buyer", http.StatusSeeOther)
		case r.Method == http.MethodPost && r.URL.Path == "/provider/rfqs/rfq_web_1/bids":
			assertPortalCookie(t, r, "sess_provider")
			bidID = "bid_web_1"
			http.Redirect(w, r, "/provider", http.StatusSeeOther)
		case r.Method == http.MethodPost && r.URL.Path == "/buyer/rfqs/rfq_web_1/award":
			assertPortalCookie(t, r, "sess_buyer")
			orderID = "ord_web_1"
			http.Redirect(w, r, "/buyer", http.StatusSeeOther)
		case r.Method == http.MethodPost && r.URL.Path == "/ops/credits/decision":
			assertPortalCookie(t, r, "sess_ops")
			http.Redirect(w, r, "/ops?creditApproved=true&recommendedLimitCents=885000&creditReason=Stable+history", http.StatusSeeOther)
		case r.Method == http.MethodPost && r.URL.Path == "/ops/disputes/disp_web_1/resolve":
			assertPortalCookie(t, r, "sess_ops")
			http.Redirect(w, r, "/ops?resolvedDisputeId=disp_web_1&disputeStatus=resolved", http.StatusSeeOther)
		default:
			t.Fatalf("unexpected web request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer web.Close()

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/rfqs":
			status := "open"
			orderRef := ""
			awardedBidID := ""
			awardedProviderOrgID := ""
			if orderID != "" {
				status = "awarded"
				orderRef = orderID
				awardedBidID = bidID
				awardedProviderOrgID = "provider_org_1"
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"rfqs": []map[string]any{
					{
						"id":                   rfqID,
						"buyerOrgId":           "buyer_org_1",
						"title":                rfqTitle,
						"category":             "agent-ops",
						"scope":                "Investigate and stabilize",
						"budgetCents":          4200,
						"status":               status,
						"awardedBidId":         awardedBidID,
						"awardedProviderOrgId": awardedProviderOrgID,
						"orderId":              orderRef,
						"responseDeadlineAt":   "2026-03-15T12:00:00Z",
						"createdAt":            "2026-03-12T00:00:00Z",
						"updatedAt":            "2026-03-12T00:00:00Z",
					},
				},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/rfqs/rfq_web_1/bids":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"bids": []map[string]any{
					{
						"id":            bidID,
						"rfqId":         rfqID,
						"providerOrgId": "provider_org_1",
						"message":       "Provider ready",
						"quoteCents":    3900,
						"status":        "open",
						"milestones": []map[string]any{
							{"id": "ms_1", "title": "Execution", "basePriceCents": 3900, "budgetCents": 4200},
						},
						"createdAt": "2026-03-12T00:00:00Z",
						"updatedAt": "2026-03-12T00:00:00Z",
					},
				},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/orders/ord_web_1/disputes":
			if settledOrderID != orderID {
				t.Fatalf("dispute opened before settlement: settled=%q order=%q", settledOrderID, orderID)
			}
			disputeID = "disp_web_1"
			_ = json.NewEncoder(w).Encode(map[string]any{
				"order":         map[string]any{"id": orderID},
				"refundEntry":   map[string]any{"kind": "buyer_reimbursement"},
				"recoveryEntry": map[string]any{"kind": "provider_recovery"},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/disputes":
			userFromBearer(t, r, usersByToken)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"disputes": []map[string]any{
					{
						"id":          disputeID,
						"orderId":     orderID,
						"milestoneId": "ms_1",
						"reason":      "Output incomplete",
						"refundCents": 900,
						"status":      "open",
						"createdAt":   "2026-03-12T00:00:00Z",
					},
				},
			})
		default:
			t.Fatalf("unexpected api request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer api.Close()

	execution := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/carrier/events":
			executionToken = r.Header.Get(serviceauth.HeaderName)
			var payload struct {
				OrderID     string `json:"orderId"`
				MilestoneID string `json:"milestoneId"`
				EventType   string `json:"eventType"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode execution payload: %v", err)
			}
			if payload.EventType != "milestone_ready" || payload.MilestoneID != "ms_1" {
				t.Fatalf("unexpected execution payload %+v", payload)
			}
			settledOrderID = payload.OrderID
			_ = json.NewEncoder(w).Encode(map[string]any{
				"accepted":        true,
				"continueAllowed": true,
			})
		default:
			t.Fatalf("unexpected execution request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer execution.Close()

	summary, err := RunPortalSmoke(context.Background(), PortalConfig{
		WebBaseURL:          web.URL,
		APIBaseURL:          api.URL,
		IAMBaseURL:          iam.URL,
		ExecutionBaseURL:    execution.URL,
		ExecutionEventToken: "portal-event-token",
	})
	if err != nil {
		t.Fatalf("run portal smoke: %v", err)
	}

	if summary.RFQID != "rfq_web_1" || summary.BidID != "bid_web_1" || summary.OrderID != "ord_web_1" {
		t.Fatalf("unexpected marketplace summary: %+v", summary)
	}
	if summary.DisputeID != "disp_web_1" || summary.ResolvedDisputeID != "disp_web_1" {
		t.Fatalf("unexpected dispute summary: %+v", summary)
	}
	if !summary.CreditApproved {
		t.Fatalf("expected approved credit summary, got %+v", summary)
	}
	if executionToken != "portal-event-token" {
		t.Fatalf("expected execution event token, got %q", executionToken)
	}
}

func userFromBearer(t *testing.T, r *http.Request, users map[string]portalUser) portalUser {
	t.Helper()
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	user, ok := users[token]
	if !ok {
		t.Fatalf("unexpected bearer token %q", token)
	}
	return user
}

func assertPortalCookie(t *testing.T, r *http.Request, expected string) {
	t.Helper()
	cookie, err := r.Cookie("one_tok_session")
	if err != nil {
		t.Fatalf("read portal cookie: %v", err)
	}
	if cookie.Value != expected {
		t.Fatalf("expected cookie %q, got %q", expected, cookie.Value)
	}
}
