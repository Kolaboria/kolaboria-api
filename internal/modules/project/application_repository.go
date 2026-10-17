package project

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/syahrulfahmi/kolaboria-api/internal/infrastructure/workspaceactivity"
)

type ApplicationRepository interface {
	FindActorForRead(context.Context, uuid.UUID) (*ProjectActor, error)
	IsActiveProjectMember(context.Context, uuid.UUID, uuid.UUID) (bool, error)
	FindOwnerProject(context.Context, uuid.UUID, uuid.UUID) (*Project, error)
	CountProjectRoleIDs(context.Context, uuid.UUID, []uuid.UUID) (int64, error)
	GetOwnerSummary(context.Context, uuid.UUID, uuid.UUID) (*ApplicationManagementReadModel, error)
	ListOwnerApplications(context.Context, uuid.UUID, uuid.UUID, ApplicationListFilter) ([]ApplicationReadModel, int64, error)
	GetVisibleApplication(context.Context, uuid.UUID, uuid.UUID) (*ApplicationReadModel, error)
	GetMySummary(context.Context, uuid.UUID) (*ApplicationStatsReadModel, error)
	ListApplicantApplications(context.Context, uuid.UUID, ApplicationListFilter) ([]ApplicationReadModel, int64, error)
	FindApplicationIdentity(context.Context, uuid.UUID) (*ApplicationIdentity, error)
	WithApplicationTransaction(context.Context, func(ApplicationTx) error) error
}

type ApplicationTx interface {
	AppendActivity(context.Context, workspaceactivity.Input) error
	FindActor(context.Context, uuid.UUID) (*ProjectActor, error)
	FindActorForRead(context.Context, uuid.UUID) (*ProjectActor, error)
	LockProjectState(context.Context, uuid.UUID) (*Project, error)
	LockRoleForApplication(context.Context, uuid.UUID, uuid.UUID) (*ProjectRole, error)
	LockApplication(context.Context, uuid.UUID) (*ProjectApplication, error)
	CountRoleUsage(context.Context, uuid.UUID) (RoleUsage, error)
	IsActiveProjectMember(context.Context, uuid.UUID, uuid.UUID) (bool, error)
	IsAuthorizedOrganizationInitiator(context.Context, uuid.UUID, uuid.UUID) (bool, error)
	CreateContributor(context.Context, *ProjectMember) error
	UpdateApplication(context.Context, *ProjectApplication, ApplicationStatus) error
	BumpProjectVersion(context.Context, uuid.UUID, time.Time) error
}
