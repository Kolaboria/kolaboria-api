package workspace

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/syahrulfahmi/kolaboria-api/internal/modules/project"
)

type Optional[T any] struct {
	Set   bool
	Value *T
}

func (field *Optional[T]) UnmarshalJSON(data []byte) error {
	field.Set = true
	if string(data) == "null" {
		field.Value = nil
		return nil
	}
	var value T
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	field.Value = &value
	return nil
}

type CreateTaskRequest struct {
	Title            string        `json:"title" binding:"required"`
	Description      string        `json:"description"`
	AssigneeMemberID *uuid.UUID    `json:"assignee_member_id"`
	Priority         *TaskPriority `json:"priority"`
	DueDate          *string       `json:"due_date"`
}

type UpdateTaskRequest struct {
	Version          int64               `json:"version" binding:"required"`
	Title            *string             `json:"title"`
	Description      *string             `json:"description"`
	AssigneeMemberID Optional[uuid.UUID] `json:"assignee_member_id"`
	Priority         *TaskPriority       `json:"priority"`
	DueDate          Optional[string]    `json:"due_date"`
	ToStatus         *TaskStatus         `json:"to_status"`
	CloseReason      *string             `json:"close_reason"`
	ReopenConfirmed  bool                `json:"reopen_confirmed"`
}

type TransitionTaskRequest struct {
	Version         int64      `json:"version" binding:"required"`
	ToStatus        TaskStatus `json:"to_status" binding:"required"`
	CloseReason     *string    `json:"close_reason"`
	ReopenConfirmed bool       `json:"reopen_confirmed"`
}

type CreateTaskCommentRequest struct {
	Body string `json:"body" binding:"required"`
}

type UpdateTaskAccessRequest struct {
	Version               int64 `json:"version" binding:"required"`
	TaskManagementGranted bool  `json:"task_management_granted"`
}

type TaskListQuery struct {
	Status           *TaskStatus
	Query            string
	Mine             bool
	AssigneeMemberID *uuid.UUID
	Page             int
	Limit            int
}

type TaskPermissionsResponse struct {
	EditContent        bool         `json:"edit_content"`
	ManageMetadata     bool         `json:"manage_metadata"`
	Delete             bool         `json:"delete"`
	Reopen             bool         `json:"reopen"`
	AllowedTransitions []TaskStatus `json:"allowed_transitions"`
}

type TaskAssigneeResponse struct {
	MembershipID *uuid.UUID `json:"membership_id"`
	UserID       uuid.UUID  `json:"user_id"`
	Name         string     `json:"name"`
	Avatar       *string    `json:"avatar"`
	Status       string     `json:"status"`
}

type TaskReviewSummaryResponse struct {
	State        string     `json:"state"`
	RequestedAt  time.Time  `json:"requested_at"`
	ResolvedAt   *time.Time `json:"resolved_at"`
	ReviewerName *string    `json:"reviewer_name"`
	SelfReview   bool       `json:"self_review"`
}

type TaskListItemResponse struct {
	ID                 uuid.UUID                  `json:"id"`
	ProjectID          uuid.UUID                  `json:"project_id"`
	Title              string                     `json:"title"`
	DescriptionPreview string                     `json:"description_preview"`
	Status             TaskStatus                 `json:"status"`
	Priority           TaskPriority               `json:"priority"`
	DueDate            *project.ProjectDate       `json:"due_date" swaggertype:"string" format:"date"`
	Assignee           *TaskAssigneeResponse      `json:"assignee"`
	Version            int64                      `json:"version"`
	ContentRevision    int64                      `json:"content_revision"`
	CommentCount       int64                      `json:"comment_count"`
	CreatedAt          time.Time                  `json:"created_at"`
	UpdatedAt          time.Time                  `json:"updated_at"`
	NeedsReassignment  bool                       `json:"needs_reassignment"`
	ReviewSummary      *TaskReviewSummaryResponse `json:"review_summary"`
	Permissions        TaskPermissionsResponse    `json:"permissions"`
}

type TaskResponse struct {
	TaskListItemResponse
	Description string  `json:"description"`
	CloseReason *string `json:"close_reason"`
}

type TaskTombstoneResponse struct {
	ID        uuid.UUID `json:"id"`
	Version   int64     `json:"version"`
	DeletedAt time.Time `json:"deleted_at"`
}

type TaskCommentResponse struct {
	ID             uuid.UUID            `json:"id"`
	TaskID         uuid.UUID            `json:"task_id"`
	Body           string               `json:"body"`
	Author         TaskAssigneeResponse `json:"author"`
	TaskRevisionID uuid.UUID            `json:"task_revision_id"`
	ReviewCycleID  *uuid.UUID           `json:"review_cycle_id"`
	CreatedAt      time.Time            `json:"created_at"`
}

type TaskAccessResponse struct {
	MemberID              uuid.UUID `json:"member_id"`
	TaskManagementGranted bool      `json:"task_management_granted"`
	TaskAccessVersion     int64     `json:"task_access_version"`
}

type TaskViewerAccess struct {
	ActiveMemberID        *uuid.UUID
	TaskManagementGranted bool
}
