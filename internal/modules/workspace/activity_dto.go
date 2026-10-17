package workspace

import (
	"time"

	"github.com/google/uuid"
)

type ActivityActorResponse struct {
	UserID       uuid.UUID  `json:"user_id"`
	MembershipID *uuid.UUID `json:"membership_id"`
	DisplayName  string     `json:"display_name"`
	Avatar       *string    `json:"avatar"`
}

type ActivitySubjectResponse struct {
	Type         string    `json:"type"`
	ID           uuid.UUID `json:"id"`
	Label        string    `json:"label"`
	Availability string    `json:"availability"`
}

type ActivityChangesResponse struct {
	FromStatus string `json:"from_status,omitempty"`
	ToStatus   string `json:"to_status,omitempty"`
	SelfReview *bool  `json:"self_review,omitempty"`
}

type ActivityResponse struct {
	ID          uuid.UUID                `json:"id"`
	EventType   string                   `json:"event_type"`
	OccurredAt  time.Time                `json:"occurred_at"`
	Actor       ActivityActorResponse    `json:"actor"`
	Subject     ActivitySubjectResponse  `json:"subject"`
	Description string                   `json:"description"`
	Changes     *ActivityChangesResponse `json:"changes,omitempty"`
}
