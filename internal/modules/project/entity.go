package project

import (
	"time"

	"github.com/google/uuid"
)

type ProjectStatus string

const (
	ProjectStatusDraft         ProjectStatus = "draft"
	ProjectStatusOpen          ProjectStatus = "open"
	ProjectStatusAwaitingOwner ProjectStatus = "awaiting_owner"
	ProjectStatusInProgress    ProjectStatus = "in_progress"
	ProjectStatusCompleted     ProjectStatus = "completed"
	ProjectStatusArchived      ProjectStatus = "archived"
)

type CreationMode string

const (
	CreationModePersonal              CreationMode = "personal"
	CreationModeOrganizationInitiated CreationMode = "organization_initiated"
)

type Project struct {
	ID                      uuid.UUID     `gorm:"column:id;type:uuid;primaryKey"`
	Slug                    string        `gorm:"column:slug"`
	CreationMode            CreationMode  `gorm:"column:creation_mode"`
	CreatedByUserID         uuid.UUID     `gorm:"column:created_by_user_id;type:uuid"`
	InitiatorUserID         *uuid.UUID    `gorm:"column:initiator_user_id;type:uuid"`
	InitiatorOrganizationID *uuid.UUID    `gorm:"column:initiator_organization_id;type:uuid"`
	OwnerID                 *uuid.UUID    `gorm:"column:owner_id;type:uuid"`
	Title                   string        `gorm:"column:title"`
	Summary                 string        `gorm:"column:summary"`
	Description             string        `gorm:"column:description"`
	ProjectCategory         string        `gorm:"column:project_category"`
	Visibility              string        `gorm:"column:visibility"`
	Origin                  string        `gorm:"column:origin"`
	WhyCollaborative        string        `gorm:"column:why_collaborative"`
	ContributorOutcome      string        `gorm:"column:contributor_outcome"`
	OwnerCommitment         string        `gorm:"column:owner_commitment"`
	LeadExpectations        string        `gorm:"column:lead_expectations"`
	ClientAcknowledgement   bool          `gorm:"column:client_acknowledgement"`
	StartDate               *time.Time    `gorm:"column:start_date;type:date"`
	Deadline                *time.Time    `gorm:"column:deadline;type:date"`
	Availability            string        `gorm:"column:availability"`
	HoursPerWeek            int           `gorm:"column:hours_per_week"`
	CollaborationAgreement  bool          `gorm:"column:collaboration_agreement"`
	AgreementAt             *time.Time    `gorm:"column:agreement_at"`
	Status                  ProjectStatus `gorm:"column:status"`
	Version                 int64         `gorm:"column:version"`
	ClientRequestID         uuid.UUID     `gorm:"column:client_request_id;type:uuid"`
	CreateRequestHash       string        `gorm:"column:create_request_hash"`
	PublishedAt             *time.Time    `gorm:"column:published_at"`
	CreatedAt               time.Time     `gorm:"column:created_at"`
	UpdatedAt               time.Time     `gorm:"column:updated_at"`
}

func (Project) TableName() string { return "projects" }

type ProjectRole struct {
	ID                 uuid.UUID  `gorm:"column:id;type:uuid;primaryKey"`
	ProjectID          uuid.UUID  `gorm:"column:project_id;type:uuid;not null"`
	ContributionRoleID *uuid.UUID `gorm:"column:contribution_role_id;type:uuid"`
	Description        string     `gorm:"column:description"`
	Capacity           int        `gorm:"column:capacity"`
	Position           int        `gorm:"column:position"`
	ArchivedAt         *time.Time `gorm:"column:archived_at"`
	CreatedAt          time.Time  `gorm:"column:created_at"`
	UpdatedAt          time.Time  `gorm:"column:updated_at"`
}

func (ProjectRole) TableName() string { return "project_roles" }

type ProjectRoleSkill struct {
	ProjectRoleID uuid.UUID `gorm:"column:project_role_id;type:uuid;primaryKey"`
	SkillID       uuid.UUID `gorm:"column:skill_id;type:uuid;primaryKey"`
	CreatedAt     time.Time `gorm:"column:created_at"`
}

func (ProjectRoleSkill) TableName() string { return "project_role_skills" }

type ProjectRoleTool struct {
	ProjectRoleID uuid.UUID `gorm:"column:project_role_id;type:uuid;primaryKey"`
	ToolID        uuid.UUID `gorm:"column:tool_id;type:uuid;primaryKey"`
	CreatedAt     time.Time `gorm:"column:created_at"`
}

func (ProjectRoleTool) TableName() string { return "project_role_tools" }

type ProjectMember struct {
	ID                 uuid.UUID  `gorm:"column:id;type:uuid;primaryKey"`
	ProjectID          uuid.UUID  `gorm:"column:project_id;type:uuid;not null"`
	UserID             uuid.UUID  `gorm:"column:user_id;type:uuid;not null"`
	MembershipRole     string     `gorm:"column:membership_role"`
	Status             string     `gorm:"column:status"`
	ProjectRoleID      *uuid.UUID `gorm:"column:project_role_id;type:uuid"`
	ContributionRoleID *uuid.UUID `gorm:"column:contribution_role_id;type:uuid"`
	JoinedAt           time.Time  `gorm:"column:joined_at"`
	LeftAt             *time.Time `gorm:"column:left_at"`
	CreatedAt          time.Time  `gorm:"column:created_at"`
	UpdatedAt          time.Time  `gorm:"column:updated_at"`
}

func (ProjectMember) TableName() string { return "project_members" }

type ProjectActor struct {
	ID              uuid.UUID  `gorm:"column:id"`
	IsActive        bool       `gorm:"column:is_active"`
	DeletedAt       *time.Time `gorm:"column:deleted_at"`
	EmailVerifiedAt *time.Time `gorm:"column:email_verified_at"`
	AccountType     *string    `gorm:"column:account_type"`
	SystemRole      string     `gorm:"column:system_role"`
}

type ProjectOrganization struct {
	ID   uuid.UUID `gorm:"column:id"`
	Name string    `gorm:"column:name"`
	Slug string    `gorm:"column:slug"`
}

type MasterReference struct {
	ID   uuid.UUID `gorm:"column:id"`
	Name string    `gorm:"column:name"`
	Slug string    `gorm:"column:slug"`
}

type ProjectRoleAggregate struct {
	ProjectRole
	ContributionRole *MasterReference
	Skills           []MasterReference
	Tools            []MasterReference
	FilledCapacity   int
}

type ProjectAggregate struct {
	Project
	Roles                 []ProjectRoleAggregate
	OwnerMember           *ProjectMember
	OwnerRole             *MasterReference
	OwnerProfile          *ProjectPublicProfileResponse
	InitiatorProfile      *ProjectPublicProfileResponse
	InitiatorOrganization *ProjectOrganizationResponse
}

type RoleUsage struct {
	ActiveContributors  int
	PendingApplications int
}
