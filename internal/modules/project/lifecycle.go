package project

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	sharedErrors "github.com/syahrulfahmi/kolaboria-api/internal/shared/errors"
	"gorm.io/gorm"
)

var (
	ErrProjectHasHistory       = fmt.Errorf("%w: project has collaboration history", sharedErrors.ErrConflict)
	ErrProjectNotDraft         = fmt.Errorf("%w: project is not a draft", sharedErrors.ErrConflict)
	ErrProjectStatusTransition = fmt.Errorf("%w: unsupported project transition", sharedErrors.ErrConflict)
	ErrProjectStaleVersion     = fmt.Errorf("%w: stale project version", sharedErrors.ErrConflict)
)

type ChangeProjectStatusRequest struct {
	Version int64         `json:"version"`
	Status  ProjectStatus `json:"status"`
}

func (request *ChangeProjectStatusRequest) UnmarshalJSON(data []byte) error {
	if _, err := strictObject(data, []string{"version", "status"}, nil); err != nil {
		return err
	}
	type plain ChangeProjectStatusRequest
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*request = ChangeProjectStatusRequest(decoded)
	return nil
}

type DeleteProjectDraftRequest struct {
	Version int64 `json:"version"`
}

func (request *DeleteProjectDraftRequest) UnmarshalJSON(data []byte) error {
	if _, err := strictObject(data, []string{"version"}, nil); err != nil {
		return err
	}
	type plain DeleteProjectDraftRequest
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*request = DeleteProjectDraftRequest(decoded)
	return nil
}

// ChangeStatus retains all collaboration records. Returning to draft is only
// allowed before any application or non-owner membership has been recorded.
func (s *ProjectService) ChangeStatus(ctx context.Context, actorID, projectID uuid.UUID, request ChangeProjectStatusRequest) (*ProjectDetailResponse, error) {
	if actorID == uuid.Nil || projectID == uuid.Nil || request.Version < 1 ||
		(request.Status != ProjectStatusDraft && request.Status != ProjectStatusArchived) {
		return nil, sharedErrors.ErrBadRequest
	}
	var result *ProjectDetailResponse
	err := s.repository.WithTransaction(ctx, func(tx ProjectTx) error {
		aggregate, actor, err := lockLifecycleProject(ctx, tx, actorID, projectID, request.Version)
		if err != nil {
			return err
		}
		if aggregate.Status != ProjectStatusOpen {
			return ErrProjectStatusTransition
		}
		if request.Status == ProjectStatusDraft {
			history, err := tx.HasProjectHistory(ctx, projectID)
			if err != nil {
				return err
			}
			if history {
				return ErrProjectHasHistory
			}
		}
		publishedAt := aggregate.PublishedAt
		if request.Status == ProjectStatusDraft {
			publishedAt = nil
		}
		if err := tx.UpdateStatus(ctx, projectID, request.Status, publishedAt, aggregate.AgreementAt, aggregate.Version+1); err != nil {
			return err
		}
		stored, err := tx.LockProject(ctx, projectID)
		if err != nil {
			return err
		}
		result = mapProjectDetail(stored, actorID, true, canPublishActor(actor))
		return nil
	})
	return result, mapPersistenceError(err)
}

func (s *ProjectService) DeleteDraft(ctx context.Context, actorID, projectID uuid.UUID, request DeleteProjectDraftRequest) error {
	if actorID == uuid.Nil || projectID == uuid.Nil || request.Version < 1 {
		return sharedErrors.ErrBadRequest
	}
	err := s.repository.WithTransaction(ctx, func(tx ProjectTx) error {
		aggregate, _, err := lockLifecycleProject(ctx, tx, actorID, projectID, request.Version)
		if err != nil {
			return err
		}
		if aggregate.Status != ProjectStatusDraft {
			return ErrProjectNotDraft
		}
		history, err := tx.HasProjectHistory(ctx, projectID)
		if err != nil {
			return err
		}
		if history {
			return ErrProjectHasHistory
		}
		if err := tx.DeleteDraftProject(ctx, projectID, request.Version); errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrProjectStaleVersion
		} else {
			return err
		}
	})
	return mapPersistenceError(err)
}

func lockLifecycleProject(ctx context.Context, tx ProjectTx, actorID, projectID uuid.UUID, version int64) (*ProjectAggregate, *ProjectActor, error) {
	actor, err := tx.FindActor(ctx, actorID)
	if err != nil {
		return nil, nil, actorError(err)
	}
	if err := requireEligibleActor(actor); err != nil {
		return nil, nil, err
	}
	aggregate, err := tx.LockProject(ctx, projectID)
	if err != nil {
		return nil, nil, projectLookupError(err)
	}
	if aggregate.OwnerID != nil {
		if *aggregate.OwnerID != actorID {
			return nil, nil, sharedErrors.ErrNotFound
		}
	} else if err := requireProjectEditor(ctx, tx, actorID, aggregate); err != nil {
		return nil, nil, err
	}
	if aggregate.Version != version {
		return nil, nil, ErrProjectStaleVersion
	}
	return aggregate, actor, nil
}
