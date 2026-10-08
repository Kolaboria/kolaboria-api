package project

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	sharedErrors "github.com/syahrulfahmi/kolaboria-api/internal/shared/errors"
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
