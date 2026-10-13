package project

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type ProjectDate struct{ time.Time }

func (d *ProjectDate) UnmarshalJSON(data []byte) error {
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return fmt.Errorf("date must use YYYY-MM-DD")
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return fmt.Errorf("date must use YYYY-MM-DD")
	}
	d.Time = parsed
	return nil
}

func (d ProjectDate) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.Format("2006-01-02"))
}

type ProjectRoleWriteInput struct {
	ID                 *uuid.UUID  `json:"id"`
	ContributionRoleID *uuid.UUID  `json:"contribution_role_id"`
	Description        string      `json:"description"`
	Capacity           int         `json:"capacity"`
	SkillIDs           []uuid.UUID `json:"skill_ids"`
	ToolIDs            []uuid.UUID `json:"tool_ids"`
}

func (input *ProjectRoleWriteInput) UnmarshalJSON(data []byte) error {
	if _, err := strictObject(data,
		[]string{"id", "contribution_role_id", "description", "capacity", "skill_ids", "tool_ids"},
		map[string]bool{"id": true, "contribution_role_id": true}); err != nil {
		return err
	}
	type plain ProjectRoleWriteInput
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*input = ProjectRoleWriteInput(decoded)
	return nil
}

type ProjectDefinitionInput struct {
	CreationMode            CreationMode            `json:"creation_mode"`
	InitiatorOrganizationID *uuid.UUID              `json:"initiator_organization_id"`
	Title                   string                  `json:"title"`
	Summary                 string                  `json:"summary"`
	Description             string                  `json:"description"`
	Slug                    string                  `json:"slug"`
	ProjectCategory         string                  `json:"project_category"`
	Visibility              string                  `json:"visibility"`
	OwnerContributionRoleID *uuid.UUID              `json:"owner_contribution_role_id"`
	Origin                  string                  `json:"origin"`
	WhyCollaborative        string                  `json:"why_collaborative"`
	ContributorOutcome      string                  `json:"contributor_outcome"`
	OwnerCommitment         string                  `json:"owner_commitment"`
	LeadExpectations        string                  `json:"lead_expectations"`
	ClientAcknowledgement   bool                    `json:"client_acknowledgement"`
	StartDate               *ProjectDate            `json:"start_date" swaggertype:"string" format:"date" example:"2026-10-08"`
	Deadline                *ProjectDate            `json:"deadline" swaggertype:"string" format:"date" example:"2026-10-08"`
	Availability            string                  `json:"availability"`
	HoursPerWeek            int                     `json:"hours_per_week"`
	CollaborationAgreement  bool                    `json:"collaboration_agreement"`
	Roles                   []ProjectRoleWriteInput `json:"roles"`
}

func (input *ProjectDefinitionInput) UnmarshalJSON(data []byte) error {
	if _, err := strictObject(data,
		[]string{"creation_mode", "initiator_organization_id", "title", "summary", "description", "slug", "project_category", "visibility", "owner_contribution_role_id", "origin", "why_collaborative", "contributor_outcome", "owner_commitment", "lead_expectations", "client_acknowledgement", "start_date", "deadline", "availability", "hours_per_week", "collaboration_agreement", "roles"},
		map[string]bool{"initiator_organization_id": true, "owner_contribution_role_id": true, "start_date": true, "deadline": true}); err != nil {
		return err
	}
	type plain ProjectDefinitionInput
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*input = ProjectDefinitionInput(decoded)
	return nil
}

type CreateProjectDraftRequest struct {
	ClientRequestID uuid.UUID              `json:"client_request_id"`
	Definition      ProjectDefinitionInput `json:"definition"`
}

func (request *CreateProjectDraftRequest) UnmarshalJSON(data []byte) error {
	if _, err := strictObject(data, []string{"client_request_id", "definition"}, nil); err != nil {
		return err
	}
	type plain CreateProjectDraftRequest
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*request = CreateProjectDraftRequest(decoded)
	return nil
}

type UpdateProjectDefinitionRequest struct {
	Version    int64                  `json:"version"`
	Definition ProjectDefinitionInput `json:"definition"`
}

func (request *UpdateProjectDefinitionRequest) UnmarshalJSON(data []byte) error {
	if _, err := strictObject(data, []string{"version", "definition"}, nil); err != nil {
		return err
	}
	type plain UpdateProjectDefinitionRequest
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*request = UpdateProjectDefinitionRequest(decoded)
	return nil
}

type PublishProjectRequest struct {
	Version int64 `json:"version"`
}

func (request *PublishProjectRequest) UnmarshalJSON(data []byte) error {
	if _, err := strictObject(data, []string{"version"}, nil); err != nil {
		return err
	}
	type plain PublishProjectRequest
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*request = PublishProjectRequest(decoded)
	return nil
}

func strictObject(data []byte, required []string, nullable map[string]bool) (map[string]json.RawMessage, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil || fields == nil {
		return nil, fmt.Errorf("request must be a JSON object")
	}
	allowed := make(map[string]struct{}, len(required))
	for _, key := range required {
		allowed[key] = struct{}{}
		value, exists := fields[key]
		if !exists {
			return nil, fmt.Errorf("missing required field %q", key)
		}
		if !nullable[key] && bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return nil, fmt.Errorf("field %q cannot be null", key)
		}
	}
	for key := range fields {
		if _, exists := allowed[key]; !exists {
			return nil, fmt.Errorf("unknown field %q", key)
		}
	}
	return fields, nil
}

type ProjectMasterResponse struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
	Slug string    `json:"slug"`
}

type ProjectRoleResponse struct {
	ID                 uuid.UUID               `json:"id"`
	ContributionRoleID *uuid.UUID              `json:"contribution_role_id"`
	ContributionRole   *ProjectMasterResponse  `json:"contribution_role"`
	Description        string                  `json:"description"`
	Capacity           int                     `json:"capacity"`
	Position           int                     `json:"position"`
	SkillIDs           []uuid.UUID             `json:"skill_ids"`
	Skills             []ProjectMasterResponse `json:"skills"`
	ToolIDs            []uuid.UUID             `json:"tool_ids"`
	Tools              []ProjectMasterResponse `json:"tools"`
	FilledCapacity     int                     `json:"filled_capacity"`
	RemainingCapacity  int                     `json:"remaining_capacity"`
	Status             string                  `json:"status"`
}

type ProjectOwnerRoleResponse struct {
	ContributionRoleID uuid.UUID             `json:"contribution_role_id"`
	ContributionRole   ProjectMasterResponse `json:"contribution_role"`
}

type ProjectPublicProfileResponse struct {
	ID         uuid.UUID `json:"id"`
	Username   string    `json:"username"`
	FullName   *string   `json:"full_name"`
	Headline   *string   `json:"headline"`
	Avatar     *string   `json:"avatar"`
	IsVerified bool      `json:"is_verified"`
}

type ProjectOrganizationResponse struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
	Slug string    `json:"slug"`
}

type ProjectOwnershipResponse struct {
	CreatedByUserID         uuid.UUID                     `json:"created_by_user_id"`
	InitiatorUserID         *uuid.UUID                    `json:"initiator_user_id"`
	InitiatorOrganizationID *uuid.UUID                    `json:"initiator_organization_id"`
	OwnerID                 *uuid.UUID                    `json:"owner_id"`
	OwnerProfile            *ProjectPublicProfileResponse `json:"owner_profile"`
	InitiatorProfile        *ProjectPublicProfileResponse `json:"initiator_profile"`
	InitiatorOrganization   *ProjectOrganizationResponse  `json:"initiator_organization"`
	ShowInitiator           bool                          `json:"show_initiator"`
	CanAcceptContributors   bool                          `json:"can_accept_contributors"`
}

type ViewerApplicationResponse struct {
	ID        uuid.UUID `json:"id"`
	Status    string    `json:"status"`
	AppliedAt time.Time `json:"applied_at"`
}

type ProjectDetailResponse struct {
	ID                      uuid.UUID                  `json:"id"`
	Slug                    string                     `json:"slug"`
	Status                  ProjectStatus              `json:"status"`
	Version                 int64                      `json:"version"`
	CreatedAt               time.Time                  `json:"created_at"`
	UpdatedAt               time.Time                  `json:"updated_at"`
	PublishedAt             *time.Time                 `json:"published_at"`
	CreationMode            CreationMode               `json:"creation_mode"`
	InitiatorOrganizationID *uuid.UUID                 `json:"initiator_organization_id"`
	Title                   string                     `json:"title"`
	Summary                 string                     `json:"summary"`
	Description             string                     `json:"description"`
	ProjectCategory         string                     `json:"project_category"`
	Visibility              string                     `json:"visibility"`
	Origin                  string                     `json:"origin"`
	WhyCollaborative        string                     `json:"why_collaborative"`
	ContributorOutcome      string                     `json:"contributor_outcome"`
	OwnerCommitment         string                     `json:"owner_commitment"`
	LeadExpectations        string                     `json:"lead_expectations"`
	ClientAcknowledgement   bool                       `json:"client_acknowledgement"`
	StartDate               *ProjectDate               `json:"start_date" swaggertype:"string" format:"date" example:"2026-10-08"`
	Deadline                *ProjectDate               `json:"deadline" swaggertype:"string" format:"date" example:"2026-10-08"`
	Availability            string                     `json:"availability"`
	HoursPerWeek            int                        `json:"hours_per_week"`
	CollaborationAgreement  bool                       `json:"collaboration_agreement"`
	Ownership               ProjectOwnershipResponse   `json:"ownership"`
	OwnerRole               *ProjectOwnerRoleResponse  `json:"owner_role"`
	Roles                   []ProjectRoleResponse      `json:"roles"`
	ViewerApplication       *ViewerApplicationResponse `json:"viewer_application"`
	created                 bool
}

type ApplyProjectRequest struct {
	ProjectRoleID         uuid.UUID `json:"project_role_id"`
	Motivation            string    `json:"motivation"`
	ExpectedContribution  *string   `json:"expected_contribution"`
	PortfolioLinks        []string  `json:"portfolio_links"`
	Availability          string    `json:"availability"`
	EstimatedHoursPerWeek *int      `json:"estimated_hours_per_week"`
}

type ProjectApplicationResponse struct {
	ID        uuid.UUID `json:"id"`
	Status    string    `json:"status"`
	AppliedAt time.Time `json:"applied_at"`
}

type ProjectSummaryResponse struct {
	ID                      uuid.UUID                     `json:"id"`
	Slug                    string                        `json:"slug"`
	Title                   string                        `json:"title"`
	Summary                 string                        `json:"summary"`
	ProjectCategory         string                        `json:"project_category"`
	Origin                  string                        `json:"origin"`
	Status                  ProjectStatus                 `json:"status"`
	OwnerID                 *uuid.UUID                    `json:"owner_id"`
	OwnerProfile            *ProjectPublicProfileResponse `json:"owner_profile"`
	InitiatorOrganizationID *uuid.UUID                    `json:"initiator_organization_id"`
	InitiatorOrganization   *ProjectOrganizationResponse  `json:"initiator_organization"`
	TotalCapacity           int                           `json:"total_capacity"`
	FilledCapacity          int                           `json:"filled_capacity"`
	HoursPerWeek            int                           `json:"hours_per_week"`
	Deadline                *ProjectDate                  `json:"deadline" swaggertype:"string" format:"date" example:"2026-10-08"`
	Skills                  []ProjectMasterResponse       `json:"skills"`
	Tools                   []ProjectMasterResponse       `json:"tools"`
	CreatedAt               time.Time                     `json:"created_at"`
}

type MyProjectRoleSummaryResponse struct {
	Name     string `json:"name"`
	Capacity int    `json:"capacity"`
}

type MyProjectSummaryResponse struct {
	ProjectSummaryResponse
	MyScope                 string                         `json:"my_scope"`
	Roles                   []MyProjectRoleSummaryResponse `json:"roles"`
	PendingApplicationCount *int                           `json:"pending_application_count,omitempty"`
}

type ProjectListFilter struct {
	Page   int
	Limit  int
	Search string
	Scope  string
}

type ProjectPaginationMeta struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}
