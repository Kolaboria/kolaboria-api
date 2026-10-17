package workspace

import (
	"github.com/google/uuid"
	"github.com/syahrulfahmi/kolaboria-api/internal/modules/project"
)

type ProjectInfoResponse struct {
	Project                 ProjectInfoProjectResponse      `json:"project"`
	Positions               []PositionInfoResponse          `json:"positions"`
	PositionCapacitySummary PositionCapacitySummaryResponse `json:"position_capacity_summary"`
	Viewer                  ProjectInfoViewerResponse       `json:"viewer"`
	Closure                 ProjectClosureResponse          `json:"closure"`
}

type ProjectInfoProjectResponse struct {
	ID                      uuid.UUID                    `json:"id"`
	Slug                    string                       `json:"slug"`
	Version                 int64                        `json:"version"`
	Status                  project.ProjectStatus        `json:"status"`
	Title                   string                       `json:"title"`
	Summary                 string                       `json:"summary"`
	Description             string                       `json:"description"`
	ProjectCategory         string                       `json:"project_category"`
	CreationMode            project.CreationMode         `json:"creation_mode"`
	InitiatorOrganizationID *uuid.UUID                   `json:"initiator_organization_id"`
	StartDate               *project.ProjectDate         `json:"start_date" swaggertype:"string" format:"date" example:"2026-10-20"`
	Deadline                *project.ProjectDate         `json:"deadline" swaggertype:"string" format:"date" example:"2026-12-01"`
	Ownership               ProjectInfoOwnershipResponse `json:"ownership"`
}

type ProjectInfoOwnershipResponse struct {
	OwnerID                 *uuid.UUID                            `json:"owner_id"`
	OwnerProfile            *project.ProjectPublicProfileResponse `json:"owner_profile"`
	InitiatorOrganizationID *uuid.UUID                            `json:"initiator_organization_id"`
	InitiatorOrganization   *project.ProjectOrganizationResponse  `json:"initiator_organization"`
	InitiatorProfile        *project.ProjectPublicProfileResponse `json:"initiator_profile"`
	ShowInitiator           bool                                  `json:"show_initiator"`
}

type PositionInfoResponse struct {
	ID                     uuid.UUID `json:"id"`
	Name                   string    `json:"name"`
	Description            string    `json:"description"`
	Capacity               int       `json:"capacity"`
	ActiveContributorCount int       `json:"active_contributor_count"`
	RemainingCapacity      int       `json:"remaining_capacity"`
	RecruitmentStatus      string    `json:"recruitment_status"`
	ReadinessState         string    `json:"readiness_state"`
}

type PositionCapacitySummaryResponse struct {
	AllPositionsFilled       bool `json:"all_positions_filled"`
	UnderfilledPositionCount int  `json:"underfilled_position_count"`
	TotalRemainingCapacity   int  `json:"total_remaining_capacity"`
}

type ProjectInfoViewerResponse struct {
	Scope        string                          `json:"scope"`
	MembershipID *uuid.UUID                      `json:"membership_id"`
	Capabilities ProjectInfoCapabilitiesResponse `json:"capabilities"`
}

type ProjectInfoCapabilitiesResponse struct {
	CanViewInfo        bool `json:"can_view_info"`
	CanStartProject    bool `json:"can_start_project"`
	CanManageWorkspace bool `json:"can_manage_workspace"`
	CanCreateTask      bool `json:"can_create_task"`
	CanCompleteProject bool `json:"can_complete_project"`
}
