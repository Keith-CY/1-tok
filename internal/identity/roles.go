package identity

import "strings"

// DefaultMembershipRoleForOrganizationKind returns the membership role
// assigned to the founding user of an organization. Every supported
// organization kind has an explicit mapping so that adding a new kind
// without defining its default role is a conscious decision.
func DefaultMembershipRoleForOrganizationKind(kind string) string {
	switch strings.TrimSpace(kind) {
	case "buyer":
		return "org_owner"
	case "provider":
		return "org_owner"
	case "ops":
		return "ops_reviewer"
	default:
		return "org_owner"
	}
}
