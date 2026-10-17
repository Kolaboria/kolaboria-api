package workspace

import (
	"time"

	"github.com/syahrulfahmi/kolaboria-api/internal/infrastructure/workspaceactivity"

	"github.com/google/uuid"
)

type TaskStatus string

const (
	TaskStatusTodo       TaskStatus = "todo"
	TaskStatusInProgress TaskStatus = "in_progress"
	TaskStatusReview     TaskStatus = "review"
	TaskStatusDone       TaskStatus = "done"
	TaskStatusClosed     TaskStatus = "closed"
)

type TaskPriority string

const (
	TaskPriorityLow    TaskPriority = "low"
	TaskPriorityMedium TaskPriority = "medium"
	TaskPriorityHigh   TaskPriority = "high"
)

type WorkspaceTask struct {
	ID                uuid.UUID    `gorm:"column:id;type:uuid;primaryKey"`
	ProjectID         uuid.UUID    `gorm:"column:project_id;type:uuid;not null"`
	Title             string       `gorm:"column:title;not null"`
	Description       string       `gorm:"column:description;not null"`
	Status            TaskStatus   `gorm:"column:status;not null"`
	Priority          TaskPriority `gorm:"column:priority;not null"`
	DueDate           *time.Time   `gorm:"column:due_date;type:date"`
	AssigneeMemberID  *uuid.UUID   `gorm:"column:assignee_member_id;type:uuid"`
	Version           int64        `gorm:"column:version;not null"`
	ContentRevision   int64        `gorm:"column:content_revision;not null"`
	CreatedByUserID   uuid.UUID    `gorm:"column:created_by_user_id;type:uuid;not null"`
	UpdatedByUserID   uuid.UUID    `gorm:"column:updated_by_user_id;type:uuid;not null"`
	CompletedAt       *time.Time   `gorm:"column:completed_at"`
	CompletedByUserID *uuid.UUID   `gorm:"column:completed_by_user_id;type:uuid"`
	ClosedAt          *time.Time   `gorm:"column:closed_at"`
	ClosedByUserID    *uuid.UUID   `gorm:"column:closed_by_user_id;type:uuid"`
	CloseReason       *string      `gorm:"column:close_reason"`
	DeletedAt         *time.Time   `gorm:"column:deleted_at"`
	DeletedByUserID   *uuid.UUID   `gorm:"column:deleted_by_user_id;type:uuid"`
	IdempotencyKey    *string      `gorm:"column:idempotency_key"`
	RequestHash       *string      `gorm:"column:request_hash"`
	CreatedAt         time.Time    `gorm:"column:created_at;not null"`
	UpdatedAt         time.Time    `gorm:"column:updated_at;not null"`
}

func (WorkspaceTask) TableName() string { return "workspace_tasks" }

type WorkspaceTaskRevision struct {
	ID                       uuid.UUID  `gorm:"column:id;type:uuid;primaryKey"`
	ProjectID                uuid.UUID  `gorm:"column:project_id;type:uuid;not null"`
	TaskID                   uuid.UUID  `gorm:"column:task_id;type:uuid;not null"`
	RevisionNumber           int64      `gorm:"column:revision_number;not null"`
	Title                    string     `gorm:"column:title;not null"`
	Description              string     `gorm:"column:description;not null"`
	ProjectContextSnapshotID uuid.UUID  `gorm:"column:project_context_snapshot_id;type:uuid;not null"`
	ChangedByUserID          uuid.UUID  `gorm:"column:changed_by_user_id;type:uuid;not null"`
	ChangedByMemberID        *uuid.UUID `gorm:"column:changed_by_member_id;type:uuid"`
	ChangedAt                time.Time  `gorm:"column:changed_at;not null"`
}

func (WorkspaceTaskRevision) TableName() string { return "workspace_task_revisions" }

type WorkspaceProjectContextSnapshot = workspaceactivity.ContextSnapshot

type WorkspaceTaskReviewCycle struct {
	ID                        uuid.UUID  `gorm:"column:id;type:uuid;primaryKey"`
	ProjectID                 uuid.UUID  `gorm:"column:project_id;type:uuid;not null"`
	TaskID                    uuid.UUID  `gorm:"column:task_id;type:uuid;not null"`
	CycleNumber               int64      `gorm:"column:cycle_number;not null"`
	TaskRevisionID            uuid.UUID  `gorm:"column:task_revision_id;type:uuid;not null"`
	ProjectContextSnapshotID  uuid.UUID  `gorm:"column:project_context_snapshot_id;type:uuid;not null"`
	AssigneeMemberIDAtRequest uuid.UUID  `gorm:"column:assignee_member_id_at_request;type:uuid;not null"`
	AssigneeRoleIDAtRequest   *uuid.UUID `gorm:"column:assignee_role_id_at_request;type:uuid"`
	AssigneeRoleNameAtRequest *string    `gorm:"column:assignee_role_name_at_request"`
	RequestedByUserID         uuid.UUID  `gorm:"column:requested_by_user_id;type:uuid;not null"`
	RequestedByMemberID       *uuid.UUID `gorm:"column:requested_by_member_id;type:uuid"`
	RequestedAt               time.Time  `gorm:"column:requested_at;not null"`
	State                     string     `gorm:"column:state;not null"`
	ResolvedByUserID          *uuid.UUID `gorm:"column:resolved_by_user_id;type:uuid"`
	ResolvedByMemberID        *uuid.UUID `gorm:"column:resolved_by_member_id;type:uuid"`
	ResolvedAt                *time.Time `gorm:"column:resolved_at"`
	ResolutionReason          *string    `gorm:"column:resolution_reason"`
	ReviewerIsAssignee        *bool      `gorm:"column:reviewer_is_assignee"`
	ReviewerIsRequester       *bool      `gorm:"column:reviewer_is_requester"`
}

func (WorkspaceTaskReviewCycle) TableName() string { return "workspace_task_review_cycles" }

type WorkspaceTaskComment struct {
	ID                      uuid.UUID  `gorm:"column:id;type:uuid;primaryKey"`
	ProjectID               uuid.UUID  `gorm:"column:project_id;type:uuid;not null"`
	TaskID                  uuid.UUID  `gorm:"column:task_id;type:uuid;not null"`
	AuthorUserID            uuid.UUID  `gorm:"column:author_user_id;type:uuid;not null"`
	AuthorMemberID          *uuid.UUID `gorm:"column:author_member_id;type:uuid"`
	Body                    string     `gorm:"column:body;not null"`
	TaskRevisionID          uuid.UUID  `gorm:"column:task_revision_id;type:uuid;not null"`
	ReviewCycleIDAtCreation *uuid.UUID `gorm:"column:review_cycle_id_at_creation;type:uuid"`
	IdempotencyKey          *string    `gorm:"column:idempotency_key"`
	RequestHash             *string    `gorm:"column:request_hash"`
	CreatedAt               time.Time  `gorm:"column:created_at;not null"`
	DeletedAt               *time.Time `gorm:"column:deleted_at"`
}

func (WorkspaceTaskComment) TableName() string { return "workspace_task_comments" }

type WorkspaceActivityEvent = workspaceactivity.Event
