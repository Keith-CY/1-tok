package identity

import "strings"

func DefaultMembershipRoleForOrganizationKind(kind string) string {
	switch strings.TrimSpace(kind) {
	case "ops":
		return "ops_reviewer"
	default:
		return "org_owner"
	}
}
