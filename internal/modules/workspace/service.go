package workspace

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/syahrulfahmi/kolaboria-api/internal/modules/project"
	sharedErrors "github.com/syahrulfahmi/kolaboria-api/internal/shared/errors"
	"gorm.io/gorm"
)

type Repository interface {
	LoadProjectInfo(context.Context, string, uuid.UUID) (*project.ProjectAggregate, bool, bool, error)
}

type Service struct{ repository Repository }

func NewService(repository Repository) *Service { return &Service{repository: repository} }

func (service *Service) GetProjectInfo(ctx context.Context, actorID uuid.UUID, slug string) (*ProjectInfoResponse, error) {
	if actorID == uuid.Nil || slug == "" {
		return nil, sharedErrors.ErrBadRequest
	}
	aggregate, activeMember, canInitiate, err := service.repository.LoadProjectInfo(ctx, slug, actorID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, sharedErrors.ErrNotFound
		}
		return nil, fmt.Errorf("load workspace project by slug: %w", err)
	}
	if aggregate == nil {
		return nil, sharedErrors.ErrNotFound
	}

	canManage, scope := service.canManage(actorID, aggregate, canInitiate)
	if !canManage && !activeMember {
		return nil, sharedErrors.ErrNotFound
	}
	if !canManage {
		scope = "member"
	}

	if err := validateProjectInfo(aggregate); err != nil {
		return nil, err
	}
	return mapProjectInfo(aggregate, scope, canManage), nil
}

func (service *Service) canManage(actorID uuid.UUID, aggregate *project.ProjectAggregate, canInitiate bool) (bool, string) {
	if aggregate.OwnerID != nil && *aggregate.OwnerID == actorID {
		return true, "owner"
	}
	if aggregate.OwnerID != nil || aggregate.CreationMode != project.CreationModeOrganizationInitiated ||
		aggregate.InitiatorOrganizationID == nil || aggregate.CreatedByUserID != actorID || !canInitiate {
		return false, ""
	}
	return true, "organization_initiator"
}

func mapProjectInfo(aggregate *project.ProjectAggregate, scope string, canManage bool) *ProjectInfoResponse {
	positions := make([]PositionInfoResponse, 0, len(aggregate.Roles))
	underfilled := 0
	totalRemaining := 0
	for _, role := range aggregate.Roles {
		if role.ArchivedAt != nil {
			continue
		}
		remaining, readiness, err := project.EvaluatePositionCapacity(role.Capacity, role.FilledCapacity)
		if err != nil {
			// Invalid persisted capacity is a data integrity failure. The mapper's
			// caller validates with validateProjectInfo before serializing.
			continue
		}
		name := "Kontributor"
		if role.ContributionRole != nil && role.ContributionRole.Name != "" {
			name = role.ContributionRole.Name
		}
		recruitmentStatus := "open"
		if remaining == 0 {
			recruitmentStatus = "filled"
		}
		positions = append(positions, PositionInfoResponse{
			ID: role.ID, Name: name, Description: role.Description, Capacity: role.Capacity,
			ActiveContributorCount: role.FilledCapacity, RemainingCapacity: remaining,
			RecruitmentStatus: recruitmentStatus, ReadinessState: readiness,
		})
		if remaining > 0 {
			underfilled++
			totalRemaining += remaining
		}
	}
	projectInfo := ProjectInfoProjectResponse{
		ID: aggregate.ID, Slug: aggregate.Slug, Version: aggregate.Version, Status: aggregate.Status,
		Title: aggregate.Title, Summary: aggregate.Summary, Description: aggregate.Description,
		ProjectCategory: aggregate.ProjectCategory, CreationMode: aggregate.CreationMode,
		InitiatorOrganizationID: cloneUUID(aggregate.InitiatorOrganizationID),
		StartDate:               dateFromTime(aggregate.StartDate), Deadline: dateFromTime(aggregate.Deadline),
		Ownership: ProjectInfoOwnershipResponse{
			OwnerID: cloneUUID(aggregate.OwnerID), OwnerProfile: aggregate.OwnerProfile,
			InitiatorOrganizationID: cloneUUID(aggregate.InitiatorOrganizationID),
			InitiatorOrganization:   aggregate.InitiatorOrganization, InitiatorProfile: aggregate.InitiatorProfile,
			ShowInitiator: aggregate.InitiatorOrganizationID != nil,
		},
	}
	return &ProjectInfoResponse{
		Project: projectInfo, Positions: positions,
		PositionCapacitySummary: PositionCapacitySummaryResponse{
			AllPositionsFilled:       len(positions) > 0 && underfilled == 0,
			UnderfilledPositionCount: underfilled, TotalRemainingCapacity: totalRemaining,
		},
		Viewer: ProjectInfoViewerResponse{Scope: scope, Capabilities: ProjectInfoCapabilitiesResponse{
			CanViewInfo: true, CanStartProject: canManage && aggregate.Status == project.ProjectStatusOpen,
			CanManageWorkspace: canManage,
		}},
	}
}

func validateProjectInfo(aggregate *project.ProjectAggregate) error {
	for _, role := range aggregate.Roles {
		if role.ArchivedAt != nil {
			continue
		}
		if _, _, err := project.EvaluatePositionCapacity(role.Capacity, role.FilledCapacity); err != nil {
			return fmt.Errorf("validate project position %s: %w", role.ID, err)
		}
	}
	return nil
}

func cloneUUID(value *uuid.UUID) *uuid.UUID {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func dateFromTime(value *time.Time) *project.ProjectDate {
	if value == nil {
		return nil
	}
	return &project.ProjectDate{Time: *value}
}
