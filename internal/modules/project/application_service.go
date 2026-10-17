package project

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/syahrulfahmi/kolaboria-api/internal/infrastructure/workspaceactivity"
	sharedErrors "github.com/syahrulfahmi/kolaboria-api/internal/shared/errors"
	"gorm.io/gorm"
)

type ApplicationService struct {
	repository ApplicationRepository
	now        func() time.Time
}

func NewApplicationService(repository ApplicationRepository, now func() time.Time) *ApplicationService {
	if now == nil {
		now = time.Now
	}
	return &ApplicationService{repository: repository, now: now}
}

func (s *ApplicationService) GetApplicantSummary(ctx context.Context, actorID, projectID uuid.UUID) (*ApplicantSummaryResponse, error) {
	actor, err := s.reader(ctx, actorID)
	if err != nil {
		return nil, err
	}
	project, err := s.repository.FindOwnerProject(ctx, actorID, projectID)
	if err != nil {
		return nil, applicationProjectError(err)
	}
	model, err := s.repository.GetOwnerSummary(ctx, actorID, projectID)
	if err != nil {
		return nil, applicationProjectError(err)
	}
	canReview := canReviewProject(project.Status) && actor.EmailVerifiedAt != nil
	var reason *string
	if !canReview {
		value := "project_not_reviewable"
		if canReviewProject(project.Status) && actor.EmailVerifiedAt == nil {
			value = "applicant_ineligible"
		}
		reason = &value
	}
	return pointer(mapApplicantSummary(*model, canReview, reason)), nil
}

func (s *ApplicationService) ListProjectApplicants(ctx context.Context, actorID, projectID uuid.UUID, filter ApplicationListFilter) ([]ApplicantListItemResponse, ProjectPaginationMeta, error) {
	if _, err := s.reader(ctx, actorID); err != nil {
		return nil, ProjectPaginationMeta{}, err
	}
	if _, err := s.repository.FindOwnerProject(ctx, actorID, projectID); err != nil {
		return nil, ProjectPaginationMeta{}, applicationProjectError(err)
	}
	filter, err := validateOwnerApplicationFilter(filter)
	if err != nil {
		return nil, ProjectPaginationMeta{}, err
	}
	if len(filter.ProjectRoleIDs) > 0 {
		count, err := s.repository.CountProjectRoleIDs(ctx, projectID, filter.ProjectRoleIDs)
		if err != nil {
			return nil, ProjectPaginationMeta{}, err
		}
		if count != int64(len(filter.ProjectRoleIDs)) {
			return nil, ProjectPaginationMeta{}, sharedErrors.ErrBadRequest
		}
	}
	models, total, err := s.repository.ListOwnerApplications(ctx, actorID, projectID, filter)
	if err != nil {
		return nil, ProjectPaginationMeta{}, err
	}
	items := make([]ApplicantListItemResponse, 0, len(models))
	for _, model := range models {
		items = append(items, mapApplicationListItem(model))
	}
	return items, paginationMeta(ProjectListFilter{Page: filter.Page, Limit: filter.Limit}, total), nil
}

func (s *ApplicationService) GetApplicationDetail(ctx context.Context, actorID, applicationID uuid.UUID) (*ApplicationDetailResponse, error) {
	actor, err := s.reader(ctx, actorID)
	if err != nil {
		return nil, err
	}
	model, err := s.repository.GetVisibleApplication(ctx, actorID, applicationID)
	if err != nil {
		return nil, applicationProjectError(err)
	}
	_, ownerErr := s.repository.FindOwnerProject(ctx, actorID, model.Application.ProjectID)
	isOwner := ownerErr == nil
	if ownerErr != nil && !errors.Is(ownerErr, gorm.ErrRecordNotFound) {
		return nil, ownerErr
	}
	applicant, err := s.repository.FindActorForRead(ctx, model.Application.ApplicantUserID)
	if err != nil {
		return nil, actorError(err)
	}
	member, err := s.repository.IsActiveProjectMember(ctx, model.Application.ProjectID, model.Application.ApplicantUserID)
	if err != nil {
		return nil, err
	}
	permissions := s.applicationPermissions(actor, applicant, *model, isOwner, member)
	result := mapApplicationDetail(*model, permissions)
	return &result, nil
}

func (s *ApplicationService) ListMyApplications(ctx context.Context, actorID uuid.UUID, filter ApplicationListFilter) ([]MyApplicationListItemResponse, ProjectPaginationMeta, error) {
	if _, err := s.reader(ctx, actorID); err != nil {
		return nil, ProjectPaginationMeta{}, err
	}
	filter, err := validateMyApplicationFilter(filter)
	if err != nil {
		return nil, ProjectPaginationMeta{}, err
	}
	models, total, err := s.repository.ListApplicantApplications(ctx, actorID, filter)
	if err != nil {
		return nil, ProjectPaginationMeta{}, err
	}
	items := make([]MyApplicationListItemResponse, 0, len(models))
	for _, model := range models {
		items = append(items, mapMyApplicationListItem(model))
	}
	return items, paginationMeta(ProjectListFilter{Page: filter.Page, Limit: filter.Limit}, total), nil
}

func (s *ApplicationService) GetMyApplicationSummary(ctx context.Context, actorID uuid.UUID) (*MyApplicationSummaryResponse, error) {
	if _, err := s.reader(ctx, actorID); err != nil {
		return nil, err
	}
	stats, err := s.repository.GetMySummary(ctx, actorID)
	if err != nil {
		return nil, err
	}
	return &MyApplicationSummaryResponse{Stats: mapApplicationStats(*stats)}, nil
}

func (s *ApplicationService) ReviewApplication(ctx context.Context, actorID, applicationID uuid.UUID, request ReviewApplicationRequest) (*ApplicationDetailResponse, error) {
	if actorID == uuid.Nil || applicationID == uuid.Nil {
		return nil, sharedErrors.ErrBadRequest
	}
	request, err := normalizeReviewRequest(request)
	if err != nil {
		return nil, err
	}
	identity, err := s.repository.FindApplicationIdentity(ctx, applicationID)
	if err != nil {
		return nil, applicationProjectError(err)
	}
	decisionTime := s.now().UTC()
	err = s.repository.WithApplicationTransaction(ctx, func(tx ApplicationTx) error {
		actor, err := tx.FindActor(ctx, actorID)
		if err != nil {
			return actorError(err)
		}
		if err := requireApplicationWriter(actor); err != nil {
			return err
		}
		project, err := tx.LockProjectState(ctx, identity.ProjectID)
		if err != nil {
			return applicationProjectError(err)
		}
		if err := requireApplicationManager(actorID, project); err != nil {
			return err
		}
		role, err := tx.LockRoleForApplication(ctx, project.ID, identity.ProjectRoleID)
		if err != nil {
			return applicationProjectError(err)
		}
		application, err := tx.LockApplication(ctx, applicationID)
		if err != nil {
			return applicationProjectError(err)
		}
		if application.ProjectID != project.ID || application.ProjectRoleID != role.ID {
			return sharedErrors.ErrConflict
		}
		if application.Status == string(request.Status) {
			if sameStringPointer(application.ReviewerNote, request.ReviewerNote) {
				return nil
			}
			return sharedErrors.ErrConflict
		}
		if application.Status != string(ApplicationStatusPending) {
			return sharedErrors.ErrConflict
		}
		if request.Status == ApplicationStatusAccepted {
			if project.Status != ProjectStatusOpen {
				return sharedErrors.ErrConflict
			}
			if role.ArchivedAt != nil {
				return sharedErrors.ErrConflict
			}
			applicant, err := tx.FindActorForRead(ctx, application.ApplicantUserID)
			if err != nil {
				return actorError(err)
			}
			if err := requireApplicationWriter(applicant); err != nil {
				return err
			}
			if applicant.ID == project.OwnerIDValue() {
				return sharedErrors.ErrConflict
			}
			if project.InitiatorOrganizationID != nil {
				isInitiator, err := tx.IsAuthorizedOrganizationInitiator(ctx, applicant.ID, *project.InitiatorOrganizationID)
				if err != nil {
					return err
				}
				if isInitiator {
					return sharedErrors.ErrConflict
				}
			}
			member, err := tx.IsActiveProjectMember(ctx, project.ID, applicant.ID)
			if err != nil {
				return err
			}
			if member {
				return sharedErrors.ErrConflict
			}
			usage, err := tx.CountRoleUsage(ctx, role.ID)
			if err != nil {
				return err
			}
			if role.Capacity-usage.ActiveContributors <= 0 {
				return sharedErrors.ErrConflict
			}
			contributionRoleID := cloneUUIDPointer(role.ContributionRoleID)
			memberRecord := &ProjectMember{ID: uuid.New(), ProjectID: project.ID, UserID: applicant.ID, MembershipRole: "contributor", Status: "active", ProjectRoleID: uuidPointer(role.ID), ContributionRoleID: contributionRoleID, JoinedAt: decisionTime, CreatedAt: decisionTime, UpdatedAt: decisionTime}
			if err := tx.CreateContributor(ctx, memberRecord); err != nil {
				return mapPersistenceError(err)
			}
			if err := tx.AppendActivity(ctx, workspaceactivity.Input{ProjectID: project.ID, ActorID: actorID, SubjectType: "project_member", SubjectID: memberRecord.ID, Type: "member.joined", At: decisionTime,
				Payload: map[string]any{"application_id": application.ID, "member_id": memberRecord.ID, "user_id": applicant.ID, "project_role_id": role.ID, "contribution_role_id": contributionRoleID},
			}); err != nil {
				return err
			}
		}
		if request.Status == ApplicationStatusRejected && !canRejectProject(project.Status) {
			return sharedErrors.ErrConflict
		}
		application.Status = string(request.Status)
		application.ReviewerNote = cloneStringPointer(request.ReviewerNote)
		application.ReviewedBy = uuidPointer(actorID)
		application.ReviewedAt = &decisionTime
		application.WithdrawnAt = nil
		application.UpdatedAt = decisionTime
		if err := tx.UpdateApplication(ctx, application, ApplicationStatusPending); err != nil {
			return mapPersistenceError(err)
		}
		if request.Status == ApplicationStatusAccepted {
			if err := tx.BumpProjectVersion(ctx, project.ID, decisionTime); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, mapPersistenceError(err)
	}
	return s.GetApplicationDetail(ctx, actorID, applicationID)
}

func (s *ApplicationService) WithdrawApplication(ctx context.Context, actorID, applicationID uuid.UUID) (*ApplicationDetailResponse, error) {
	if actorID == uuid.Nil || applicationID == uuid.Nil {
		return nil, sharedErrors.ErrBadRequest
	}
	identity, err := s.repository.FindApplicationIdentity(ctx, applicationID)
	if err != nil {
		return nil, applicationProjectError(err)
	}
	now := s.now().UTC()
	err = s.repository.WithApplicationTransaction(ctx, func(tx ApplicationTx) error {
		actor, err := tx.FindActorForRead(ctx, actorID)
		if err != nil {
			return actorError(err)
		}
		if err := requireApplicationReader(actor); err != nil {
			return err
		}
		project, err := tx.LockProjectState(ctx, identity.ProjectID)
		if err != nil {
			return applicationProjectError(err)
		}
		role, err := tx.LockRoleForApplication(ctx, project.ID, identity.ProjectRoleID)
		if err != nil {
			return applicationProjectError(err)
		}
		application, err := tx.LockApplication(ctx, applicationID)
		if err != nil {
			return applicationProjectError(err)
		}
		if application.ProjectID != project.ID || application.ProjectRoleID != role.ID || application.ApplicantUserID != actorID {
			return sharedErrors.ErrNotFound
		}
		if application.Status == string(ApplicationStatusWithdrawn) {
			return nil
		}
		if application.Status != string(ApplicationStatusPending) {
			return sharedErrors.ErrConflict
		}
		application.Status = string(ApplicationStatusWithdrawn)
		application.WithdrawnAt = &now
		application.UpdatedAt = now
		return mapPersistenceError(tx.UpdateApplication(ctx, application, ApplicationStatusPending))
	})
	if err != nil {
		return nil, mapPersistenceError(err)
	}
	return s.GetApplicationDetail(ctx, actorID, applicationID)
}

func (s *ApplicationService) reader(ctx context.Context, actorID uuid.UUID) (*ProjectActor, error) {
	if actorID == uuid.Nil {
		return nil, sharedErrors.ErrUnauthorized
	}
	actor, err := s.repository.FindActorForRead(ctx, actorID)
	if err != nil {
		return nil, actorError(err)
	}
	if err := requireApplicationReader(actor); err != nil {
		return nil, err
	}
	return actor, nil
}

func (s *ApplicationService) applicationPermissions(actor, applicant *ProjectActor, model ApplicationReadModel, owner, alreadyMember bool) ApplicationPermissionsResponse {
	permissions := ApplicationPermissionsResponse{}
	if !owner {
		return permissions
	}
	if model.Application.Status != string(ApplicationStatusPending) {
		reason := "application_not_pending"
		permissions.AcceptBlockReason = &reason
		permissions.RejectBlockReason = &reason
		return permissions
	}
	if canReviewProject(model.Project.Status) {
		permissions.CanReject = actor.EmailVerifiedAt != nil
	} else {
		reason := "project_not_reviewable"
		permissions.RejectBlockReason = &reason
	}
	if model.Project.Status != ProjectStatusOpen {
		reason := "project_not_open"
		permissions.AcceptBlockReason = &reason
	} else if model.Role.Role.ArchivedAt != nil {
		reason := "role_archived"
		permissions.AcceptBlockReason = &reason
	} else if model.Role.Role.Capacity-model.Role.ActiveMembers <= 0 {
		reason := "role_full"
		permissions.AcceptBlockReason = &reason
	} else if alreadyMember {
		reason := "already_member"
		permissions.AcceptBlockReason = &reason
	} else if applicant == nil || !applicant.IsActive || applicant.DeletedAt != nil || applicant.AccountType == nil || applicant.EmailVerifiedAt == nil {
		reason := "applicant_ineligible"
		permissions.AcceptBlockReason = &reason
	} else {
		permissions.CanAccept = true
	}
	return permissions
}

func canReviewProject(status ProjectStatus) bool {
	return status == ProjectStatusOpen || status == ProjectStatusInProgress || status == ProjectStatusCompleted || status == ProjectStatusArchived
}
func canRejectProject(status ProjectStatus) bool { return canReviewProject(status) }
func sameStringPointer(left, right *string) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}
func applicationProjectError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return sharedErrors.ErrNotFound
	}
	return err
}
func pointer[T any](value T) *T { return &value }
