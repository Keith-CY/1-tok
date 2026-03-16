package carrier

import "strings"

// canonicalEventNames maps legacy snake_case event names to canonical dot-separated names.
var canonicalEventNames = map[string]string{
	"execution_accepted":       "execution.accepted",
	"execution_started":        "execution.started",
	"execution_heartbeat":      "execution.heartbeat",
	"execution_completed":      "execution.completed",
	"execution_failed":         "execution.failed",
	"execution_paused":         "execution.paused",
	"execution_resumed":        "execution.resumed",
	"execution_pause_requested": "execution.pause_requested",
	"milestone_started":        "milestone.started",
	"milestone_ready":          "milestone.ready",
	"usage_reported":           "usage.reported",
	"artifact_ready":           "artifact.ready",
	"budget_low":               "budget.low",
}

// NormalizeEventName converts legacy snake_case event names to canonical dot-separated form.
// Already canonical names pass through unchanged.
func NormalizeEventName(name string) string {
	if canonical, ok := canonicalEventNames[name]; ok {
		return canonical
	}
	// If it already contains dots, assume canonical
	if strings.Contains(name, ".") {
		return name
	}
	// Unknown snake_case → convert underscores to dots
	return strings.ReplaceAll(name, "_", ".")
}
