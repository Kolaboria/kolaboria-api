package workspace

import (
	"context"
	"database/sql"
	"encoding/json"
	"sort"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// TaskEvidenceTrail is an internal, source-linked record for future narrative
// processing. It is intentionally not exposed by the Workspace HTTP handlers.
type TaskEvidenceTrail struct {
	Task          TaskEvidenceCurrent        `json:"task"`
	Revisions     []TaskEvidenceRevision     `json:"revisions"`
	ProjectScopes []TaskEvidenceProjectScope `json:"project_scopes"`
	ReviewCycles  []TaskEvidenceReviewCycle  `json:"review_cycles"`
	Comments      []TaskEvidenceCommentRef   `json:"comments"`
	Events        []TaskEvidenceEvent        `json:"events"`
	Sources       []TaskEvidenceSource       `json:"sources"`
}

type TaskEvidenceRevision struct {
	ID                       uuid.UUID  `json:"id"`
	ProjectID                uuid.UUID  `json:"project_id"`
	TaskID                   uuid.UUID  `json:"task_id"`
	RevisionNumber           int64      `json:"revision_number"`
	Title                    string     `json:"title"`
	Description              string     `json:"description"`
	ProjectContextSnapshotID uuid.UUID  `json:"project_context_snapshot_id"`
	ChangedByUserID          uuid.UUID  `json:"changed_by_user_id"`
	ChangedByMemberID        *uuid.UUID `json:"changed_by_member_id"`
	ChangedAt                time.Time  `json:"changed_at"`
}

type TaskEvidenceProjectScope struct {
	ID                        uuid.UUID  `json:"id"`
	ProjectID                 uuid.UUID  `json:"project_id"`
	ProjectVersion            int64      `json:"project_version"`
	ContextHash               string     `json:"context_hash"`
	Title                     string     `json:"title"`
	Summary                   string     `json:"summary"`
	ProjectCategory           string     `json:"project_category"`
	InitiatorOrganizationID   *uuid.UUID `json:"initiator_organization_id"`
	InitiatorOrganizationName *string    `json:"initiator_organization_name"`
	CapturedAt                time.Time  `json:"captured_at"`
	SchemaVersion             int16      `json:"schema_version"`
}

type TaskEvidenceReviewCycle struct {
	ID                        uuid.UUID  `json:"id"`
	CycleNumber               int64      `json:"cycle_number"`
	TaskRevisionID            uuid.UUID  `json:"task_revision_id"`
	ProjectContextSnapshotID  uuid.UUID  `json:"project_context_snapshot_id"`
	AssigneeMemberIDAtRequest uuid.UUID  `json:"assignee_member_id_at_request"`
	AssigneeRoleIDAtRequest   *uuid.UUID `json:"assignee_role_id_at_request"`
	AssigneeRoleNameAtRequest *string    `json:"assignee_role_name_at_request"`
	RequestedByUserID         uuid.UUID  `json:"requested_by_user_id"`
	RequestedByMemberID       *uuid.UUID `json:"requested_by_member_id"`
	RequestedAt               time.Time  `json:"requested_at"`
	State                     string     `json:"state"`
	ResolvedByUserID          *uuid.UUID `json:"resolved_by_user_id"`
	ResolvedByMemberID        *uuid.UUID `json:"resolved_by_member_id"`
	ResolvedAt                *time.Time `json:"resolved_at"`
	ReviewerIsAssignee        *bool      `json:"reviewer_is_assignee"`
	ReviewerIsRequester       *bool      `json:"reviewer_is_requester"`
}

type TaskEvidenceCurrent struct {
	ID                uuid.UUID    `json:"id"`
	ProjectID         uuid.UUID    `json:"project_id"`
	Title             string       `json:"title"`
	Description       string       `json:"description"`
	Status            TaskStatus   `json:"status"`
	Priority          TaskPriority `json:"priority"`
	AssigneeMemberID  *uuid.UUID   `json:"assignee_member_id"`
	Version           int64        `json:"version"`
	ContentRevision   int64        `json:"content_revision"`
	CreatedByUserID   uuid.UUID    `json:"created_by_user_id"`
	UpdatedByUserID   uuid.UUID    `json:"updated_by_user_id"`
	CreatedAt         time.Time    `json:"created_at"`
	UpdatedAt         time.Time    `json:"updated_at"`
	CompletedAt       *time.Time   `json:"completed_at"`
	CompletedByUserID *uuid.UUID   `json:"completed_by_user_id"`
	ClosedAt          *time.Time   `json:"closed_at"`
	ClosedByUserID    *uuid.UUID   `json:"closed_by_user_id"`
	CloseReason       *string      `json:"close_reason"`
	DeletedAt         *time.Time   `json:"deleted_at"`
	DeletedByUserID   *uuid.UUID   `json:"deleted_by_user_id"`
}

type TaskEvidenceCommentRef struct {
	ID             uuid.UUID  `json:"id"`
	AuthorUserID   uuid.UUID  `json:"author_user_id"`
	AuthorMemberID *uuid.UUID `json:"author_member_id"`
	TaskRevisionID uuid.UUID  `json:"task_revision_id"`
	ReviewCycleID  *uuid.UUID `json:"review_cycle_id"`
	CreatedAt      time.Time  `json:"created_at"`
	Deleted        bool       `json:"deleted"`
}

type TaskEvidenceEvent struct {
	ID             uuid.UUID       `json:"id"`
	Type           string          `json:"type"`
	ActorUserID    uuid.UUID       `json:"actor_user_id"`
	ActorMemberID  *uuid.UUID      `json:"actor_member_id"`
	ActorScope     string          `json:"actor_scope"`
	ActorRoleID    *uuid.UUID      `json:"actor_role_id"`
	ActorRoleName  *string         `json:"actor_role_name"`
	OccurredAt     time.Time       `json:"occurred_at"`
	ProjectScopeID uuid.UUID       `json:"project_scope_id"`
	Payload        json.RawMessage `json:"payload"`
}

type TaskEvidenceSource struct {
	Type          string    `json:"source_type"`
	ID            uuid.UUID `json:"source_id"`
	SchemaVersion int16     `json:"schema_version"`
}

// GetTaskEvidenceTrail returns historical task facts only to an actor who can
// currently access the workspace. It deliberately omits comment bodies and
// profile data; callers can resolve references under their own policy.
func (service *TaskService) GetTaskEvidenceTrail(ctx context.Context, actorID uuid.UUID, slug string, taskID uuid.UUID) (*TaskEvidenceTrail, error) {
	access, err := service.repository.ResolveAccess(ctx, slug, actorID)
	if err != nil {
		return nil, mapTaskPersistenceError(err)
	}
	trail := &TaskEvidenceTrail{
		Revisions: []TaskEvidenceRevision{}, ProjectScopes: []TaskEvidenceProjectScope{},
		ReviewCycles: []TaskEvidenceReviewCycle{}, Comments: []TaskEvidenceCommentRef{},
		Events: []TaskEvidenceEvent{}, Sources: []TaskEvidenceSource{},
	}
	err = service.repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var task WorkspaceTask
		if err := tx.Where("project_id = ? AND id = ?", access.Project.ID, taskID).Take(&task).Error; err != nil {
			return err
		}
		trail.Task = TaskEvidenceCurrent{
			ID: task.ID, ProjectID: task.ProjectID, Title: task.Title, Description: task.Description,
			Status: task.Status, Priority: task.Priority, AssigneeMemberID: task.AssigneeMemberID,
			Version: task.Version, ContentRevision: task.ContentRevision, CreatedByUserID: task.CreatedByUserID,
			UpdatedByUserID: task.UpdatedByUserID, CreatedAt: task.CreatedAt, UpdatedAt: task.UpdatedAt,
			CompletedAt: task.CompletedAt, CompletedByUserID: task.CompletedByUserID,
			ClosedAt: task.ClosedAt, ClosedByUserID: task.ClosedByUserID, CloseReason: task.CloseReason,
			DeletedAt: task.DeletedAt, DeletedByUserID: task.DeletedByUserID,
		}
		trail.Sources = append(trail.Sources, TaskEvidenceSource{Type: "task", ID: task.ID, SchemaVersion: 1})
		var revisions []WorkspaceTaskRevision
		if err := tx.Where("project_id = ? AND task_id = ?", access.Project.ID, taskID).Order("revision_number ASC").Find(&revisions).Error; err != nil {
			return err
		}
		for _, revision := range revisions {
			trail.Revisions = append(trail.Revisions, TaskEvidenceRevision{ID: revision.ID, ProjectID: revision.ProjectID, TaskID: revision.TaskID, RevisionNumber: revision.RevisionNumber, Title: revision.Title, Description: revision.Description, ProjectContextSnapshotID: revision.ProjectContextSnapshotID, ChangedByUserID: revision.ChangedByUserID, ChangedByMemberID: revision.ChangedByMemberID, ChangedAt: revision.ChangedAt.UTC()})
		}
		var cycles []WorkspaceTaskReviewCycle
		if err := tx.Where("project_id = ? AND task_id = ?", access.Project.ID, taskID).Order("cycle_number ASC").Find(&cycles).Error; err != nil {
			return err
		}
		for _, cycle := range cycles {
			trail.ReviewCycles = append(trail.ReviewCycles, TaskEvidenceReviewCycle{ID: cycle.ID, CycleNumber: cycle.CycleNumber, TaskRevisionID: cycle.TaskRevisionID, ProjectContextSnapshotID: cycle.ProjectContextSnapshotID, AssigneeMemberIDAtRequest: cycle.AssigneeMemberIDAtRequest, AssigneeRoleIDAtRequest: cycle.AssigneeRoleIDAtRequest, AssigneeRoleNameAtRequest: cycle.AssigneeRoleNameAtRequest, RequestedByUserID: cycle.RequestedByUserID, RequestedByMemberID: cycle.RequestedByMemberID, RequestedAt: cycle.RequestedAt.UTC(), State: cycle.State, ResolvedByUserID: cycle.ResolvedByUserID, ResolvedByMemberID: cycle.ResolvedByMemberID, ResolvedAt: cycle.ResolvedAt, ReviewerIsAssignee: cycle.ReviewerIsAssignee, ReviewerIsRequester: cycle.ReviewerIsRequester})
		}
		var comments []WorkspaceTaskComment
		if err := tx.Where("project_id = ? AND task_id = ?", access.Project.ID, taskID).Order("created_at ASC, id ASC").Find(&comments).Error; err != nil {
			return err
		}
		for _, comment := range comments {
			trail.Comments = append(trail.Comments, TaskEvidenceCommentRef{ID: comment.ID, AuthorUserID: comment.AuthorUserID, AuthorMemberID: comment.AuthorMemberID, TaskRevisionID: comment.TaskRevisionID, ReviewCycleID: comment.ReviewCycleIDAtCreation, CreatedAt: comment.CreatedAt.UTC(), Deleted: comment.DeletedAt != nil})
		}
		var events []WorkspaceActivityEvent
		if err := tx.Where("project_id = ? AND task_id = ?", access.Project.ID, taskID).Order("occurred_at ASC, event_order ASC, id ASC").Find(&events).Error; err != nil {
			return err
		}
		scopeIDs := make(map[uuid.UUID]struct{})
		for _, revision := range revisions {
			scopeIDs[revision.ProjectContextSnapshotID] = struct{}{}
			trail.Sources = append(trail.Sources, TaskEvidenceSource{Type: "task_revision", ID: revision.ID, SchemaVersion: 1})
		}
		for _, cycle := range cycles {
			scopeIDs[cycle.ProjectContextSnapshotID] = struct{}{}
			trail.Sources = append(trail.Sources, TaskEvidenceSource{Type: "review_cycle", ID: cycle.ID, SchemaVersion: 1})
		}
		for _, event := range events {
			scopeIDs[event.ProjectContextSnapshotID] = struct{}{}
			trail.Events = append(trail.Events, TaskEvidenceEvent{ID: event.ID, Type: event.EventType, ActorUserID: event.ActorUserID, ActorMemberID: event.ActorMemberID, ActorScope: event.ActorScope, ActorRoleID: event.ActorRoleID, ActorRoleName: event.ActorRoleName, OccurredAt: event.OccurredAt.UTC(), ProjectScopeID: event.ProjectContextSnapshotID, Payload: append(json.RawMessage(nil), event.Payload...)})
			trail.Sources = append(trail.Sources, TaskEvidenceSource{Type: "activity_event", ID: event.ID, SchemaVersion: event.SchemaVersion})
		}
		for _, comment := range comments {
			trail.Sources = append(trail.Sources, TaskEvidenceSource{Type: "task_comment", ID: comment.ID, SchemaVersion: 1})
		}
		if len(scopeIDs) > 0 {
			ids := make([]uuid.UUID, 0, len(scopeIDs))
			for id := range scopeIDs {
				ids = append(ids, id)
			}
			sort.Slice(ids, func(i, j int) bool { return ids[i].String() < ids[j].String() })
			var scopes []WorkspaceProjectContextSnapshot
			if err := tx.Where("project_id = ? AND id IN ?", access.Project.ID, ids).Order("captured_at ASC, id ASC").Find(&scopes).Error; err != nil {
				return err
			}
			for _, scope := range scopes {
				trail.ProjectScopes = append(trail.ProjectScopes, TaskEvidenceProjectScope{ID: scope.ID, ProjectID: scope.ProjectID, ProjectVersion: scope.ProjectVersion, ContextHash: scope.ContextHash, Title: scope.Title, Summary: scope.Summary, ProjectCategory: scope.ProjectCategory, InitiatorOrganizationID: scope.InitiatorOrganizationID, InitiatorOrganizationName: scope.InitiatorOrganizationName, CapturedAt: scope.CapturedAt.UTC(), SchemaVersion: scope.SchemaVersion})
				trail.Sources = append(trail.Sources, TaskEvidenceSource{Type: "project_context_snapshot", ID: scope.ID, SchemaVersion: scope.SchemaVersion})
			}
		}
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, mapTaskPersistenceError(err)
	}
	return trail, nil
}
