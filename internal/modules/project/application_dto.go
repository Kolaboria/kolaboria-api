package project

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type ApplicationProfileResponse struct {
	ID       uuid.UUID `json:"id"`
	Username string    `json:"username"`
	FullName *string   `json:"full_name"`
	Headline *string   `json:"headline"`
	Avatar   *string   `json:"avatar"`
}

type ApplicationRoleResponse struct {
	ID                 uuid.UUID               `json:"id"`
	ContributionRoleID *uuid.UUID              `json:"contribution_role_id"`
	ContributionRole   *ProjectMasterResponse  `json:"contribution_role"`
	Description        string                  `json:"description"`
	Capacity           int                     `json:"capacity"`
	FilledCapacity     int                     `json:"filled_capacity"`
	RemainingCapacity  int                     `json:"remaining_capacity"`
	Status             string                  `json:"status"`
	IsArchived         bool                    `json:"is_archived"`
	Skills             []ProjectMasterResponse `json:"skills"`
	Tools              []ProjectMasterResponse `json:"tools"`
}

type ApplicationProjectResponse struct {
	ID                    uuid.UUID                    `json:"id"`
	Title                 string                       `json:"title"`
	Slug                  string                       `json:"slug"`
	Status                ProjectStatus                `json:"status"`
	Summary               string                       `json:"summary"`
	Origin                string                       `json:"origin"`
	CreationMode          CreationMode                 `json:"creation_mode"`
	OwnerProfile          *ApplicationProfileResponse  `json:"owner_profile"`
	InitiatorOrganization *ProjectOrganizationResponse `json:"initiator_organization"`
}

type ApplicationPermissionsResponse struct {
	CanAccept         bool    `json:"can_accept"`
	CanReject         bool    `json:"can_reject"`
	AcceptBlockReason *string `json:"accept_block_reason"`
	RejectBlockReason *string `json:"reject_block_reason"`
}

type ApplicantListItemResponse struct {
	ID                    uuid.UUID                   `json:"id"`
	ProjectID             uuid.UUID                   `json:"project_id"`
	ApplicantUserID       uuid.UUID                   `json:"applicant_user_id"`
	ProjectRoleID         uuid.UUID                   `json:"project_role_id"`
	Status                ApplicationStatus           `json:"status"`
	AppliedAt             time.Time                   `json:"applied_at"`
	Availability          string                      `json:"availability"`
	EstimatedHoursPerWeek *int                        `json:"estimated_hours_per_week"`
	Motivation            string                      `json:"motivation"`
	ExpectedContribution  *string                     `json:"expected_contribution"`
	ApplicantProfile      *ApplicationProfileResponse `json:"applicant_profile"`
	ProjectRole           ApplicationRoleResponse     `json:"project_role"`
}

type ApplicationDetailResponse struct {
	ApplicantListItemResponse
	PortfolioLinks      []string                       `json:"portfolio_links"`
	ReviewerNote        *string                        `json:"reviewer_note"`
	ReviewedBy          *uuid.UUID                     `json:"reviewed_by"`
	ReviewedAt          *time.Time                     `json:"reviewed_at"`
	WithdrawnAt         *time.Time                     `json:"withdrawn_at"`
	RetryAfter          *time.Time                     `json:"retry_after"`
	RetryAfterEstimated bool                           `json:"retry_after_estimated"`
	ReapplyBlockReason  *string                        `json:"reapply_block_reason"`
	Project             ApplicationProjectResponse     `json:"project"`
	Permissions         ApplicationPermissionsResponse `json:"permissions"`
}

type MyApplicationListItemResponse struct {
	ApplicantListItemResponse
	ReviewerNote        *string                    `json:"reviewer_note"`
	ReviewedAt          *time.Time                 `json:"reviewed_at"`
	WithdrawnAt         *time.Time                 `json:"withdrawn_at"`
	RetryAfter          *time.Time                 `json:"retry_after"`
	RetryAfterEstimated bool                       `json:"retry_after_estimated"`
	ReapplyBlockReason  *string                    `json:"reapply_block_reason"`
	Project             ApplicationProjectResponse `json:"project"`
}

type ApplicantStatsResponse struct {
	Total     int64 `json:"total"`
	Pending   int64 `json:"pending"`
	Accepted  int64 `json:"accepted"`
	Rejected  int64 `json:"rejected"`
	Withdrawn int64 `json:"withdrawn"`
}

type ApplicantCapacityResponse struct {
	TotalCapacity     int `json:"total_capacity"`
	FilledCapacity    int `json:"filled_capacity"`
	RemainingCapacity int `json:"remaining_capacity"`
}

type ApplicantRoleSummaryResponse struct {
	ID                uuid.UUID `json:"id"`
	Name              string    `json:"name"`
	Capacity          int       `json:"capacity"`
	FilledCapacity    int       `json:"filled_capacity"`
	RemainingCapacity int       `json:"remaining_capacity"`
	Status            string    `json:"status"`
	IsArchived        bool      `json:"is_archived"`
}

type ApplicantSummaryPermissionsResponse struct {
	CanReviewApplications bool    `json:"can_review_applications"`
	ReviewBlockReason     *string `json:"review_block_reason"`
}

type ApplicantSummaryResponse struct {
	Project     ApplicationProjectResponse          `json:"project"`
	Capacity    ApplicantCapacityResponse           `json:"capacity"`
	Stats       ApplicantStatsResponse              `json:"stats"`
	Roles       []ApplicantRoleSummaryResponse      `json:"roles"`
	Permissions ApplicantSummaryPermissionsResponse `json:"permissions"`
}

type MyApplicationSummaryResponse struct {
	Stats ApplicantStatsResponse `json:"stats"`
}

type ReviewApplicationRequest struct {
	Status       ApplicationStatus `json:"status"`
	ReviewerNote *string           `json:"reviewer_note"`
}

func (request *ReviewApplicationRequest) UnmarshalJSON(data []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil || fields == nil {
		return fmt.Errorf("request must be a JSON object")
	}
	if _, exists := fields["status"]; !exists {
		return fmt.Errorf("status is required")
	}
	for key := range fields {
		if key != "status" && key != "reviewer_note" {
			return fmt.Errorf("unknown field %q", key)
		}
	}
	type plain ReviewApplicationRequest
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*request = ReviewApplicationRequest(decoded)
	return nil
}
