package workspace

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/syahrulfahmi/kolaboria-api/internal/infrastructure/workspaceactivity"
	"github.com/syahrulfahmi/kolaboria-api/internal/modules/project"
	sharedErrors "github.com/syahrulfahmi/kolaboria-api/internal/shared/errors"
	"gorm.io/gorm"
)

var (
	ErrCompletionStale   = fmt.Errorf("%w: project or tasks changed; review the latest closure summary", sharedErrors.ErrConflict)
	ErrCompletionBlocked = fmt.Errorf("%w: all tasks must be done or closed with a scope-change reason", sharedErrors.ErrConflict)
)

type CompletionRepository interface {
	WithTransaction(context.Context, func(CompletionTx) error) error
}

type CompletionTx interface {
	LockProject(context.Context, string, uuid.UUID) (*WorkspaceProjectAccess, error)
	LoadTaskSummary(uuid.UUID) (ProjectTaskSummaryResponse, error)
	FindCompletion(uuid.UUID) (*WorkspaceProjectCompletion, error)
	InsertCompletion(*WorkspaceProjectCompletion) error
	MarkCompleted(uuid.UUID, int64, time.Time) error
	AppendActivity(workspaceactivity.Input) error
}

type CompletionService struct {
	repository CompletionRepository
	now        func() time.Time
}

func NewCompletionService(repository CompletionRepository) *CompletionService {
	return &CompletionService{repository: repository, now: time.Now}
}

func (service *CompletionService) Complete(ctx context.Context, actorID uuid.UUID, slug string, request CompleteProjectRequest) (*ProjectCompletionResponse, error) {
	request.ResultSummary = strings.TrimSpace(request.ResultSummary)
	request.ReviewBasis = strings.TrimSpace(request.ReviewBasis)
	if actorID == uuid.Nil || strings.TrimSpace(slug) == "" || request.Version < 1 || request.TaskVersion < 0 ||
		request.ResultSummary == "" || len([]rune(request.ResultSummary)) > 5000 || len([]rune(request.ReviewBasis)) > 5000 {
		return nil, fmt.Errorf("%w: project version, task version and result summary are required", sharedErrors.ErrBadRequest)
	}
	var record *WorkspaceProjectCompletion
	err := service.repository.WithTransaction(ctx, func(tx CompletionTx) error {
		access, err := tx.LockProject(ctx, slug, actorID)
		if err != nil {
			return err
		}
		if !access.IsOwner {
			return sharedErrors.ErrForbidden
		}
		existing, err := tx.FindCompletion(access.Project.ID)
		if err != nil {
			return err
		}
		if existing != nil {
			if access.Project.Status != project.ProjectStatusCompleted || !isCompletionReplay(existing, actorID, request) {
				return ErrCompletionStale
			}
			record = existing
			return nil
		}
		summary, err := tx.LoadTaskSummary(access.Project.ID)
		if err != nil {
			return err
		}
		if err := validateProjectCompletion(access, summary, request); err != nil {
			return err
		}
		now := service.now().UTC()
		record = &WorkspaceProjectCompletion{ID: uuid.New(), ProjectID: access.Project.ID, CompletedBy: actorID,
			CompletedAt: now, SourceProjectVersion: request.Version, ProjectVersion: request.Version + 1,
			TaskVersion: summary.Version, TotalTasks: summary.TotalTasks, DoneTasks: summary.DoneTasks,
			ClosedTasks: summary.ClosedTasks, DeletedTasks: summary.DeletedTasks, ResultSummary: request.ResultSummary, ReviewBasis: request.ReviewBasis}
		if err := tx.InsertCompletion(record); err != nil {
			return err
		}
		if err := tx.MarkCompleted(access.Project.ID, request.Version, now); err != nil {
			return err
		}
		return tx.AppendActivity(workspaceactivity.Input{ProjectID: access.Project.ID, ActorID: actorID, SubjectType: "project",
			SubjectID: access.Project.ID, Type: "project.completed", At: now, Payload: map[string]any{
				"completion_id": record.ID, "from": project.ProjectStatusInProgress, "to": project.ProjectStatusCompleted,
				"version": record.ProjectVersion, "task_version": summary.Version, "done_tasks": summary.DoneTasks,
				"closed_tasks": summary.ClosedTasks, "deleted_tasks": summary.DeletedTasks,
			}})
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("%w: %v", sharedErrors.ErrNotFound, err)
	}
	if err != nil {
		return nil, fmt.Errorf("complete workspace project: %w", err)
	}
	return mapProjectCompletion(record), nil
}

func isCompletionReplay(record *WorkspaceProjectCompletion, actorID uuid.UUID, request CompleteProjectRequest) bool {
	return record.CompletedBy == actorID && record.SourceProjectVersion == request.Version && record.TaskVersion == request.TaskVersion &&
		record.ResultSummary == request.ResultSummary && record.ReviewBasis == request.ReviewBasis
}

func validateProjectCompletion(access *WorkspaceProjectAccess, summary ProjectTaskSummaryResponse, request CompleteProjectRequest) error {
	if !access.IsOwner {
		return sharedErrors.ErrForbidden
	}
	if access.Project.Status != project.ProjectStatusInProgress {
		return fmt.Errorf("%w: only an in-progress project can be completed", sharedErrors.ErrConflict)
	}
	if access.Project.Version != request.Version || summary.Version != request.TaskVersion {
		return ErrCompletionStale
	}
	if summary.OutstandingTasks > 0 || summary.InvalidClosedTasks > 0 {
		return ErrCompletionBlocked
	}
	if summary.TotalTasks == 0 && strings.TrimSpace(request.ReviewBasis) == "" {
		return fmt.Errorf("%w: review basis and evidence limitations are required without structured tasks", sharedErrors.ErrBadRequest)
	}
	return nil
}

func mapProjectCompletion(record *WorkspaceProjectCompletion) *ProjectCompletionResponse {
	if record == nil {
		return nil
	}
	return &ProjectCompletionResponse{
		ID: record.ID, CompletedAt: record.CompletedAt.UTC(), CompletedBy: record.CompletedBy,
		ResultSummary: record.ResultSummary, ReviewBasis: record.ReviewBasis, ProjectVersion: record.ProjectVersion,
		TaskSummary: ProjectTaskSummaryResponse{Version: record.TaskVersion, TotalTasks: record.TotalTasks,
			DoneTasks: record.DoneTasks, ClosedTasks: record.ClosedTasks, DeletedTasks: record.DeletedTasks},
	}
}
