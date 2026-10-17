package workspace

import (
	"time"

	"github.com/google/uuid"
	"github.com/syahrulfahmi/kolaboria-api/internal/modules/project"
)

const (
	WorkspaceOverviewTaskPreviewLimit       = 5
	WorkspaceOverviewDiscussionPreviewLimit = 2
	WorkspaceOverviewActivityPreviewLimit   = 3
)

type OverviewTaskSummaryResponse struct {
	TotalTasks       int64 `json:"total_tasks"`
	TodoTasks        int64 `json:"todo_tasks"`
	InProgressTasks  int64 `json:"in_progress_tasks"`
	ReviewTasks      int64 `json:"review_tasks"`
	DoneTasks        int64 `json:"done_tasks"`
	ClosedTasks      int64 `json:"closed_tasks"`
	OutstandingTasks int64 `json:"outstanding_tasks"`
}

type OverviewTeamSummaryResponse struct {
	ActiveMemberCount int64 `json:"active_member_count"`
}

type OverviewTaskPreviewResponse struct {
	ID       uuid.UUID            `json:"id"`
	Title    string               `json:"title"`
	Status   TaskStatus           `json:"status"`
	Priority TaskPriority         `json:"priority"`
	DueDate  *project.ProjectDate `json:"due_date" swaggertype:"string" format:"date"`
}

type OverviewDiscussionPreviewResponse struct {
	ID             uuid.UUID                `json:"id"`
	Title          string                   `json:"title"`
	Author         DiscussionAuthorResponse `json:"author"`
	CreatedAt      time.Time                `json:"created_at"`
	LastActivityAt time.Time                `json:"last_activity_at"`
}

type WorkspaceOverviewResponse struct {
	Project struct {
		ID      uuid.UUID             `json:"id"`
		Slug    string                `json:"slug"`
		Title   string                `json:"title"`
		Status  project.ProjectStatus `json:"status"`
		Version int64                 `json:"version"`
	} `json:"project"`
	Viewer struct {
		Scope        string     `json:"scope"`
		MembershipID *uuid.UUID `json:"membership_id"`
		Capabilities struct {
			CanManageWorkspace bool `json:"can_manage_workspace"`
			CanCreateTask      bool `json:"can_create_task"`
		} `json:"capabilities"`
	} `json:"viewer"`
	TaskSummary       OverviewTaskSummaryResponse         `json:"task_summary"`
	TeamSummary       OverviewTeamSummaryResponse         `json:"team_summary"`
	MyTasks           []OverviewTaskPreviewResponse       `json:"my_tasks"`
	RecentDiscussions []OverviewDiscussionPreviewResponse `json:"recent_discussions"`
	RecentActivities  []ActivityResponse                  `json:"recent_activities"`
}
