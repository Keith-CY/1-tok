package gateway

import (
	"context"
	"fmt"
	"log"
	"path"
	"strings"
	"time"

	"github.com/chenyu/1-tok/internal/carrier"
	"github.com/chenyu/1-tok/internal/core"
	carrierclient "github.com/chenyu/1-tok/internal/integrations/carrier"
	"github.com/chenyu/1-tok/internal/platform"
)

type carrierAwardExecutionInput struct {
	RFQ     platform.RFQ
	Order   *core.Order
	Binding platform.ProviderCarrierBinding
}

type carrierAwardExecutor interface {
	Execute(context.Context, carrierAwardExecutionInput) error
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
	command := buildCarrierRunCommand(reportPath, buildCarrierPrompt(input.RFQ, input.Order, milestone))

	runResult, err := e.clientForBinding(input.Binding).RunCodeAgent(ctx, carrierclient.CodeAgentRunInput{
		HostID:        strings.TrimSpace(input.Binding.HostID),
		AgentID:       firstNonEmptyString(strings.TrimSpace(input.Binding.AgentID), "main"),
		Backend:       firstNonEmptyString(strings.TrimSpace(input.Binding.Backend), "codex"),
		WorkspaceRoot: firstNonEmptyString(strings.TrimSpace(input.Binding.WorkspaceRoot), "/workspace"),
		Capability:    "run_shell",
		Command:       command,
		TimeoutSec:    900,
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

func runningMilestone(order *core.Order) *core.Milestone {
	if order == nil {
		return nil
	}
	for i := range order.Milestones {
		if order.Milestones[i].State == core.MilestoneStateRunning {
			return &order.Milestones[i]
		}
	}
	if len(order.Milestones) == 0 {
		return nil
	}
	return &order.Milestones[0]
}

func carrierReportPath(workspaceRoot, orderID, milestoneID string) string {
	root := firstNonEmptyString(strings.TrimSpace(workspaceRoot), "/workspace")
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

func buildCarrierRunCommand(reportPath, prompt string) string {
	reportDir := path.Dir(reportPath)
	inner := fmt.Sprintf(
		"export HOME=/home/carrier; export CODEX_HOME=/home/carrier/.codex; . /home/carrier/.bash_profile >/dev/null 2>&1 || true; mkdir -p %s && codex exec --skip-git-repo-check --output-last-message %s %s",
		shellQuote(reportDir),
		shellQuote(reportPath),
		shellQuote(prompt),
	)
	return "bash -lc " + shellQuote(inner)
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
	go func() {
		if err := s.carrierAwardExecutor.Execute(context.Background(), input); err != nil {
			log.Printf("gateway: carrier auto execution failed for order %s: %v", order.ID, err)
		}
	}()
}
