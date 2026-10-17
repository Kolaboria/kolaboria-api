package workspace

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	sharedErrors "github.com/syahrulfahmi/kolaboria-api/internal/shared/errors"
)

type OverviewService struct{ repository OverviewRepository }

func NewOverviewService(repository OverviewRepository) *OverviewService {
	return &OverviewService{repository: repository}
}

func (service *OverviewService) Get(ctx context.Context, actorID uuid.UUID, slug string) (WorkspaceOverviewResponse, error) {
	if actorID == uuid.Nil || strings.TrimSpace(slug) == "" {
		return WorkspaceOverviewResponse{}, sharedErrors.ErrBadRequest
	}
	snapshot, err := service.repository.LoadSnapshot(ctx, strings.TrimSpace(slug), actorID)
	if err != nil {
		return WorkspaceOverviewResponse{}, err
	}
	if snapshot == nil || snapshot.Access == nil {
		return WorkspaceOverviewResponse{}, errors.New("workspace overview repository returned an empty snapshot")
	}
	return mapWorkspaceOverview(snapshot), nil
}
