package project

import "fmt"

const (
	PositionReadinessReady  = "ready"
	PositionReadinessToFill = "to_fill"
)

// EvaluatePositionCapacity derives the readiness label and remaining slots
// from persisted capacity and active contributor membership counts.
func EvaluatePositionCapacity(capacity, activeContributorCount int) (remaining int, readiness string, err error) {
	if capacity < 1 || activeContributorCount < 0 {
		return 0, "", fmt.Errorf("invalid project position capacity: capacity=%d active_contributors=%d", capacity, activeContributorCount)
	}
	remaining = capacity - activeContributorCount
	if remaining <= 0 {
		return 0, PositionReadinessReady, nil
	}
	return remaining, PositionReadinessToFill, nil
}
