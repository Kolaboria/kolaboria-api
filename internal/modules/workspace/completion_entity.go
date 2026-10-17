package workspace

import (
	"time"

	"github.com/google/uuid"
)

// One immutable owner decision per project; no contribution certification is
// implied by this operational closure record.
type WorkspaceProjectCompletion struct {
	ID                   uuid.UUID `gorm:"column:id;type:uuid;primaryKey"`
	ProjectID            uuid.UUID `gorm:"column:project_id;type:uuid"`
	CompletedBy          uuid.UUID `gorm:"column:completed_by;type:uuid"`
	CompletedAt          time.Time `gorm:"column:completed_at"`
	SourceProjectVersion int64     `gorm:"column:source_project_version"`
	ProjectVersion       int64     `gorm:"column:project_version"`
	TaskVersion          int64     `gorm:"column:task_version"`
	TotalTasks           int64     `gorm:"column:total_tasks"`
	DoneTasks            int64     `gorm:"column:done_tasks"`
	ClosedTasks          int64     `gorm:"column:closed_tasks"`
	DeletedTasks         int64     `gorm:"column:deleted_tasks"`
	ResultSummary        string    `gorm:"column:result_summary"`
	ReviewBasis          string    `gorm:"column:review_basis"`
}

func (WorkspaceProjectCompletion) TableName() string { return "workspace_project_completions" }
