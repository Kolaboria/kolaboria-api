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

type publishStateTx struct {
	ProjectTx
	actor     *ProjectActor
	aggregate *ProjectAggregate
}

func (tx publishStateTx) FindActor(context.Context, uuid.UUID) (*ProjectActor, error) {
	return tx.actor, nil
}

func (tx publishStateTx) LockProject(context.Context, uuid.UUID) (*ProjectAggregate, error) {
	return tx.aggregate, nil
}

type publishStateRepository struct {
	ProjectRepository
	tx ProjectTx
}

func (repository publishStateRepository) WithTransaction(ctx context.Context, fn func(ProjectTx) error) error {
	return fn(repository.tx)
}

func TestPublishRejectsStaleVersionAfterAnotherRequestPublished(t *testing.T) {
	actorID := uuid.New()
	accountType := "individual"
	verifiedAt := time.Now().UTC()
	actor := &ProjectActor{ID: actorID, IsActive: true, AccountType: &accountType, EmailVerifiedAt: &verifiedAt}
	aggregate := &ProjectAggregate{Project: Project{
		ID: uuid.New(), CreationMode: CreationModePersonal, OwnerID: &actorID,
		Status: ProjectStatusOpen, Version: 2, Visibility: "public",
	}}
	repository := publishStateRepository{tx: publishStateTx{actor: actor, aggregate: aggregate}}
	service := NewProjectService(repository)

	_, err := service.Publish(context.Background(), actorID, aggregate.ID, PublishProjectRequest{Version: 1})
	if !errors.Is(err, sharedErrors.ErrConflict) {
		t.Fatalf("stale publish should return conflict, got %v", err)
	}
}

type publishNewTx struct {
	ProjectTx
	actor     *ProjectActor
	aggregate *ProjectAggregate
	saves     int
}

func (tx *publishNewTx) FindActor(context.Context, uuid.UUID) (*ProjectActor, error) {
	return tx.actor, nil
}

func (tx *publishNewTx) FindByClientRequest(context.Context, uuid.UUID, uuid.UUID) (*ProjectAggregate, error) {
	return nil, gorm.ErrRecordNotFound
}

func (tx *publishNewTx) FindMasterReferences(_ context.Context, _ string, ids []uuid.UUID) ([]MasterReference, error) {
	items := make([]MasterReference, 0, len(ids))
	for _, id := range ids {
		items = append(items, MasterReference{ID: id, Name: "Reference", Slug: "reference"})
	}
	return items, nil
}

func (tx *publishNewTx) SlugExists(context.Context, string, *uuid.UUID) (bool, error) {
	return false, nil
}

func (tx *publishNewTx) SaveAggregate(_ context.Context, aggregate *ProjectAggregate, create bool) error {
	if !create {
		return errors.New("expected atomic project creation")
	}
	tx.saves++
	tx.aggregate = aggregate
	return nil
}

func (tx *publishNewTx) LockProject(context.Context, uuid.UUID) (*ProjectAggregate, error) {
	return tx.aggregate, nil
}

type publishNewRepository struct {
	ProjectRepository
	tx           *publishNewTx
	transactions int
}

func (repository *publishNewRepository) WithTransaction(ctx context.Context, fn func(ProjectTx) error) error {
	repository.transactions++
	return fn(repository.tx)
}

func TestCreateAndPublishCreatesPublishedProjectInOneTransaction(t *testing.T) {
	actorID := uuid.New()
	roleID := uuid.New()
	skillID := uuid.New()
	verifiedAt := time.Now().UTC()
	accountType := "individual"
	tx := &publishNewTx{actor: &ProjectActor{
		ID: actorID, IsActive: true, AccountType: &accountType, EmailVerifiedAt: &verifiedAt,
	}}
	repository := &publishNewRepository{tx: tx}
	service := NewProjectService(repository, WithProjectClock(func() time.Time { return verifiedAt }))
	request := CreateProjectDraftRequest{
		ClientRequestID: uuid.New(),
		Definition: ProjectDefinitionInput{
			CreationMode: CreationModePersonal, Title: "Project title", Summary: "Project summary",
			Description: "Project description", Slug: "project-title", ProjectCategory: "product",
			Visibility: "public", Origin: "personal", WhyCollaborative: "Shared effort",
			ContributorOutcome: "A useful result", OwnerCommitment: "I will coordinate",
			Availability: "flexible", HoursPerWeek: 5, CollaborationAgreement: true,
			Roles: []ProjectRoleWriteInput{{ContributionRoleID: &roleID, Description: "Build the feature", Capacity: 1, SkillIDs: []uuid.UUID{skillID}}},
		},
	}

	response, err := service.CreateAndPublish(context.Background(), actorID, request)
	if err != nil {
		t.Fatalf("publish new project: %v", err)
	}
	if repository.transactions != 1 || tx.saves != 1 {
		t.Fatalf("expected one transaction and project save, got transactions=%d saves=%d", repository.transactions, tx.saves)
	}
	if response.Status != ProjectStatusOpen || response.PublishedAt == nil || response.Version != 1 {
		t.Fatalf("new project should be created directly as published, got status=%q version=%d published_at=%v", response.Status, response.Version, response.PublishedAt)
	}
}
