package project

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/syahrulfahmi/kolaboria-api/internal/infrastructure/workspaceactivity"
	sharedErrors "github.com/syahrulfahmi/kolaboria-api/internal/shared/errors"
	"gorm.io/gorm"
)

var (
	ErrProjectHasHistory                               = fmt.Errorf("%w: project has collaboration history", sharedErrors.ErrConflict)
	ErrProjectNotDraft                                 = fmt.Errorf("%w: project is not a draft", sharedErrors.ErrConflict)
	ErrProjectStatusTransition                         = fmt.Errorf("%w: unsupported project transition", sharedErrors.ErrConflict)
	ErrProjectStaleVersion                             = fmt.Errorf("%w: stale project version", sharedErrors.ErrConflict)
	ErrProjectUnderfilledPositionsConfirmationRequired = fmt.Errorf("underfilled project positions require explicit confirmation: %w", sharedErrors.ErrConflict)
)

type ChangeProjectStatusRequest struct {
	Version                     int64         `json:"version"`
	Status                      ProjectStatus `json:"status"`
	ConfirmUnderfilledPositions bool          `json:"confirm_underfilled_positions,omitempty"`
}

func (request *ChangeProjectStatusRequest) UnmarshalJSON(data []byte) error {
	if _, err := strictObject(data, []string{"version", "status"}, nil, "confirm_underfilled_positions"); err != nil {
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
		(request.Status != ProjectStatusDraft && request.Status != ProjectStatusInProgress && request.Status != ProjectStatusArchived) {
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
		if request.Status == ProjectStatusInProgress {
			for _, role := range aggregate.Roles {
				if role.ArchivedAt != nil {
					continue
				}
				_, readiness, err := EvaluatePositionCapacity(role.Capacity, role.FilledCapacity)
				if err != nil {
					return err
				}
				if readiness == PositionReadinessToFill && !request.ConfirmUnderfilledPositions {
					return ErrProjectUnderfilledPositionsConfirmationRequired
				}
			}
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
		eventType := ""
		if request.Status == ProjectStatusInProgress {
			eventType = "project.started"
		}
		if request.Status == ProjectStatusArchived {
			eventType = "project.archived"
		}
		if eventType != "" {
			if err := tx.AppendActivity(ctx, workspaceactivity.Input{ProjectID: projectID, ActorID: actorID, SubjectType: "project", SubjectID: projectID, Type: eventType, At: s.now().UTC(), Payload: map[string]any{"from": ProjectStatusOpen, "to": request.Status, "version": request.Version + 1, "underfilled_confirmed": request.ConfirmUnderfilledPositions}}); err != nil {
				return err
			}
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
