package workspace

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/syahrulfahmi/kolaboria-api/internal/modules/project"
	sharedErrors "github.com/syahrulfahmi/kolaboria-api/internal/shared/errors"
)

type ActivityService struct{ repository ActivityRepository }

func NewActivityService(repository ActivityRepository) *ActivityService {
	return &ActivityService{repository: repository}
}

func (service *ActivityService) List(ctx context.Context, actorID uuid.UUID, slug string, page, limit int) ([]ActivityResponse, project.ProjectPaginationMeta, error) {
	if actorID == uuid.Nil || strings.TrimSpace(slug) == "" || page < 1 || limit < 1 {
		return nil, project.ProjectPaginationMeta{}, sharedErrors.ErrBadRequest
	}
	if limit > 50 {
		limit = 50
	}
	rows, total, err := service.repository.LoadPage(ctx, strings.TrimSpace(slug), actorID, page, limit)
	if err != nil {
		return nil, project.ProjectPaginationMeta{}, err
	}
	items := make([]ActivityResponse, 0, len(rows))
	for _, row := range rows {
		items = append(items, mapActivity(row))
	}
	return items, project.ProjectPaginationMeta{Page: page, Limit: limit, Total: total, TotalPages: int((total + int64(limit) - 1) / int64(limit))}, nil
}
