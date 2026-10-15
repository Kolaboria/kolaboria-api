package workspace

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/syahrulfahmi/kolaboria-api/internal/modules/project"
	sharedErrors "github.com/syahrulfahmi/kolaboria-api/internal/shared/errors"
)

type infoRepositoryFake struct {
	project      *project.ProjectAggregate
	activeMember bool
	canInitiate  bool
	findErr      error
}

func (repository infoRepositoryFake) LoadProjectInfo(context.Context, string, uuid.UUID) (*project.ProjectAggregate, bool, bool, error) {
	return repository.project, repository.activeMember, repository.canInitiate, repository.findErr
}

func TestGetProjectInfoUsesPersistedCapacityAndKeepsInitiatorSeparate(t *testing.T) {
	actorID := uuid.New()
	ownerID := actorID
	organizationID := uuid.New()
	roleID := uuid.New()
	repository := infoRepositoryFake{
		project: &project.ProjectAggregate{Project: project.Project{
			ID: uuid.New(), Slug: "platform", Version: 12, Status: project.ProjectStatusOpen,
			Title: "Platform", Summary: "Ringkasan", Description: "Deskripsi",
			CreationMode:            project.CreationModeOrganizationInitiated,
			InitiatorOrganizationID: &organizationID, OwnerID: &ownerID,
		}, InitiatorOrganization: &project.ProjectOrganizationResponse{ID: organizationID, Name: "Komunitas", Slug: "komunitas"}, Roles: []project.ProjectRoleAggregate{{
			ProjectRole:      project.ProjectRole{ID: roleID, Capacity: 3},
			ContributionRole: &project.MasterReference{ID: uuid.New(), Name: "Web Developer", Slug: "web-developer"},
			FilledCapacity:   2,
		}}},
	}

	result, err := NewService(repository).GetProjectInfo(context.Background(), actorID, "platform")
	if err != nil {
		t.Fatalf("get project info: %v", err)
	}
	if result.Positions[0].ActiveContributorCount != 2 || result.Positions[0].RemainingCapacity != 1 || result.Positions[0].ReadinessState != project.PositionReadinessToFill {
		t.Fatalf("expected readiness derived from server occupancy, got %#v", result.Positions[0])
	}
	if result.PositionCapacitySummary.TotalRemainingCapacity != 1 || result.PositionCapacitySummary.UnderfilledPositionCount != 1 {
		t.Fatalf("unexpected capacity summary: %#v", result.PositionCapacitySummary)
	}
	if result.Project.Ownership.OwnerID == nil || *result.Project.Ownership.OwnerID != ownerID || result.Project.Ownership.InitiatorOrganization == nil || result.Project.Ownership.InitiatorOrganization.ID != organizationID {
		t.Fatal("owner and initiator organization must remain separate facts")
	}
	if !result.Viewer.Capabilities.CanStartProject {
		t.Fatal("project owner should be allowed to start an open project")
	}
}

func TestGetProjectInfoAllowsActiveMemberWithoutManagementCapability(t *testing.T) {
	actorID := uuid.New()
	ownerID := uuid.New()
	repository := infoRepositoryFake{
		project:      &project.ProjectAggregate{Project: project.Project{ID: uuid.New(), Slug: "platform", Status: project.ProjectStatusOpen, OwnerID: &ownerID}},
		activeMember: true,
	}
	result, err := NewService(repository).GetProjectInfo(context.Background(), actorID, "platform")
	if err != nil {
		t.Fatalf("active project member should read info: %v", err)
	}
	if result.Viewer.Capabilities.CanManageWorkspace || result.Viewer.Capabilities.CanStartProject {
		t.Fatalf("member must not receive full management capabilities: %#v", result.Viewer.Capabilities)
	}
}

func TestGetProjectInfoDoesNotGrantAccessFromPublicVisibility(t *testing.T) {
	actorID := uuid.New()
	repository := infoRepositoryFake{project: &project.ProjectAggregate{Project: project.Project{
		ID: uuid.New(), Slug: "public", Visibility: "public", Status: project.ProjectStatusOpen,
	}}}
	_, err := NewService(repository).GetProjectInfo(context.Background(), actorID, "public")
	if !errors.Is(err, sharedErrors.ErrNotFound) {
		t.Fatalf("public visibility must not grant workspace access, got %v", err)
	}
}

func TestGetProjectInfoRejectsInvalidPositionCapacity(t *testing.T) {
	actorID := uuid.New()
	repository := infoRepositoryFake{
		project: &project.ProjectAggregate{Project: project.Project{ID: uuid.New(), Slug: "platform", OwnerID: &actorID}, Roles: []project.ProjectRoleAggregate{{
			ProjectRole: project.ProjectRole{ID: uuid.New(), Capacity: 0},
		}}},
	}
	if _, err := NewService(repository).GetProjectInfo(context.Background(), actorID, "platform"); err == nil {
		t.Fatal("invalid capacity must not be presented as ready or filled")
	}
}
