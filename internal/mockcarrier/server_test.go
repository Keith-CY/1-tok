package mockcarrier

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	carrierclient "github.com/chenyu/1-tok/internal/integrations/carrier"
)

func TestServerServesCodeAgentFlow(t *testing.T) {
	server := httptest.NewServer(NewServerWithOptions(Options{APIToken: "carrier-token"}))
	defer server.Close()

	client := carrierclient.NewClient(server.URL, "carrier-token")

	health, err := client.GetCodeAgentHealth(context.Background(), carrierclient.CodeAgentHealthInput{
		HostID:        "host_1",
		AgentID:       "agent_1",
		Backend:       "codex",
		WorkspaceRoot: "/workspace",
	})
	if err != nil {
		t.Fatalf("get codeagent health: %v", err)
	}
	if !health.Healthy || health.Backend != "codex" || health.WorkspaceRoot != "/workspace" {
		t.Fatalf("unexpected health result: %+v", health)
	}

	version, err := client.GetCodeAgentVersion(context.Background(), carrierclient.CodeAgentVersionInput{
		HostID:  "host_1",
		AgentID: "agent_1",
		Backend: "opencode",
	})
	if err != nil {
		t.Fatalf("get codeagent version: %v", err)
	}
	if version.Backend != "opencode" || version.Value == "" {
		t.Fatalf("unexpected version result: %+v", version)
	}

	run, err := client.RunCodeAgent(context.Background(), carrierclient.CodeAgentRunInput{
		HostID:        "host_1",
		AgentID:       "agent_1",
		Backend:       "codex",
		WorkspaceRoot: "/workspace",
		Capability:    "run_shell",
		Command:       "pwd",
	})
	if err != nil {
		t.Fatalf("run codeagent: %v", err)
	}
	if run.Backend != "codex" || !run.Result.OK || run.Result.PolicyDecision != "allow" {
		t.Fatalf("unexpected run result: %+v", run)
	}
}

func TestServerRejectsMissingBearerTokenWhenConfigured(t *testing.T) {
	server := httptest.NewServer(NewServerWithOptions(Options{APIToken: "carrier-token"}))
	defer server.Close()

	req, err := http.NewRequest(http.MethodGet, server.URL+"/api/v1/remote/hosts/host_1/instances/agent_1/codeagent/health", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", res.StatusCode)
	}
}
