package project

import (
	"context"
	"errors"
	"sort"
	"strings"

	"github.com/google/uuid"
	sharedErrors "github.com/syahrulfahmi/kolaboria-api/internal/shared/errors"
	"gorm.io/gorm"
)

func (s *ProjectService) GetByID(ctx context.Context, actorID *uuid.UUID, projectID uuid.UUID) (*ProjectDetailResponse, error) {
	aggregate, err := s.repository.FindByID(ctx, projectID)
	if err != nil {
		return nil, projectLookupError(err)
	}
	return s.projectDetailForActor(ctx, actorID, aggregate)
}

func (s *ProjectService) GetBySlug(ctx context.Context, actorID *uuid.UUID, slug string) (*ProjectDetailResponse, error) {
	aggregate, err := s.repository.FindBySlug(ctx, slug)
	if err != nil {
		return nil, projectLookupError(err)
	}
	return s.projectDetailForActor(ctx, actorID, aggregate)
}

func (s *ProjectService) ListPublic(ctx context.Context, filter ProjectListFilter) ([]ProjectSummaryResponse, ProjectPaginationMeta, error) {
	filter, err := validateListFilter(filter)
	if err != nil {
		return nil, ProjectPaginationMeta{}, err
	}
	projects, total, err := s.repository.ListPublic(ctx, filter)
	if err != nil {
		return nil, ProjectPaginationMeta{}, err
	}
	items := make([]ProjectSummaryResponse, 0, len(projects))
	for i := range projects {
		items = append(items, mapProjectSummary(&projects[i]))
	}
	return items, paginationMeta(filter, total), nil
}

func (s *ProjectService) ListMine(ctx context.Context, actorID uuid.UUID, filter ProjectListFilter) ([]ProjectSummaryResponse, ProjectPaginationMeta, error) {
	if actorID == uuid.Nil {
		return nil, ProjectPaginationMeta{}, sharedErrors.ErrUnauthorized
	}
	filter, err := validateListFilter(filter)
	if err != nil {
		return nil, ProjectPaginationMeta{}, err
	}
	if filter.Scope == "" {
		filter.Scope = "owned"
	}
	if filter.Scope != "owned" && filter.Scope != "initiated" {
		return nil, ProjectPaginationMeta{}, sharedErrors.ErrBadRequest
	}
	err = s.repository.WithTransaction(ctx, func(tx ProjectTx) error {
		actor, err := tx.FindActor(ctx, actorID)
		if err != nil {
			return actorError(err)
		}
		return requireEligibleActor(actor)
	})
	if err != nil {
		return nil, ProjectPaginationMeta{}, err
	}
	projects, total, err := s.repository.ListMine(ctx, actorID, filter)
	if err != nil {
		return nil, ProjectPaginationMeta{}, err
	}
	items := make([]ProjectSummaryResponse, 0, len(projects))
	for i := range projects {
		items = append(items, mapProjectSummary(&projects[i]))
	}
	return items, paginationMeta(filter, total), nil
}

func (s *ProjectService) projectDetailForActor(ctx context.Context, actorID *uuid.UUID, aggregate *ProjectAggregate) (*ProjectDetailResponse, error) {
	isPublic := aggregate.Visibility == "public" && aggregate.Status != ProjectStatusDraft && aggregate.Status != ProjectStatusArchived
	canEdit := false
	canRead := isPublic
	var actor *ProjectActor
	if actorID != nil && *actorID != uuid.Nil {
		err := s.repository.WithTransaction(ctx, func(tx ProjectTx) error {
			foundActor, err := tx.FindActor(ctx, *actorID)
			if err != nil {
				if err == gorm.ErrRecordNotFound {
					return nil
				}
				return err
			}
			if err := requireEligibleActor(foundActor); err != nil {
				return nil
			}
			actor = foundActor
			if isEditableStatus(aggregate.Status) {
				editErr := requireProjectEditor(ctx, tx, *actorID, aggregate)
				if editErr == nil {
					canEdit = true
				} else if !errors.Is(editErr, sharedErrors.ErrNotFound) {
					return editErr
				}
			}
			member, err := tx.IsActiveProjectMember(ctx, aggregate.ID, *actorID)
			if err != nil {
				return err
			}
			canRead = canRead || canEdit || member
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	if !canRead {
		return nil, sharedErrors.ErrNotFound
	}
	canPublish := canEdit && actor != nil && canPublishActor(actor)
	return mapProjectDetail(aggregate, uuid.Nil, canEdit, canPublish), nil
}

func isEditableStatus(status ProjectStatus) bool {
	return status == ProjectStatusDraft || status == ProjectStatusOpen || status == ProjectStatusAwaitingOwner
}

func validateListFilter(filter ProjectListFilter) (ProjectListFilter, error) {
	if filter.Page == 0 {
		filter.Page = 1
	}
	if filter.Limit == 0 {
		filter.Limit = 20
	}
	if filter.Page < 1 || filter.Limit < 1 || filter.Limit > 100 || runeLength(filter.Search) > 200 {
		return ProjectListFilter{}, sharedErrors.ErrBadRequest
	}
	filter.Search = strings.TrimSpace(filter.Search)
	return filter, nil
}

func paginationMeta(filter ProjectListFilter, total int64) ProjectPaginationMeta {
	pages := 0
	if total > 0 {
		pages = int((total + int64(filter.Limit) - 1) / int64(filter.Limit))
	}
	return ProjectPaginationMeta{Page: filter.Page, Limit: filter.Limit, Total: total, TotalPages: pages}
}

func mapProjectSummary(aggregate *ProjectAggregate) ProjectSummaryResponse {
	skills := make(map[uuid.UUID]ProjectMasterResponse)
	tools := make(map[uuid.UUID]ProjectMasterResponse)
	totalCapacity := 0
	filledCapacity := 0
	for _, role := range aggregate.Roles {
		if role.ArchivedAt != nil {
			continue
		}
		totalCapacity += role.Capacity
		filledCapacity += role.FilledCapacity
		for _, skill := range role.Skills {
			skills[skill.ID] = ProjectMasterResponse{ID: skill.ID, Name: skill.Name, Slug: skill.Slug}
		}
		for _, tool := range role.Tools {
			tools[tool.ID] = ProjectMasterResponse{ID: tool.ID, Name: tool.Name, Slug: tool.Slug}
		}
	}
	skillItems := make([]ProjectMasterResponse, 0, len(skills))
	for _, item := range skills {
		skillItems = append(skillItems, item)
	}
	toolItems := make([]ProjectMasterResponse, 0, len(tools))
	for _, item := range tools {
		toolItems = append(toolItems, item)
	}
	sort.Slice(skillItems, func(i, j int) bool { return strings.ToLower(skillItems[i].Name) < strings.ToLower(skillItems[j].Name) })
	sort.Slice(toolItems, func(i, j int) bool { return strings.ToLower(toolItems[i].Name) < strings.ToLower(toolItems[j].Name) })
	return ProjectSummaryResponse{
		ID: aggregate.ID, Slug: aggregate.Slug, Title: aggregate.Title, Summary: aggregate.Summary,
		ProjectCategory: aggregate.ProjectCategory, Origin: aggregate.Origin, Status: aggregate.Status,
		OwnerID: cloneUUIDPointer(aggregate.OwnerID), OwnerProfile: aggregate.OwnerProfile,
		InitiatorOrganizationID: cloneUUIDPointer(aggregate.InitiatorOrganizationID),
		InitiatorOrganization:   aggregate.InitiatorOrganization,
		TotalCapacity:           totalCapacity, FilledCapacity: filledCapacity, HoursPerWeek: aggregate.HoursPerWeek,
		Deadline: projectDateFromTime(aggregate.Deadline), Skills: skillItems, Tools: toolItems, CreatedAt: aggregate.CreatedAt,
	}
}
