package carrier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type CodeAgentClient interface {
	GetCodeAgentHealth(ctx context.Context, input CodeAgentHealthInput) (CodeAgentHealthResult, error)
	GetCodeAgentVersion(ctx context.Context, input CodeAgentVersionInput) (CodeAgentVersionResult, error)
	RunCodeAgent(ctx context.Context, input CodeAgentRunInput) (CodeAgentRunResult, error)
}

type CodeAgentHealthInput struct {
	HostID        string
	AgentID       string
	Backend       string
	WorkspaceRoot string
}

type CodeAgentHealthResult struct {
	Backend       string `json:"backend"`
	WorkspaceRoot string `json:"workspaceRoot"`
	Healthy       bool   `json:"healthy"`
}

type CodeAgentVersionInput struct {
	HostID  string
	AgentID string
	Backend string
}

type CodeAgentVersionResult struct {
	Backend string `json:"backend"`
	Value   string `json:"value"`
}

type CodeAgentRunInput struct {
	HostID          string `json:"hostId"`
	AgentID         string `json:"agentId"`
	Backend         string `json:"backend"`
	WorkspaceRoot   string `json:"workspaceRoot"`
	Capability      string `json:"capability"`
	Path            string `json:"path,omitempty"`
	Content         string `json:"content,omitempty"`
	WriteMode       string `json:"writeMode,omitempty"`
	Command         string `json:"command,omitempty"`
	CWD             string `json:"cwd,omitempty"`
	TimeoutSec      int    `json:"timeoutSec,omitempty"`
	StdoutPath      string `json:"stdoutPath,omitempty"`
	StderrPath      string `json:"stderrPath,omitempty"`
	AppendOutput    bool   `json:"appendOutput,omitempty"`
	ResumeSessionID string `json:"resumeSessionId,omitempty"`
}

type CodeAgentRunOutput struct {
	OK              bool    `json:"ok"`
	PolicyDecision  string  `json:"policy_decision"`
	CostEstimateUSD float64 `json:"cost_estimate_usd,omitempty"`
}

type CodeAgentRunResult struct {
	Backend string             `json:"backend"`
	Result  CodeAgentRunOutput `json:"result"`
}

type Client struct {
	baseURL    string
	apiToken   string
	httpClient *http.Client
}

const defaultCodeAgentGatewayTimeout = 2 * time.Minute

func NewClient(baseURL, apiToken string) *Client {
	trimmed := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if trimmed == "" {
		trimmed = "http://127.0.0.1:8787"
	}
	return &Client{
		baseURL:    trimmed,
		apiToken:   strings.TrimSpace(apiToken),
		httpClient: &http.Client{Timeout: defaultCodeAgentGatewayTimeout},
	}
}

func NewClientFromEnv() CodeAgentClient {
	return NewClient(os.Getenv("CARRIER_GATEWAY_URL"), os.Getenv("CARRIER_GATEWAY_API_TOKEN"))
}

func (c *Client) GetCodeAgentHealth(ctx context.Context, input CodeAgentHealthInput) (CodeAgentHealthResult, error) {
	query := url.Values{}
	if strings.TrimSpace(input.Backend) != "" {
		query.Set("backend", input.Backend)
	}
	if strings.TrimSpace(input.WorkspaceRoot) != "" {
		query.Set("workspaceRoot", input.WorkspaceRoot)
	}

	var response struct {
		Health CodeAgentHealthResult `json:"health"`
	}
	if err := c.doJSON(ctx, http.MethodGet, codeAgentPath(input.HostID, input.AgentID, "health"), query, nil, &response); err != nil {
		return CodeAgentHealthResult{}, err
	}
	return response.Health, nil
}

func (c *Client) GetCodeAgentVersion(ctx context.Context, input CodeAgentVersionInput) (CodeAgentVersionResult, error) {
	query := url.Values{}
	if strings.TrimSpace(input.Backend) != "" {
		query.Set("backend", input.Backend)
	}

	var response struct {
		Version CodeAgentVersionResult `json:"version"`
	}
	if err := c.doJSON(ctx, http.MethodGet, codeAgentPath(input.HostID, input.AgentID, "version"), query, nil, &response); err != nil {
		return CodeAgentVersionResult{}, err
	}
	return response.Version, nil
}

func (c *Client) RunCodeAgent(ctx context.Context, input CodeAgentRunInput) (CodeAgentRunResult, error) {
	body := map[string]any{
		"backend":         input.Backend,
		"workspaceRoot":   input.WorkspaceRoot,
		"capability":      input.Capability,
		"path":            input.Path,
		"content":         input.Content,
		"writeMode":       input.WriteMode,
		"command":         input.Command,
		"cwd":             input.CWD,
		"timeoutSec":      input.TimeoutSec,
		"stdoutPath":      input.StdoutPath,
		"stderrPath":      input.StderrPath,
		"appendOutput":    input.AppendOutput,
		"resumeSessionId": input.ResumeSessionID,
	}

	var response struct {
		Run CodeAgentRunResult `json:"run"`
	}
	if err := c.doJSON(ctx, http.MethodPost, codeAgentPath(input.HostID, input.AgentID, "run"), nil, body, &response); err != nil {
		return CodeAgentRunResult{}, err
	}
	return response.Run, nil
}

func (c *Client) doJSON(ctx context.Context, method, path string, query url.Values, body any, target any) error {
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(payload)
	}

	endpoint := c.baseURL + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.apiToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiToken)
	}

	res, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(res.Body, 10<<20)) // 10MB max
	if err != nil {
		return err
	}
	if res.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("carrier gateway returned %d: %s", res.StatusCode, strings.TrimSpace(string(responseBody)))
	}

	return json.Unmarshal(responseBody, target)
}

func codeAgentPath(hostID, agentID, action string) string {
	return fmt.Sprintf(
		"/api/v1/remote/hosts/%s/instances/%s/codeagent/%s",
		url.PathEscape(strings.TrimSpace(hostID)),
		url.PathEscape(strings.TrimSpace(agentID)),
		action,
	)
}
