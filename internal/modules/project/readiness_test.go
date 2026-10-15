package project

import "testing"

func TestEvaluatePositionCapacity(t *testing.T) {
	for _, test := range []struct {
		name      string
		capacity  int
		active    int
		remaining int
		readiness string
		wantError bool
	}{
		{name: "full", capacity: 3, active: 3, remaining: 0, readiness: PositionReadinessReady},
		{name: "over capacity", capacity: 3, active: 4, remaining: 0, readiness: PositionReadinessReady},
		{name: "partial", capacity: 3, active: 2, remaining: 1, readiness: PositionReadinessToFill},
		{name: "empty", capacity: 3, active: 0, remaining: 3, readiness: PositionReadinessToFill},
		{name: "invalid capacity", capacity: 0, active: 0, wantError: true},
		{name: "invalid active count", capacity: 1, active: -1, wantError: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			remaining, readiness, err := EvaluatePositionCapacity(test.capacity, test.active)
			if (err != nil) != test.wantError {
				t.Fatalf("expected error=%v, got %v", test.wantError, err)
			}
			if test.wantError {
				return
			}
			if remaining != test.remaining || readiness != test.readiness {
				t.Fatalf("expected remaining=%d readiness=%s, got remaining=%d readiness=%s", test.remaining, test.readiness, remaining, readiness)
			}
		})
	}
}
