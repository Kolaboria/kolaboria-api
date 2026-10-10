package project

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	sharedErrors "github.com/syahrulfahmi/kolaboria-api/internal/shared/errors"
	"gorm.io/gorm"
)

type ProjectService struct{ repository ProjectRepository }

func NewProjectService(repository ProjectRepository) *ProjectService {
	return &ProjectService{repository: repository}
}

func (s *ProjectService) CreateDraft(ctx context.Context, actorID uuid.UUID, request CreateProjectDraftRequest) (*ProjectDetailResponse, error) {
	if actorID == uuid.Nil || request.ClientRequestID == uuid.Nil {
		return nil, sharedErrors.ErrBadRequest
	}
	if err := validateDefinition(&request.Definition, true); err != nil {
		return nil, err
	}
	if request.Definition.Visibility != "public" {
		return nil, sharedErrors.ErrBadRequest
	}
	body, err := json.Marshal(request.Definition)
	if err != nil {
		return nil, fmt.Errorf("encode project create request: %w", err)
	}
	hash := sha256.Sum256(body)
	hashValue := hex.EncodeToString(hash[:])

	var response *ProjectDetailResponse
	err = s.repository.WithTransaction(ctx, func(tx ProjectTx) error {
		actor, err := tx.FindActor(ctx, actorID)
		if err != nil {
			return actorError(err)
		}
		if err := requireEligibleActor(actor); err != nil {
			return err
		}
		if request.Definition.CreationMode == CreationModeOrganizationInitiated {
			if _, err := tx.FindInitiableOrganization(ctx, actorID, *request.Definition.InitiatorOrganizationID); err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return sharedErrors.ErrForbidden
				}
				return err
			}
		}
		if existing, err := tx.FindByClientRequest(ctx, actorID, request.ClientRequestID); err == nil {
			if existing.CreateRequestHash != hashValue {
				return sharedErrors.ErrConflict
			}
			response = mapProjectDetail(existing, actorID, false, canPublishActor(actor))
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		roleRefs, skillRefs, toolRefs, err := validateMasterReferences(tx, ctx, request.Definition)
		if err != nil {
			return err
		}
		projectID := uuid.New()
		slug, err := resolveCreateSlug(tx, ctx, request.Definition.Slug, request.Definition.Title, projectID)
		if err != nil {
			return err
		}
		projectRecord := projectFromDefinition(request.Definition)
		projectRecord.ID = projectID
		projectRecord.Slug = slug
		projectRecord.CreatedByUserID = actorID
		projectRecord.Visibility = "public"
		projectRecord.Status = ProjectStatusDraft
		projectRecord.Version = 1
		projectRecord.ClientRequestID = request.ClientRequestID
		projectRecord.CreateRequestHash = hashValue
		projectRecord.CreatedAt = time.Now().UTC()
		projectRecord.UpdatedAt = projectRecord.CreatedAt
		if request.Definition.CreationMode == CreationModePersonal {
			projectRecord.InitiatorUserID = uuidPointer(actorID)
			projectRecord.OwnerID = uuidPointer(actorID)
		} else {
			projectRecord.InitiatorOrganizationID = cloneUUIDPointer(request.Definition.InitiatorOrganizationID)
		}
		aggregate := &ProjectAggregate{Project: projectRecord, Roles: buildRoleAggregates(request.Definition.Roles, roleRefs, skillRefs, toolRefs)}
		if projectRecord.OwnerID != nil {
			aggregate.OwnerMember = &ProjectMember{
				ID: uuid.New(), ProjectID: projectID, UserID: actorID, MembershipRole: "owner", Status: "active",
				ContributionRoleID: cloneUUIDPointer(request.Definition.OwnerContributionRoleID),
				JoinedAt:           projectRecord.CreatedAt, CreatedAt: projectRecord.CreatedAt, UpdatedAt: projectRecord.UpdatedAt,
			}
		}
		if err := tx.SaveAggregate(ctx, aggregate, true); err != nil {
			return mapPersistenceError(err)
		}
		stored, err := tx.LockProject(ctx, projectID)
		if err != nil {
			return err
		}
		response = mapProjectDetail(stored, actorID, true, canPublishActor(actor))
		response.created = true
		return nil
	})
	if err != nil {
		return nil, mapPersistenceError(err)
	}
	return response, nil
}

func (s *ProjectService) UpdateDefinition(ctx context.Context, actorID, projectID uuid.UUID, request UpdateProjectDefinitionRequest) (*ProjectDetailResponse, error) {
	if actorID == uuid.Nil || projectID == uuid.Nil || request.Version < 1 {
		return nil, sharedErrors.ErrBadRequest
	}
	if err := validateDefinition(&request.Definition, false); err != nil {
		return nil, err
	}
	var response *ProjectDetailResponse
	err := s.repository.WithTransaction(ctx, func(tx ProjectTx) error {
		actor, err := tx.FindActor(ctx, actorID)
		if err != nil {
			return actorError(err)
		}
		if err := requireEligibleActor(actor); err != nil {
			return err
		}
		aggregate, err := tx.LockProject(ctx, projectID)
		if err != nil {
			return projectLookupError(err)
		}
		if err := requireProjectEditor(ctx, tx, actorID, aggregate); err != nil {
			return err
		}
		if aggregate.Status == ProjectStatusCompleted || aggregate.Status == ProjectStatusArchived || aggregate.Status == ProjectStatusInProgress {
			return sharedErrors.ErrConflict
		}
		if aggregate.Status != ProjectStatusDraft && !canPublishActor(actor) {
			return sharedErrors.ErrForbidden
		}
		if aggregate.Version != request.Version {
			return sharedErrors.ErrConflict
		}
		if request.Definition.CreationMode != aggregate.CreationMode ||
			!sameUUIDPointer(request.Definition.InitiatorOrganizationID, aggregate.InitiatorOrganizationID) ||
			request.Definition.Visibility != aggregate.Visibility {
			return sharedErrors.ErrBadRequest
		}
		roleRefs, skillRefs, toolRefs, err := validateMasterReferences(tx, ctx, request.Definition)
		if err != nil {
			return err
		}
		if err := validateExistingRoleChanges(ctx, tx, aggregate, request.Definition); err != nil {
			return err
		}
		if aggregate.Status != ProjectStatusDraft {
			if err := validatePublishDefinition(request.Definition); err != nil {
				return err
			}
		}

		applyDefinition(&aggregate.Project, request.Definition)
		if aggregate.OwnerMember != nil && aggregate.CreationMode == CreationModePersonal {
			aggregate.OwnerMember.ContributionRoleID = cloneUUIDPointer(request.Definition.OwnerContributionRoleID)
		}
		aggregate.Roles = buildRoleAggregates(request.Definition.Roles, roleRefs, skillRefs, toolRefs)
		aggregate.Version++
		if request.Definition.CollaborationAgreement && aggregate.AgreementAt == nil {
			now := time.Now().UTC()
			aggregate.AgreementAt = &now
		} else if !request.Definition.CollaborationAgreement && aggregate.Status == ProjectStatusDraft {
			aggregate.AgreementAt = nil
		}
		if err := tx.SaveAggregate(ctx, aggregate, false); err != nil {
			return mapPersistenceError(err)
		}
		stored, err := tx.LockProject(ctx, projectID)
		if err != nil {
			return err
		}
		response = mapProjectDetail(stored, actorID, true, canPublishActor(actor))
		return nil
	})
	if err != nil {
		return nil, mapPersistenceError(err)
	}
	return response, nil
}

func (s *ProjectService) Publish(ctx context.Context, actorID, projectID uuid.UUID, request PublishProjectRequest) (*ProjectDetailResponse, error) {
	if actorID == uuid.Nil || projectID == uuid.Nil || request.Version < 1 {
		return nil, sharedErrors.ErrBadRequest
	}
	var response *ProjectDetailResponse
	err := s.repository.WithTransaction(ctx, func(tx ProjectTx) error {
		actor, err := tx.FindActor(ctx, actorID)
		if err != nil {
			return actorError(err)
		}
		if err := requireEligibleActor(actor); err != nil {
			return err
		}
		if !canPublishActor(actor) {
			return sharedErrors.ErrForbidden
		}
		aggregate, err := tx.LockProject(ctx, projectID)
		if err != nil {
			return projectLookupError(err)
		}
		if err := requireProjectEditor(ctx, tx, actorID, aggregate); err != nil {
			return err
		}
		target := ProjectStatusOpen
		if aggregate.CreationMode == CreationModeOrganizationInitiated {
			target = ProjectStatusAwaitingOwner
		}
		if aggregate.Version != request.Version {
			return sharedErrors.ErrConflict
		}
		if aggregate.Status == target {
			response = mapProjectDetail(aggregate, actorID, true, true)
			return nil
		}
		if aggregate.Status != ProjectStatusDraft {
			return sharedErrors.ErrConflict
		}
		input := definitionFromAggregate(aggregate)
		if err := validatePublishDefinition(input); err != nil {
			return err
		}
		if _, _, _, err := validateMasterReferences(tx, ctx, input); err != nil {
			return err
		}
		for _, role := range aggregate.Roles {
			locked, err := tx.LockRole(ctx, projectID, role.ID)
			if err != nil {
				return err
			}
			usage, err := tx.CountRoleUsage(ctx, role.ID)
			if err != nil {
				return err
			}
			if role.Capacity < usage.ActiveContributors {
				return sharedErrors.ErrConflict
			}
			_ = locked
		}
		now := time.Now().UTC()
		agreementAt := aggregate.AgreementAt
		if agreementAt == nil {
			agreementAt = &now
		}
		if err := tx.UpdateStatus(ctx, projectID, target, &now, agreementAt, aggregate.Version+1); err != nil {
			return mapPersistenceError(err)
		}
		stored, err := tx.LockProject(ctx, projectID)
		if err != nil {
			return err
		}
		response = mapProjectDetail(stored, actorID, true, true)
		return nil
	})
	if err != nil {
		return nil, mapPersistenceError(err)
	}
	return response, nil
}

func (s *ProjectService) Apply(ctx context.Context, actorID, projectID uuid.UUID, request ApplyProjectRequest) (*ProjectApplicationResponse, error) {
	if actorID == uuid.Nil || projectID == uuid.Nil || request.ProjectRoleID == uuid.Nil ||
		len(strings.TrimSpace(request.Motivation)) < 10 || len(request.Motivation) > 5000 ||
		len(request.ExpectedContributionOrEmpty()) > 5000 || len(request.PortfolioLinks) > 10 ||
		(request.EstimatedHoursPerWeek != nil && (*request.EstimatedHoursPerWeek < 1 || *request.EstimatedHoursPerWeek > 168)) {
		return nil, sharedErrors.ErrBadRequest
	}
	if request.Availability != "flexible" && request.Availability != "full_time" && request.Availability != "part_time" && request.Availability != "weekends_only" {
		return nil, sharedErrors.ErrBadRequest
	}
	for _, link := range request.PortfolioLinks {
		if strings.TrimSpace(link) == "" || len(link) > 2048 {
			return nil, sharedErrors.ErrBadRequest
		}
	}

	var response *ProjectApplicationResponse
	err := s.repository.WithTransaction(ctx, func(tx ProjectTx) error {
		actor, err := tx.FindActor(ctx, actorID)
		if err != nil {
			return actorError(err)
		}
		if err := requireEligibleActor(actor); err != nil {
			return err
		}
		if actor.EmailVerifiedAt == nil {
			return sharedErrors.ErrForbidden
		}

		aggregate, err := tx.LockProject(ctx, projectID)
		if err != nil {
			return projectLookupError(err)
		}
		if aggregate.Status != ProjectStatusOpen || aggregate.Visibility != "public" || aggregate.OwnerID == nil {
			return sharedErrors.ErrConflict
		}
		if *aggregate.OwnerID == actorID {
			return sharedErrors.ErrForbidden
		}
		if aggregate.CreationMode == CreationModeOrganizationInitiated && aggregate.InitiatorOrganizationID != nil {
			if _, err := tx.FindInitiableOrganization(ctx, actorID, *aggregate.InitiatorOrganizationID); err == nil {
				return sharedErrors.ErrForbidden
			} else if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		}

		role, err := tx.LockRole(ctx, projectID, request.ProjectRoleID)
		if err != nil {
			return projectLookupError(err)
		}
		usage, err := tx.CountRoleUsage(ctx, role.ID)
		if err != nil {
			return err
		}
		if role.Capacity-usage.ActiveContributors <= 0 {
			return sharedErrors.ErrConflict
		}
		activeApplication, err := tx.FindActiveApplication(ctx, projectID, actorID)
		if err != nil {
			return err
		}
		if activeApplication != nil {
			return sharedErrors.ErrConflict
		}

		now := time.Now().UTC()
		portfolioLinks := request.PortfolioLinks
		if portfolioLinks == nil {
			portfolioLinks = []string{}
		}
		application := &ProjectApplication{
			ID: uuid.New(), ProjectID: projectID, ProjectRoleID: role.ID, ApplicantUserID: actorID,
			Motivation: strings.TrimSpace(request.Motivation), ExpectedContribution: request.ExpectedContribution,
			PortfolioLinks: portfolioLinks, Availability: request.Availability,
			EstimatedHoursPerWeek: request.EstimatedHoursPerWeek, Status: "pending", AppliedAt: now,
			CreatedAt: now, UpdatedAt: now,
		}
		if err := tx.CreateApplication(ctx, application); err != nil {
			return mapPersistenceError(err)
		}
		response = &ProjectApplicationResponse{ID: application.ID, Status: application.Status, AppliedAt: application.AppliedAt}
		return nil
	})
	if err != nil {
		return nil, mapPersistenceError(err)
	}
	return response, nil
}

func (request ApplyProjectRequest) ExpectedContributionOrEmpty() string {
	if request.ExpectedContribution == nil {
		return ""
	}
	return *request.ExpectedContribution
}

func requireEligibleActor(actor *ProjectActor) error {
	if actor == nil || !actor.IsActive || actor.DeletedAt != nil {
		return sharedErrors.ErrForbidden
	}
	if actor.AccountType == nil {
		return sharedErrors.ErrForbidden
	}
	return nil
}

func canPublishActor(actor *ProjectActor) bool {
	return actor != nil && actor.IsActive && actor.DeletedAt == nil && actor.AccountType != nil && actor.EmailVerifiedAt != nil
}

func requireProjectEditor(ctx context.Context, tx ProjectTx, actorID uuid.UUID, aggregate *ProjectAggregate) error {
	if aggregate.CreationMode == CreationModePersonal {
		if aggregate.OwnerID == nil || *aggregate.OwnerID != actorID {
			return sharedErrors.ErrNotFound
		}
		return nil
	}
	if aggregate.CreatedByUserID != actorID || aggregate.InitiatorOrganizationID == nil {
		return sharedErrors.ErrNotFound
	}
	if _, err := tx.FindInitiableOrganization(ctx, actorID, *aggregate.InitiatorOrganizationID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return sharedErrors.ErrNotFound
		}
		return err
	}
	return nil
}

func validateExistingRoleChanges(ctx context.Context, tx ProjectTx, aggregate *ProjectAggregate, input ProjectDefinitionInput) error {
	existing := make(map[uuid.UUID]ProjectRole, len(aggregate.Roles))
	for _, role := range aggregate.Roles {
		existing[role.ID] = role.ProjectRole
	}
	for _, requested := range input.Roles {
		if requested.ID == nil {
			continue
		}
		role, ok := existing[*requested.ID]
		if !ok {
			return sharedErrors.ErrConflict
		}
		lockedRole, err := tx.LockRole(ctx, aggregate.ID, role.ID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return sharedErrors.ErrConflict
			}
			return err
		}
		usage, err := tx.CountRoleUsage(ctx, role.ID)
		if err != nil {
			return err
		}
		if err := ValidateRoleChange(*lockedRole, requested, usage); err != nil {
			return err
		}
	}
	requestedIDs := make(map[uuid.UUID]struct{}, len(input.Roles))
	for _, role := range input.Roles {
		if role.ID != nil {
			requestedIDs[*role.ID] = struct{}{}
		}
	}
	for id := range existing {
		if _, remains := requestedIDs[id]; remains {
			continue
		}
		if _, err := tx.LockRole(ctx, aggregate.ID, id); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return sharedErrors.ErrConflict
			}
			return err
		}
		usage, err := tx.CountRoleUsage(ctx, id)
		if err != nil {
			return err
		}
		if usage.ActiveContributors > 0 || usage.PendingApplications > 0 {
			return sharedErrors.ErrConflict
		}
	}
	return nil
}

func projectFromDefinition(input ProjectDefinitionInput) Project {
	project := Project{
		CreationMode: input.CreationMode, InitiatorOrganizationID: cloneUUIDPointer(input.InitiatorOrganizationID),
		Title: input.Title, Summary: input.Summary, Description: input.Description, Slug: input.Slug,
		ProjectCategory: input.ProjectCategory, Visibility: input.Visibility, Origin: input.Origin,
		WhyCollaborative: input.WhyCollaborative, ContributorOutcome: input.ContributorOutcome,
		OwnerCommitment: input.OwnerCommitment, LeadExpectations: input.LeadExpectations,
		ClientAcknowledgement: input.ClientAcknowledgement, StartDate: projectDatePointer(input.StartDate),
		Deadline: projectDatePointer(input.Deadline), Availability: input.Availability, HoursPerWeek: input.HoursPerWeek,
		CollaborationAgreement: input.CollaborationAgreement,
	}
	return project
}

func applyDefinition(project *Project, input ProjectDefinitionInput) {
	updated := projectFromDefinition(input)
	project.Slug = updated.Slug
	project.Title = updated.Title
	project.Summary = updated.Summary
	project.Description = updated.Description
	project.ProjectCategory = updated.ProjectCategory
	project.Visibility = updated.Visibility
	project.Origin = updated.Origin
	project.WhyCollaborative = updated.WhyCollaborative
	project.ContributorOutcome = updated.ContributorOutcome
	project.OwnerCommitment = updated.OwnerCommitment
	project.LeadExpectations = updated.LeadExpectations
	project.ClientAcknowledgement = updated.ClientAcknowledgement
	project.StartDate = updated.StartDate
	project.Deadline = updated.Deadline
	project.Availability = updated.Availability
	project.HoursPerWeek = updated.HoursPerWeek
	project.CollaborationAgreement = updated.CollaborationAgreement
}

func buildRoleAggregates(inputs []ProjectRoleWriteInput, roles, skills, tools map[uuid.UUID]MasterReference) []ProjectRoleAggregate {
	result := make([]ProjectRoleAggregate, 0, len(inputs))
	for position, input := range inputs {
		id := uuid.New()
		if input.ID != nil {
			id = *input.ID
		}
		role := ProjectRoleAggregate{ProjectRole: ProjectRole{ID: id, ContributionRoleID: cloneUUIDPointer(input.ContributionRoleID), Description: input.Description, Capacity: input.Capacity, Position: position}, Skills: make([]MasterReference, 0, len(input.SkillIDs)), Tools: make([]MasterReference, 0, len(input.ToolIDs))}
		if input.ContributionRoleID != nil {
			if ref, exists := roles[*input.ContributionRoleID]; exists {
				_ = ref
			}
		}
		for _, id := range input.SkillIDs {
			role.Skills = append(role.Skills, skills[id])
		}
		for _, id := range input.ToolIDs {
			role.Tools = append(role.Tools, tools[id])
		}
		result = append(result, role)
	}
	return result
}

func resolveCreateSlug(tx ProjectTx, ctx context.Context, supplied, title string, projectID uuid.UUID) (string, error) {
	slug := supplied
	if slug == "" {
		base := slugify(title)
		if base == "" {
			base = "project"
		}
		suffix := strings.ReplaceAll(projectID.String(), "-", "")[:8]
		if len(base) > 90 {
			base = strings.TrimRight(base[:90], "-")
		}
		slug = base + "-" + suffix
	}
	exists, err := tx.SlugExists(ctx, slug, nil)
	if err != nil {
		return "", err
	}
	if exists {
		if supplied != "" {
			return "", sharedErrors.ErrSlugConflict
		}
		return "", sharedErrors.ErrConflict
	}
	return slug, nil
}

var nonSlugChars = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = nonSlugChars.ReplaceAllString(value, "-")
	return strings.Trim(value, "-")
}

func projectDatePointer(value *ProjectDate) *time.Time {
	if value == nil {
		return nil
	}
	date := value.Time
	return &date
}

func uuidPointer(value uuid.UUID) *uuid.UUID { return &value }

func cloneUUIDPointer(value *uuid.UUID) *uuid.UUID {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func actorError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return sharedErrors.ErrUnauthorized
	}
	return err
}

func projectLookupError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return sharedErrors.ErrNotFound
	}
	return err
}

func mapPersistenceError(err error) error {
	if err == nil {
		return nil
	}
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) {
		switch postgresError.Code {
		case "23505":
			if postgresError.ConstraintName == "projects_slug_key" {
				return sharedErrors.ErrSlugConflict
			}
			return sharedErrors.ErrConflict
		case "23503", "23514", "22P02":
			return sharedErrors.ErrBadRequest
		}
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return sharedErrors.ErrNotFound
	}
	return err
}

func isDatabaseNotFound(err error) bool { return errors.Is(err, gorm.ErrRecordNotFound) }
