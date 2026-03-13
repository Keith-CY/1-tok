package platform

import "errors"

var (
	// ErrRFQNotFound is returned when an RFQ lookup yields no result.
	ErrRFQNotFound = errors.New("rfq not found")
	// ErrBidNotFound is returned when a bid lookup yields no result.
	ErrBidNotFound = errors.New("bid not found")
	// ErrDisputeNotFound is returned when a dispute lookup yields no result.
	ErrDisputeNotFound = errors.New("dispute not found")
	// ErrMembershipRequired is returned when a user has no qualifying membership.
	ErrMembershipRequired = errors.New("membership is required")
	// ErrOrgMismatch is returned when the requested org doesn't match the actor's membership.
	ErrOrgMismatch = errors.New("organization mismatch")
)
