package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path"
	"strings"
	"time"

	"github.com/chenyu/1-tok/internal/carrier"
	"github.com/chenyu/1-tok/internal/core"
	carrierclient "github.com/chenyu/1-tok/internal/integrations/carrier"
	"github.com/chenyu/1-tok/internal/platform"
)

const (
	defaultCarrierWorkspaceRoot     = "/workspace"
	defaultCarrierBackend           = "codex"
	carrierRunCapability            = "run_shell"
	carrierRunTimeoutSec            = 900
	carrierExecutionDispatchTimeout = 20 * time.Minute
)

type carrierAwardExecutionInput struct {
	RFQ     platform.RFQ
	Order   *core.Order
	Binding platform.ProviderCarrierBinding
}

type carrierAwardExecutor interface {
	Execute(context.Context, carrierAwardExecutionInput) error
}

type carrierReportCallbackConfig struct {
	BaseURL        string
	BindingID      string
	JobID          string
	ReportPath     string
	CallbackSecret string
	CallbackKeyID  string
}

type carrierOrderAutoExecutor struct {
	app              *platform.App
	carrier          *carrier.Service
	now              func() time.Time
	clientForBinding func(platform.ProviderCarrierBinding) carrierclient.CodeAgentClient
}

func newCarrierOrderAutoExecutor(app *platform.App, carrierSvc *carrier.Service) *carrierOrderAutoExecutor {
	return &carrierOrderAutoExecutor{
		app:     app,
		carrier: carrierSvc,
		now:     time.Now,
		clientForBinding: func(binding platform.ProviderCarrierBinding) carrierclient.CodeAgentClient {
			return carrierclient.NewClient(binding.CarrierBaseURL, binding.IntegrationToken)
		},
	}
}

func (e *carrierOrderAutoExecutor) Execute(ctx context.Context, input carrierAwardExecutionInput) error {
	if input.Order == nil {
		return fmt.Errorf("order is required")
	}
	milestone := runningMilestone(input.Order)
	if milestone == nil {
		return fmt.Errorf("order %s has no running milestone", input.Order.ID)
	}

	binding, err := e.carrier.Bind(input.Order.ID, milestone.ID, input.Binding.ID, []string{"codeagent"})
	if err != nil {
		if err != carrier.ErrBindingExists {
			return err
		}
		binding, err = e.carrier.GetBinding(input.Order.ID, milestone.ID)
		if err != nil {
			return err
		}
	}

	job, err := e.carrier.CreateJob(binding.ID, milestone.ID, carrierJobInput(input.RFQ, input.Order, milestone))
	if err != nil {
		return err
	}
	if _, err := e.carrier.StartJob(job.ID); err != nil {
		return err
	}

	reportPath := carrierReportPath(input.Binding.WorkspaceRoot, input.Order.ID, milestone.ID)
	stdoutPath := carrierStdoutPath(reportPath)
	stderrPath := carrierStderrPath(reportPath)
	reportDir := path.Dir(reportPath)
	command := buildCarrierRunCommand(reportDir, reportPath, buildCarrierPrompt(input.RFQ, input.Order, milestone), carrierReportCallbackConfig{})
	hostID := strings.TrimSpace(input.Binding.HostID)
	agentID := firstNonEmptyString(strings.TrimSpace(input.Binding.AgentID), "main")
	backend := firstNonEmptyString(strings.TrimSpace(input.Binding.Backend), defaultCarrierBackend)
	workspaceRoot := firstNonEmptyString(strings.TrimSpace(input.Binding.WorkspaceRoot), defaultCarrierWorkspaceRoot)
	client := e.clientForBinding(input.Binding)

	if err := ensureCarrierCodeAgentReady(ctx, client, hostID, agentID, backend, workspaceRoot); err != nil {
		_, _ = e.carrier.FailJob(job.ID, err.Error())
		return err
	}

	runResult, err := client.RunCodeAgent(ctx, carrierclient.CodeAgentRunInput{
		HostID:        hostID,
		AgentID:       agentID,
		Backend:       backend,
		WorkspaceRoot: workspaceRoot,
		Capability:    carrierRunCapability,
		Command:       command,
		CWD:           reportDir,
		TimeoutSec:    carrierRunTimeoutSec,
		StdoutPath:    stdoutPath,
		StderrPath:    stderrPath,
	})
	if err != nil {
		_, _ = e.carrier.FailJob(job.ID, err.Error())
		return err
	}
	if !strings.EqualFold(strings.TrimSpace(runResult.Result.PolicyDecision), "allow") {
		err := fmt.Errorf("carrier policy decision rejected run: ok=%t decision=%s", runResult.Result.OK, runResult.Result.PolicyDecision)
		_, _ = e.carrier.FailJob(job.ID, err.Error())
		return err
	}
	if !runResult.Result.OK {
		err := buildCarrierCommandFailure(stdoutPath, stderrPath)
		_, _ = e.carrier.FailJob(job.ID, err.Error())
		return err
	}

	if _, err := e.carrier.CompleteJob(job.ID, reportPath); err != nil {
		return err
	}

	_, _, err = e.app.SettleMilestone(input.Order.ID, platform.SettleMilestoneInput{
		MilestoneID: milestone.ID,
		Summary:     fmt.Sprintf("Carrier execution completed. Result saved to %s", reportPath),
		Source:      "carrier-auto",
		OccurredAt:  e.now().UTC(),
	})
	return err
}

func ensureCarrierCodeAgentReady(ctx context.Context, client carrierclient.CodeAgentClient, hostID, agentID, backend, workspaceRoot string) error {
	versionInput := carrierclient.CodeAgentVersionInput{
		HostID:  hostID,
		AgentID: agentID,
		Backend: backend,
	}
	version, err := client.GetCodeAgentVersion(ctx, versionInput)
	if err == nil && strings.TrimSpace(version.Value) != "" {
		return nil
	}

	installInput := carrierclient.CodeAgentInstallInput{
		HostID:        hostID,
		AgentID:       agentID,
		Backend:       backend,
		WorkspaceRoot: workspaceRoot,
	}
	if installErr := client.InstallCodeAgent(ctx, installInput); installErr != nil {
		if err != nil {
			return fmt.Errorf("carrier codeagent preflight failed for host=%s agent=%s backend=%s: version=%v install=%w", hostID, agentID, backend, err, installErr)
		}
		return fmt.Errorf("carrier codeagent preflight failed for host=%s agent=%s backend=%s: install=%w", hostID, agentID, backend, installErr)
	}

	version, err = client.GetCodeAgentVersion(ctx, versionInput)
	if err != nil {
		return fmt.Errorf("carrier codeagent version after install failed for host=%s agent=%s backend=%s: %w", hostID, agentID, backend, err)
	}
	if strings.TrimSpace(version.Value) == "" {
		return fmt.Errorf("carrier codeagent version after install is empty for host=%s agent=%s backend=%s", hostID, agentID, backend)
	}
	return nil
}

func runningMilestone(order *core.Order) *core.Milestone {
	if order == nil {
		return nil
	}
	for i := range order.Milestones {
		if order.Milestones[i].State == core.MilestoneStateRunning {
			return &order.Milestones[i]
		}
	}
	for i := range order.Milestones {
		// Fallback to the first milestone that is not settled.
		// This keeps execution resilient when an award is processed before the
		// milestone is explicitly moved to running by other services.
		if order.Milestones[i].State != core.MilestoneStateSettled {
			return &order.Milestones[i]
		}
	}
	return nil

}

func carrierReportPath(workspaceRoot, orderID, milestoneID string) string {
	root := firstNonEmptyString(strings.TrimSpace(workspaceRoot), defaultCarrierWorkspaceRoot)
	return path.Join(root, "1tok", strings.TrimSpace(orderID), strings.TrimSpace(milestoneID), "result.md")
}

func carrierStdoutPath(reportPath string) string {
	return strings.TrimSpace(reportPath) + ".stdout.log"
}

func carrierStderrPath(reportPath string) string {
	return strings.TrimSpace(reportPath) + ".stderr.log"
}

func carrierJobInput(rfq platform.RFQ, order *core.Order, milestone *core.Milestone) string {
	if order == nil || milestone == nil {
		return "carrier auto execution"
	}
	if strings.TrimSpace(rfq.Scope) == "" {
		return fmt.Sprintf("%s :: %s", order.ID, milestone.Title)
	}
	return fmt.Sprintf("%s :: %s :: %s", order.ID, milestone.Title, rfq.Scope)
}

func buildCarrierPrompt(rfq platform.RFQ, order *core.Order, milestone *core.Milestone) string {
	var builder strings.Builder
	builder.WriteString("You are the awarded provider on 1 Tok.\n")
	builder.WriteString("Produce a concise buyer-facing delivery note in markdown.\n")
	builder.WriteString("Sections: Summary, Findings, Recommendation.\n")
	builder.WriteString("Keep it under 500 words.\n")
	builder.WriteString("Do not browse the web or use external network tools.\n")
	builder.WriteString("Work only from the request brief and your general knowledge.\n")
	builder.WriteString("If a detail is uncertain, label it as a directional recommendation instead of a verified fact.\n\n")
	builder.WriteString("Request title: ")
	builder.WriteString(strings.TrimSpace(firstNonEmptyString(rfq.Title, orderTitle(order))))
	builder.WriteString("\n")
	if scope := strings.TrimSpace(rfq.Scope); scope != "" {
		builder.WriteString("Scope: ")
		builder.WriteString(scope)
		builder.WriteString("\n")
	}
	if milestone != nil {
		builder.WriteString("Milestone: ")
		builder.WriteString(strings.TrimSpace(milestone.Title))
		builder.WriteString("\n")
	}
	builder.WriteString("Return only the delivery note markdown.")
	return builder.String()
}

func buildCarrierRunCommand(reportDir, reportPath, prompt string, callbackConfig carrierReportCallbackConfig) string {
	segments := []string{
		"set -e",
		"export HOME=/home/carrier",
		"export CODEX_HOME=/home/carrier/.codex",
		". /home/carrier/.bash_profile >/dev/null 2>&1 || true",
		fmt.Sprintf("mkdir -p %s", shellQuote(reportDir)),
		fmt.Sprintf("cd %s", shellQuote(reportDir)),
		fmt.Sprintf(
			"codex exec --cd %s --skip-git-repo-check --full-auto --output-last-message %s %s",
			shellQuote(reportDir),
			shellQuote(reportPath),
			shellQuote(prompt),
		),
	}
	if callbackConfig.Enabled() {
		segments = append(segments, buildCarrierCallbackCommand(callbackConfig))
	}
	inner := strings.Join(segments, "; ")
	return "bash -lc " + shellQuote(inner)
}

func buildCarrierCallbackCommand(config carrierReportCallbackConfig) string {
	script := strings.TrimSpace(fmt.Sprintf(`
(async () => {
  const fs = require("node:fs");
  const crypto = require("node:crypto");
  const callbackBaseUrl = %s;
  const jobId = %s;
  const bindingId = %s;
  const reportPath = %s;
  const callbackSecret = %s;
  const callbackKeyId = %s;
  const report = fs.readFileSync(reportPath, "utf8").trim();
  const summary = report || ("Carrier execution completed. Result saved to " + reportPath);
  const output = report || reportPath;
  const envelope = {
    eventId: jobId + "-ready",
    sequence: 1,
    eventType: "milestone.ready",
    bindingId,
    carrierExecutionId: jobId,
    createdAt: new Date().toISOString(),
    payload: {
      jobId,
      output,
      summary,
    },
  };
  const body = JSON.stringify(envelope);
  const headers = {
    "Accept": "application/json",
    "Content-Type": "application/json",
    "X-Carrier-Signature": "sha256=" + crypto.createHmac("sha256", callbackSecret).update(body).digest("hex"),
  };
  if (callbackKeyId) headers["X-Carrier-Key-Id"] = callbackKeyId;
  const response = await fetch(callbackBaseUrl.replace(/\/$/, "") + "/api/v1/carrier/callbacks/events", {
    method: "POST",
    headers,
    body,
  });
  if (!response.ok) {
    const message = await response.text();
    throw new Error("carrier callback failed: " + response.status + " " + message);
  }
})().catch((error) => {
  console.error(error && error.stack ? error.stack : String(error));
  process.exit(1);
});
`, mustJSONJS(config.BaseURL), mustJSONJS(config.JobID), mustJSONJS(config.BindingID), mustJSONJS(config.ReportPath), mustJSONJS(config.CallbackSecret), mustJSONJS(config.CallbackKeyID)))
	return "node -e " + shellQuote(script)
}

func resolveCarrierReportCallbackConfig(binding platform.ProviderCarrierBinding, carrierBindingID, jobID, reportPath string) carrierReportCallbackConfig {
	callbackSecret := strings.TrimSpace(binding.CallbackSecret)
	baseURL := strings.TrimSpace(firstNonEmptyString(
		os.Getenv("CARRIER_CALLBACK_BASE_URL"),
		os.Getenv("DEMO_API_BASE_URL"),
		os.Getenv("RELEASE_USDI_E2E_API_BASE_URL"),
	))
	if callbackSecret == "" || baseURL == "" || strings.TrimSpace(carrierBindingID) == "" || strings.TrimSpace(jobID) == "" {
		return carrierReportCallbackConfig{}
	}
	return carrierReportCallbackConfig{
		BaseURL:        strings.TrimRight(baseURL, "/"),
		BindingID:      strings.TrimSpace(carrierBindingID),
		JobID:          strings.TrimSpace(jobID),
		ReportPath:     strings.TrimSpace(reportPath),
		CallbackSecret: callbackSecret,
		CallbackKeyID:  strings.TrimSpace(binding.CallbackKeyID),
	}
}

func (c carrierReportCallbackConfig) Enabled() bool {
	return strings.TrimSpace(c.BaseURL) != "" &&
		strings.TrimSpace(c.BindingID) != "" &&
		strings.TrimSpace(c.JobID) != "" &&
		strings.TrimSpace(c.ReportPath) != "" &&
		strings.TrimSpace(c.CallbackSecret) != ""
}

func buildCarrierCommandFailure(stdoutPath, stderrPath string) error {
	return fmt.Errorf("carrier command failed: stdout=%s stderr=%s", stdoutPath, stderrPath)
}

func orderTitle(order *core.Order) string {
	if order == nil {
		return ""
	}
	if len(order.Milestones) > 0 && strings.TrimSpace(order.Milestones[0].Title) != "" {
		return order.Milestones[0].Title
	}
	return order.ID
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

func mustJSONJS(value string) string {
	body, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return string(body)
}

func (s *Server) dispatchCarrierExecution(rfq platform.RFQ, order *core.Order) {
	if s == nil || s.carrierAwardExecutor == nil || order == nil {
		return
	}
	binding, err := s.app.GetProviderCarrierBinding(order.ProviderOrgID)
	if err != nil || binding.Status != "active" {
		return
	}

	input := carrierAwardExecutionInput{
		RFQ:     rfq,
		Order:   order,
		Binding: binding,
	}
	ctx, cancel := context.WithTimeout(context.Background(), carrierExecutionDispatchTimeout)
	go func() {
		defer cancel()
		if err := s.carrierAwardExecutor.Execute(ctx, input); err != nil {
			orderID := ""
			if input.Order != nil {
				orderID = input.Order.ID
			}
			log.Printf("gateway: carrier auto execution failed for order=%s provider=%s binding=%s: %v", orderID, binding.ProviderOrgID, input.Binding.ID, err)
		}
	}()
}
