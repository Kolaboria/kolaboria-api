package project

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	sharedErrors "github.com/syahrulfahmi/kolaboria-api/internal/shared/errors"
	"gorm.io/gorm"
)

type lifecycleTx struct {
	ProjectTx
	actor               *ProjectActor
	project             *ProjectAggregate
	hasHistory          bool
	historyError        error
	organizationAllowed bool
	updated             bool
	deleted             bool
}

func (tx *lifecycleTx) FindActor(context.Context, uuid.UUID) (*ProjectActor, error) {
	return tx.actor, nil
}
func (tx *lifecycleTx) LockProject(context.Context, uuid.UUID) (*ProjectAggregate, error) {
	return tx.project, nil
}
func (tx *lifecycleTx) HasProjectHistory(context.Context, uuid.UUID) (bool, error) {
	return tx.hasHistory, tx.historyError
}
func (tx *lifecycleTx) FindInitiableOrganization(_ context.Context, _, organizationID uuid.UUID) (*ProjectOrganization, error) {
	if !tx.organizationAllowed {
		return nil, gorm.ErrRecordNotFound
	}
	return &ProjectOrganization{ID: organizationID}, nil
}
func (tx *lifecycleTx) DeleteDraftProject(context.Context, uuid.UUID, int64) error {
	tx.deleted = true
	return nil
}
func (tx *lifecycleTx) UpdateStatus(_ context.Context, _ uuid.UUID, status ProjectStatus, publishedAt, agreementAt *time.Time, version int64) error {
	tx.updated = true
	tx.project.Status, tx.project.Version = status, version
	tx.project.PublishedAt, tx.project.AgreementAt = publishedAt, agreementAt
	return nil
}

func lifecycleFixture(status ProjectStatus) (*ProjectService, *lifecycleTx) {
	actorID := uuid.New()
	accountType := "individual"
	verifiedAt := time.Now().UTC()
	tx := &lifecycleTx{
		actor: &ProjectActor{ID: actorID, IsActive: true, AccountType: &accountType, EmailVerifiedAt: &verifiedAt},
		project: &ProjectAggregate{Project: Project{ID: uuid.New(), CreationMode: CreationModePersonal,
			OwnerID: &actorID, CreatedByUserID: actorID, Status: status, Version: 3, PublishedAt: &verifiedAt}},
	}
	return NewProjectService(publishStateRepository{tx: tx}), tx
}

func TestProjectLifecycleTransitions(t *testing.T) {
	for _, target := range []ProjectStatus{ProjectStatusDraft, ProjectStatusArchived, ProjectStatusInProgress} {
		t.Run(string(target), func(t *testing.T) {
			service, tx := lifecycleFixture(ProjectStatusOpen)
			tx.hasHistory = target == ProjectStatusArchived
			result, err := service.ChangeStatus(context.Background(), tx.actor.ID, tx.project.ID, ChangeProjectStatusRequest{Version: 3, Status: target})
			if err != nil || !tx.updated || result.Status != target || result.Version != 4 {
				t.Fatalf("expected versioned transition to %s, got %#v, %v", target, result, err)
			}
			if target == ProjectStatusDraft && result.PublishedAt != nil {
				t.Fatal("draft must clear publication timestamp")
			}
			if target == ProjectStatusArchived && result.PublishedAt == nil {
				t.Fatal("archive must retain publication timestamp")
			}
			if target == ProjectStatusInProgress && result.PublishedAt == nil {
				t.Fatal("starting a project must retain publication timestamp")
			}
			if tx.deleted {
				t.Fatal("status transition must retain project history")
			}
		})
	}
}

func TestStartingUnderfilledProjectRequiresExplicitAcknowledgement(t *testing.T) {
	service, tx := lifecycleFixture(ProjectStatusOpen)
	tx.project.Roles = []ProjectRoleAggregate{{
		ProjectRole:    ProjectRole{ID: uuid.New(), Capacity: 3},
		FilledCapacity: 2,
	}}

	_, err := service.ChangeStatus(context.Background(), tx.actor.ID, tx.project.ID, ChangeProjectStatusRequest{
		Version: 3,
		Status:  ProjectStatusInProgress,
	})
	if !errors.Is(err, sharedErrors.ErrConflict) {
		t.Fatalf("expected conflict when underfilled positions are not acknowledged, got %v", err)
	}
	if tx.updated {
		t.Fatal("project status must not change without underfilled acknowledgement")
	}
}

func TestStartingUnderfilledProjectAfterAcknowledgement(t *testing.T) {
	service, tx := lifecycleFixture(ProjectStatusOpen)
	tx.project.Roles = []ProjectRoleAggregate{{
		ProjectRole:    ProjectRole{ID: uuid.New(), Capacity: 3},
		FilledCapacity: 2,
	}}

	result, err := service.ChangeStatus(context.Background(), tx.actor.ID, tx.project.ID, ChangeProjectStatusRequest{
		Version:                     3,
		Status:                      ProjectStatusInProgress,
		ConfirmUnderfilledPositions: true,
	})
	if err != nil || !tx.updated || result.Status != ProjectStatusInProgress {
		t.Fatalf("expected acknowledged underfilled project to start, got %#v, %v", result, err)
	}
}

func TestProjectLifecycleGuards(t *testing.T) {
	for _, test := range []struct {
		name                string
		status, target      ProjectStatus
		version             int64
		history, otherActor bool
		expected            error
	}{
		{"application or contributor history", ProjectStatusOpen, ProjectStatusDraft, 3, true, false, sharedErrors.ErrConflict},
		{"stale version", ProjectStatusOpen, ProjectStatusDraft, 2, false, false, sharedErrors.ErrConflict},
		{"non-owner", ProjectStatusOpen, ProjectStatusArchived, 3, false, true, sharedErrors.ErrNotFound},
		{"completed", ProjectStatusCompleted, ProjectStatusDraft, 3, false, false, sharedErrors.ErrConflict},
		{"draft archive not supported", ProjectStatusDraft, ProjectStatusArchived, 3, false, false, sharedErrors.ErrConflict},
		{"unsupported target", ProjectStatusOpen, ProjectStatusCompleted, 3, false, false, sharedErrors.ErrBadRequest},
	} {
		t.Run(test.name, func(t *testing.T) {
			service, tx := lifecycleFixture(test.status)
			tx.hasHistory = test.history
			actorID := tx.actor.ID
			if test.otherActor {
				actorID = uuid.New()
			}
			_, err := service.ChangeStatus(context.Background(), actorID, tx.project.ID, ChangeProjectStatusRequest{Version: test.version, Status: test.target})
			if !errors.Is(err, test.expected) || tx.updated || tx.deleted {
				t.Fatalf("guard failed: %v, %#v", err, tx)
			}
		})
	}
}

func TestDeleteProjectOnlyRemovesDraftWithoutHistory(t *testing.T) {
	for _, test := range []struct {
		name     string
		status   ProjectStatus
		history  bool
		version  int64
		expected error
	}{
		{"draft", ProjectStatusDraft, false, 3, nil},
		{"open", ProjectStatusOpen, false, 3, sharedErrors.ErrConflict},
		{"archived", ProjectStatusArchived, false, 3, sharedErrors.ErrConflict},
		{"draft with history", ProjectStatusDraft, true, 3, sharedErrors.ErrConflict},
		{"stale draft", ProjectStatusDraft, false, 2, sharedErrors.ErrConflict},
	} {
		t.Run(test.name, func(t *testing.T) {
			service, tx := lifecycleFixture(test.status)
			tx.hasHistory = test.history
			err := service.DeleteDraft(context.Background(), tx.actor.ID, tx.project.ID, DeleteProjectDraftRequest{Version: test.version})
			if !errors.Is(err, test.expected) || tx.deleted != (test.expected == nil) {
				t.Fatalf("unexpected delete: %v, deleted=%v", err, tx.deleted)
			}
		})
	}
}

func TestClaimedOrganizationLifecycleUsesOwner(t *testing.T) {
	service, tx := lifecycleFixture(ProjectStatusOpen)
	tx.project.CreationMode = CreationModeOrganizationInitiated
	tx.project.CreatedByUserID = uuid.New()
	_, err := service.ChangeStatus(context.Background(), tx.actor.ID, tx.project.ID, ChangeProjectStatusRequest{Version: 3, Status: ProjectStatusDraft})
	if err != nil {
		t.Fatalf("claimed owner must manage lifecycle: %v", err)
	}
}

func TestClaimedOrganizationPublicationKeepsOwnerOpen(t *testing.T) {
	service, tx := lifecycleFixture(ProjectStatusOpen)
	tx.project.CreationMode = CreationModeOrganizationInitiated
	tx.project.CreatedByUserID = uuid.New()
	result, err := service.Publish(context.Background(), tx.actor.ID, tx.project.ID, PublishProjectRequest{Version: 3})
	if err != nil || result.Status != ProjectStatusOpen || result.Ownership.OwnerID == nil || *result.Ownership.OwnerID != tx.actor.ID {
		t.Fatalf("claimed project must keep its owner and open publication state: %#v, %v", result, err)
	}
}

func TestDeleteUnclaimedOrganizationDraftRequiresCurrentInitiationPermission(t *testing.T) {
	for _, allowed := range []bool{true, false} {
		service, tx := lifecycleFixture(ProjectStatusDraft)
		organizationID := uuid.New()
		tx.project.CreationMode = CreationModeOrganizationInitiated
		tx.project.OwnerID = nil
		tx.project.InitiatorOrganizationID = &organizationID
		tx.organizationAllowed = allowed
		err := service.DeleteDraft(context.Background(), tx.actor.ID, tx.project.ID, DeleteProjectDraftRequest{Version: 3})
		if allowed && (err != nil || !tx.deleted) {
			t.Fatalf("authorized initiator should delete its unclaimed draft: %v", err)
		}
		if !allowed && (!errors.Is(err, sharedErrors.ErrNotFound) || tx.deleted) {
			t.Fatalf("revoked initiator must not delete draft: %v", err)
		}
	}
}

func TestProjectLifecycleHistoryLookupFailureDoesNotMutate(t *testing.T) {
	service, tx := lifecycleFixture(ProjectStatusDraft)
	tx.historyError = errors.New("history lookup unavailable")
	err := service.DeleteDraft(context.Background(), tx.actor.ID, tx.project.ID, DeleteProjectDraftRequest{Version: 3})
	if !errors.Is(err, tx.historyError) || tx.deleted {
		t.Fatalf("failed history lookup must retain draft: %v", err)
	}
}
