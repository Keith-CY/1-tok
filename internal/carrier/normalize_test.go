package carrier

import "testing"

func TestNormalizeEventName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"usage_reported", "usage.reported"},
		{"milestone_ready", "milestone.ready"},
		{"execution_accepted", "execution.accepted"},
		{"execution_completed", "execution.completed"},
		{"budget_low", "budget.low"},
		{"artifact_ready", "artifact.ready"},
		{"execution_pause_requested", "execution.pause_requested"},
		// Already canonical
		{"usage.reported", "usage.reported"},
		{"execution.completed", "execution.completed"},
		// Unknown snake_case → generic conversion
		{"custom_event_name", "custom.event.name"},
	}
	for _, tc := range tests {
		got := NormalizeEventName(tc.input)
		if got != tc.want {
			t.Errorf("NormalizeEventName(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}
