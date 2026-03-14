package carrier

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientGetsCodeAgentHealth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer gateway-token" {
			t.Fatalf("expected bearer token, got %q", got)
		}
		if r.URL.Path != "/api/v1/remote/hosts/host_1/instances/agent_1/codeagent/health" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if r.URL.Query().Get("backend") != "codex" {
			t.Fatalf("expected backend query codex, got %q", r.URL.Query().Get("backend"))
		}
		if r.URL.Query().Get("workspaceRoot") != "/workspace" {
			t.Fatalf("expected workspaceRoot query, got %q", r.URL.Query().Get("workspaceRoot"))
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"health": map[string]any{
				"backend":       "codex",
				"workspaceRoot": "/workspace",
				"healthy":       true,
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "gateway-token")
	result, err := client.GetCodeAgentHealth(context.Background(), CodeAgentHealthInput{
		HostID:        "host_1",
		AgentID:       "agent_1",
		Backend:       "codex",
		WorkspaceRoot: "/workspace",
	})
	if err != nil {
		t.Fatalf("get codeagent health: %v", err)
	}
	if !result.Healthy || result.Backend != "codex" || result.WorkspaceRoot != "/workspace" {
		t.Fatalf("unexpected health result: %+v", result)
	}
}

func TestClientRunsCodeAgent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/remote/hosts/host_1/instances/agent_1/codeagent/run" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}

		payload, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		var body struct {
			Backend       string `json:"backend"`
			WorkspaceRoot string `json:"workspaceRoot"`
			Capability    string `json:"capability"`
			Command       string `json:"command"`
		}
		if err := json.Unmarshal(payload, &body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body.Backend != "codex" || body.WorkspaceRoot != "/workspace" || body.Capability != "run_shell" || body.Command != "ls -la" {
			t.Fatalf("unexpected run body: %+v", body)
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"run": map[string]any{
				"backend": "codex",
				"result": map[string]any{
					"ok":                true,
					"policy_decision":   "allow",
					"cost_estimate_usd": 0.42,
				},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "")
	result, err := client.RunCodeAgent(context.Background(), CodeAgentRunInput{
		HostID:        "host_1",
		AgentID:       "agent_1",
		Backend:       "codex",
		WorkspaceRoot: "/workspace",
		Capability:    "run_shell",
		Command:       "ls -la",
	})
	if err != nil {
		t.Fatalf("run codeagent: %v", err)
	}
	if !result.Result.OK || result.Result.PolicyDecision != "allow" {
		t.Fatalf("unexpected run result: %+v", result)
	}
}

func TestClientGetsCodeAgentVersion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/remote/hosts/host_1/instances/agent_1/codeagent/version" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if r.URL.Query().Get("backend") != "opencode" {
			t.Fatalf("expected backend query opencode, got %q", r.URL.Query().Get("backend"))
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"version": map[string]any{
				"backend": "opencode",
				"value":   "1.2.3",
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "")
	result, err := client.GetCodeAgentVersion(context.Background(), CodeAgentVersionInput{
		HostID:  "host_1",
		AgentID: "agent_1",
		Backend: "opencode",
	})
	if err != nil {
		t.Fatalf("get codeagent version: %v", err)
	}
	if result.Value != "1.2.3" || result.Backend != "opencode" {
		t.Fatalf("unexpected version result: %+v", result)
	}
}

func TestClientPropagatesCarrierErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"message":"blocked"}}`, http.StatusBadGateway)
	}))
	defer server.Close()

	client := NewClient(server.URL, "")
	_, err := client.RunCodeAgent(context.Background(), CodeAgentRunInput{
		HostID:     "host_1",
		AgentID:    "agent_1",
		Capability: "run_shell",
		Command:    "ls -la",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("502")) {
		t.Fatalf("expected status code in error, got %v", err)
	}
}

func TestGetCodeAgentHealth_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"internal"}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "token")
	_, err := c.GetCodeAgentHealth(context.Background(), CodeAgentHealthInput{
		HostID: "h", AgentID: "a",
	})
	if err == nil {
		t.Error("expected error for 500 response")
	}
}

func TestGetCodeAgentVersion_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "token")
	_, err := c.GetCodeAgentVersion(context.Background(), CodeAgentVersionInput{
		HostID: "h", AgentID: "a",
	})
	if err == nil {
		t.Error("expected error for 503 response")
	}
}

func TestNewClientFromEnv_Empty(t *testing.T) {
	t.Setenv("CARRIER_GATEWAY_URL", "")
	t.Setenv("CARRIER_GATEWAY_API_TOKEN", "")

	c := NewClientFromEnv()
	_, err := c.GetCodeAgentHealth(context.Background(), CodeAgentHealthInput{HostID: "h", AgentID: "a"})
	if err == nil {
		t.Error("expected error with empty URL")
	}
}

func TestNewClientFromEnv_Configured(t *testing.T) {
	t.Setenv("CARRIER_GATEWAY_URL", "http://carrier:8090")
	t.Setenv("CARRIER_GATEWAY_API_TOKEN", "test-token")

	c := NewClientFromEnv()
	if c == nil {
		t.Error("expected non-nil client")
	}
}
