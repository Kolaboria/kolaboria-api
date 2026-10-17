package workspace

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/syahrulfahmi/kolaboria-api/internal/infrastructure/workspaceactivity"
	"github.com/syahrulfahmi/kolaboria-api/internal/modules/project"
	"gorm.io/gorm"
)

type completionRepository struct{ db *gorm.DB }
type completionTx struct{ db *gorm.DB }

func NewCompletionRepository(db *gorm.DB) CompletionRepository { return &completionRepository{db: db} }

func (repository *completionRepository) WithTransaction(ctx context.Context, operation func(CompletionTx) error) error {
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return operation(&completionTx{db: tx}) })
}

func (tx *completionTx) LockProject(ctx context.Context, slug string, actorID uuid.UUID) (*WorkspaceProjectAccess, error) {
	return lockWorkspaceProject(tx.db, ctx, slug, actorID)
}

func (tx *completionTx) LoadTaskSummary(projectID uuid.UUID) (ProjectTaskSummaryResponse, error) {
	return loadProjectTaskSummary(tx.db, projectID)
}

func loadProjectTaskSummary(db *gorm.DB, projectID uuid.UUID) (ProjectTaskSummaryResponse, error) {
	var summary ProjectTaskSummaryResponse
	err := db.Model(&WorkspaceTask{}).Select(`COALESCE(SUM(version), 0) AS version,
		COUNT(*) FILTER (WHERE deleted_at IS NULL) AS total_tasks,
		COUNT(*) FILTER (WHERE deleted_at IS NULL AND status = 'done') AS done_tasks,
		COUNT(*) FILTER (WHERE deleted_at IS NULL AND status = 'closed') AS closed_tasks,
		COUNT(*) FILTER (WHERE deleted_at IS NULL AND status NOT IN ('done', 'closed')) AS outstanding_tasks,
		COUNT(*) FILTER (WHERE deleted_at IS NOT NULL) AS deleted_tasks,
		COUNT(*) FILTER (WHERE deleted_at IS NULL AND status = 'closed' AND (close_reason IS NULL OR btrim(close_reason) = '')) AS invalid_closed_tasks`).
		Where("project_id = ?", projectID).Scan(&summary).Error
	return summary, err
}

func findProjectCompletion(db *gorm.DB, projectID uuid.UUID) (*WorkspaceProjectCompletion, error) {
	var record WorkspaceProjectCompletion
	err := db.Where("project_id = ?", projectID).Take(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &record, err
}

func (tx *completionTx) FindCompletion(projectID uuid.UUID) (*WorkspaceProjectCompletion, error) {
	return findProjectCompletion(tx.db, projectID)
}

func (tx *completionTx) InsertCompletion(record *WorkspaceProjectCompletion) error {
	return tx.db.Create(record).Error
}

func (tx *completionTx) MarkCompleted(projectID uuid.UUID, version int64, at time.Time) error {
	result := tx.db.Model(&project.Project{}).Where("id = ? AND version = ? AND status = ?", projectID, version, project.ProjectStatusInProgress).
		Updates(map[string]any{"status": project.ProjectStatusCompleted, "version": version + 1, "updated_at": at})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrCompletionStale
	}
	return nil
}

func (tx *completionTx) AppendActivity(input workspaceactivity.Input) error {
	return workspaceactivity.Record(tx.db, input)
}
