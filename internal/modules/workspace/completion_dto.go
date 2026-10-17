package workspace

import (
	"time"

	"github.com/google/uuid"
)

type CompleteProjectRequest struct {
	Version       int64  `json:"version" binding:"required,min=1"`
	TaskVersion   int64  `json:"task_version" binding:"min=0"`
	ResultSummary string `json:"result_summary" binding:"required,max=5000"`
	ReviewBasis   string `json:"review_basis" binding:"max=5000"`
}

// Version is the sum of all retained task versions, including tombstones.
// Creates and edits monotonically increase it; comments do not change it.
type ProjectTaskSummaryResponse struct {
	Version            int64 `json:"version"`
	TotalTasks         int64 `json:"total_tasks"`
	DoneTasks          int64 `json:"done_tasks"`
	ClosedTasks        int64 `json:"closed_tasks"`
	OutstandingTasks   int64 `json:"outstanding_tasks"`
	DeletedTasks       int64 `json:"deleted_tasks"`
	InvalidClosedTasks int64 `json:"invalid_closed_tasks"`
}

type ProjectCompletionResponse struct {
	ID             uuid.UUID                  `json:"id"`
	CompletedAt    time.Time                  `json:"completed_at"`
	CompletedBy    uuid.UUID                  `json:"completed_by"`
	ResultSummary  string                     `json:"result_summary"`
	ReviewBasis    string                     `json:"review_basis"`
	ProjectVersion int64                      `json:"project_version"`
	TaskSummary    ProjectTaskSummaryResponse `json:"task_summary"`
}

type ProjectClosureResponse struct {
	TaskSummary ProjectTaskSummaryResponse `json:"task_summary"`
	Eligible    bool                       `json:"eligible"`
	Completion  *ProjectCompletionResponse `json:"completion"`
}
