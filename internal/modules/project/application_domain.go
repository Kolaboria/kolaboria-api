package project

import (
	"time"

	"github.com/google/uuid"
)

type ApplicationStatus string

const (
	ApplicationStatusPending   ApplicationStatus = "pending"
	ApplicationStatusAccepted  ApplicationStatus = "accepted"
	ApplicationStatusRejected  ApplicationStatus = "rejected"
	ApplicationStatusWithdrawn ApplicationStatus = "withdrawn"
)

type ApplicationSort string

const (
	ApplicationSortNewest ApplicationSort = "newest"
	ApplicationSortOldest ApplicationSort = "oldest"
)

type ApplicationListFilter struct {
	Page           int
	Limit          int
	Search         string
	Statuses       []ApplicationStatus
	ProjectRoleIDs []uuid.UUID
	Origins        []string
	Sort           ApplicationSort
}

type ApplicationStatsReadModel struct {
	Total     int64
	Pending   int64
	Accepted  int64
	Rejected  int64
	Withdrawn int64
}

type ApplicationReadModel struct {
	Application         ProjectApplication
	Profile             *ApplicationProfileResponse
	Project             ApplicationProjectResponse
	Role                ApplicationRoleReadModel
	RetryAfter          *time.Time
	RetryAfterEstimated bool
	BlockReason         *string
}

type ApplicationRoleReadModel struct {
	Role             ProjectRole
	ContributionRole *MasterReference
	Skills           []MasterReference
	Tools            []MasterReference
	ActiveMembers    int
}

type ApplicationManagementReadModel struct {
	Project Project
	Owner   *ApplicationProfileResponse
	Org     *ProjectOrganizationResponse
	Stats   ApplicationStatsReadModel
	Roles   []ApplicationRoleReadModel
}

type ApplicationIdentity struct {
	ApplicationID uuid.UUID
	ProjectID     uuid.UUID
	ProjectRoleID uuid.UUID
	ApplicantID   uuid.UUID
}

type ApplicationReviewContext struct {
	Actor       *ProjectActor
	Project     *Project
	Application *ProjectApplication
	Role        *ProjectRole
	Applicant   *ProjectActor
	Usage       RoleUsage
	IsMember    bool
}
