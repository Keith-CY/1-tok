package core

import "errors"

var (
	// ErrOrderNotFound is returned when an order lookup yields no result.
	ErrOrderNotFound = errors.New("order not found")
	// ErrMilestoneNotFound is returned when a milestone lookup yields no result.
	ErrMilestoneNotFound = errors.New("milestone not found")
)
