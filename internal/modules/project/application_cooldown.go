package project

import "time"

const applicationRejectionCooldown = 72 * time.Hour

// rejectionRetryAfter estimates the start of the cooldown for legacy rejected
// applications that predate reviewed_at. updated_at is refreshed by the legacy
// review mutation; applied_at is a final deterministic fallback for malformed
// historical rows. The estimate is exposed to clients and is never persisted as
// reviewer metadata.
func rejectionRetryAfter(application ProjectApplication) (time.Time, bool) {
	if application.ReviewedAt != nil {
		return application.ReviewedAt.Add(applicationRejectionCooldown), false
	}
	start := application.UpdatedAt
	if start.IsZero() {
		start = application.AppliedAt
	}
	return start.Add(applicationRejectionCooldown), true
}

func rejectionCooldownActive(application ProjectApplication, now time.Time) bool {
	retryAfter, _ := rejectionRetryAfter(application)
	return now.Before(retryAfter)
}
