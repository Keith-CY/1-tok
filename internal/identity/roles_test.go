package identity

import "testing"

func TestDefaultMembershipRoleForOrganizationKind(t *testing.T) {
	tests := []struct {
		kind     string
		wantRole string
	}{
		{"buyer", "org_owner"},
		{"provider", "org_owner"},
		{"ops", "ops_reviewer"},
		{"  ops  ", "ops_reviewer"},
		{"unknown", "org_owner"},
		{"", "org_owner"},
	}
	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			got := DefaultMembershipRoleForOrganizationKind(tt.kind)
			if got != tt.wantRole {
				t.Errorf("DefaultMembershipRoleForOrganizationKind(%q) = %q, want %q", tt.kind, got, tt.wantRole)
			}
		})
	}
}
