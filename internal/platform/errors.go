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

	// Validation errors
	ErrMissingRequiredFields = errors.New("missing required fields")
	ErrDeadlineRequired      = errors.New("response deadline is required")
	ErrMilestonesRequired    = errors.New("milestones are required")

	// RFQ/Bid state errors
	ErrRFQNotOpenForBids  = errors.New("rfq is not open for bids")
	ErrRFQNotOpenForAward = errors.New("rfq is not open for award")
	ErrBidIDRequired      = errors.New("bid id is required")
	ErrBidNotBelongToRFQ  = errors.New("bid does not belong to rfq")

	// Provider errors
	ErrProviderSuspended = errors.New("provider carrier binding is suspended")

	// Bid errors
	ErrBidExceedsBudget = errors.New("bid milestone totals exceed RFQ budget")

	// Rating errors
	ErrInvalidScore    = errors.New("score must be between 1 and 5")
	ErrOrderNotCompleted = errors.New("only completed orders can be rated")
	ErrOrderAlreadyRated = errors.New("order already rated")
	ErrOrderNotRated     = errors.New("order not rated")
)
