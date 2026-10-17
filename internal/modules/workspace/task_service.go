package workspace

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/syahrulfahmi/kolaboria-api/internal/modules/project"
	sharedErrors "github.com/syahrulfahmi/kolaboria-api/internal/shared/errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrTaskProjectNotWritable  = fmt.Errorf("%w: workspace tasks can only be changed while the project is in progress", sharedErrors.ErrConflict)
	ErrTaskVersionConflict     = fmt.Errorf("%w: task has changed since it was opened", sharedErrors.ErrConflict)
	ErrTaskIdempotencyConflict = fmt.Errorf("%w: idempotency key was reused with another task request", sharedErrors.ErrConflict)
)

type TaskService struct{ repository *TaskRepository }

func NewTaskService(repository *TaskRepository) *TaskService {
	return &TaskService{repository: repository}
}

func (service *TaskService) Create(ctx context.Context, actorID uuid.UUID, slug, key string, request CreateTaskRequest) (*TaskResponse, bool, error) {
	title, err := normalizedTaskTitle(request.Title)
	if err != nil {
		return nil, false, err
	}
	description, err := normalizedTaskDescription(request.Description)
	if err != nil {
		return nil, false, err
	}
	if len(key) == 0 || len(key) > 100 {
		return nil, false, sharedErrors.ErrBadRequest
	}
	priority := TaskPriorityMedium
	if request.Priority != nil {
		priority = *request.Priority
	}
	if priority != TaskPriorityLow && priority != TaskPriorityMedium && priority != TaskPriorityHigh {
		return nil, false, sharedErrors.ErrBadRequest
	}
	dueDate, err := parseTaskDate(request.DueDate)
	if err != nil {
		return nil, false, err
	}
	hashInput, _ := json.Marshal(struct {
		Title, Description string
		Assignee           *uuid.UUID
		Priority           TaskPriority
		Due                *time.Time
	}{title, description, request.AssigneeMemberID, priority, dueDate})
	hashBytes := sha256.Sum256(hashInput)
	requestHash := hex.EncodeToString(hashBytes[:])
	var response *TaskResponse
	replayed := false
	err = service.repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		access, err := lockTaskProject(tx, ctx, slug, actorID)
		if err != nil {
			return err
		}
		if !access.CanManage && (!access.ActiveMember || access.Member.MembershipRole != "contributor" || !access.Member.TaskManagementGranted) {
			return sharedErrors.ErrForbidden
		}
		if !access.CanManage {
			if dueDate != nil || priority != TaskPriorityMedium || (request.AssigneeMemberID != nil && (access.Member == nil || *request.AssigneeMemberID != access.Member.ID)) {
				return sharedErrors.ErrForbidden
			}
		}
		var existing WorkspaceTask
		err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("project_id = ? AND created_by_user_id = ? AND idempotency_key = ?", access.Project.ID, actorID, key).Take(&existing).Error
		if err == nil {
			if existing.RequestHash == nil || *existing.RequestHash != requestHash {
				return ErrTaskIdempotencyConflict
			}
			if existing.DeletedAt != nil {
				return sharedErrors.ErrConflict
			}
			response, err = taskResponse(tx, &existing, access, actorID)
			replayed = true
			return err
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err := ensureTaskAssignee(tx, access.Project.ID, request.AssigneeMemberID); err != nil {
			return err
		}
		now := time.Now().UTC()
		snapshot, err := captureTaskContext(tx, access, now)
		if err != nil {
			return err
		}
		id := uuid.New()
		actorKey, requestHashCopy := key, requestHash
		memberID := (*uuid.UUID)(nil)
		if access.Member != nil {
			memberID = &access.Member.ID
		}
		task := WorkspaceTask{ID: id, ProjectID: access.Project.ID, Title: title, Description: description, Status: TaskStatusTodo, Priority: priority, DueDate: dueDate, AssigneeMemberID: request.AssigneeMemberID, Version: 1, ContentRevision: 1, CreatedByUserID: actorID, UpdatedByUserID: actorID, IdempotencyKey: &actorKey, RequestHash: &requestHashCopy, CreatedAt: now, UpdatedAt: now}
		if err := tx.Create(&task).Error; err != nil {
			return err
		}
		revision := WorkspaceTaskRevision{ID: uuid.New(), ProjectID: access.Project.ID, TaskID: id, RevisionNumber: 1, Title: title, Description: description, ProjectContextSnapshotID: snapshot.ID, ChangedByUserID: actorID, ChangedByMemberID: memberID, ChangedAt: now}
		if err := tx.Create(&revision).Error; err != nil {
			return err
		}
		if err := appendTaskEvent(tx, access, actorID, "task", id, &id, snapshot.ID, uuid.New(), 1, "task.created", map[string]any{"task_id": id, "revision_id": revision.ID, "assignee_member_id": request.AssigneeMemberID, "status": TaskStatusTodo, "priority": priority, "due_date": dueDate}, now); err != nil {
			return err
		}
		response, err = taskResponse(tx, &task, access, actorID)
		return err
	})
	if err != nil {
		return nil, false, mapTaskPersistenceError(err)
	}
	return response, replayed, nil
}

func (service *TaskService) List(ctx context.Context, actorID uuid.UUID, slug string, query TaskListQuery) ([]TaskListItemResponse, project.ProjectPaginationMeta, error) {
	if query.Page < 1 {
		query.Page = 1
	}
	if query.Limit < 1 {
		query.Limit = 20
	}
	if query.Limit > 50 {
		query.Limit = 50
	}
	if query.Page-1 > math.MaxInt/query.Limit || (query.Mine && query.AssigneeMemberID != nil) {
		return nil, project.ProjectPaginationMeta{}, sharedErrors.ErrBadRequest
	}
	query.Query = strings.TrimSpace(query.Query)
	if len([]rune(query.Query)) > 160 {
		return nil, project.ProjectPaginationMeta{}, sharedErrors.ErrBadRequest
	}
	access, err := service.repository.ResolveAccess(ctx, slug, actorID)
	if err != nil {
		return nil, project.ProjectPaginationMeta{}, mapTaskPersistenceError(err)
	}
	if query.Status != nil && !validTaskStatus(*query.Status) {
		return nil, project.ProjectPaginationMeta{}, sharedErrors.ErrBadRequest
	}
	var rows []WorkspaceTask
	var total int64
	items := make([]TaskListItemResponse, 0)
	err = service.repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		base := tx.Model(&WorkspaceTask{}).Where("project_id = ? AND deleted_at IS NULL", access.Project.ID)
		if query.Status != nil {
			base = base.Where("status = ?", *query.Status)
		}
		if query.Mine {
			if access.Member == nil {
				base = base.Where("1 = 0")
			} else {
				base = base.Where("assignee_member_id = ?", access.Member.ID)
			}
		}
		if query.AssigneeMemberID != nil {
			base = base.Where("assignee_member_id = ?", *query.AssigneeMemberID)
		}
		if query.Query != "" {
			search := strings.NewReplacer("!", "!!", "%", "!%", "_", "!_").Replace(query.Query)
			base = base.Where("(title ILIKE ? ESCAPE '!' OR description ILIKE ? ESCAPE '!')", "%"+search+"%", "%"+search+"%")
		}
		if err := base.Count(&total).Error; err != nil {
			return err
		}
		if err := base.Order("created_at DESC, id DESC").Offset((query.Page - 1) * query.Limit).Limit(query.Limit).Find(&rows).Error; err != nil {
			return err
		}
		var err error
		items, err = taskListItems(tx, rows, access)
		return err
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, project.ProjectPaginationMeta{}, err
	}
	pages := int((total + int64(query.Limit) - 1) / int64(query.Limit))
	return items, project.ProjectPaginationMeta{Page: query.Page, Limit: query.Limit, Total: total, TotalPages: pages}, nil
}

func (service *TaskService) Get(ctx context.Context, actorID uuid.UUID, slug string, taskID uuid.UUID) (*TaskResponse, error) {
	access, err := service.repository.ResolveAccess(ctx, slug, actorID)
	if err != nil {
		return nil, mapTaskPersistenceError(err)
	}
	var task WorkspaceTask
	if err := service.repository.db.WithContext(ctx).Where("project_id = ? AND id = ? AND deleted_at IS NULL", access.Project.ID, taskID).Take(&task).Error; err != nil {
		return nil, mapTaskPersistenceError(err)
	}
	return taskResponse(service.repository.db.WithContext(ctx), &task, access, actorID)
}

func (service *TaskService) Update(ctx context.Context, actorID uuid.UUID, slug string, taskID uuid.UUID, request UpdateTaskRequest) (*TaskResponse, error) {
	if request.Version < 1 {
		return nil, sharedErrors.ErrBadRequest
	}
	var response *TaskResponse
	err := service.repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		access, err := lockTaskProject(tx, ctx, slug, actorID)
		if err != nil {
			return err
		}
		var task WorkspaceTask
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("project_id = ? AND id = ? AND deleted_at IS NULL", access.Project.ID, taskID).Take(&task).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("%w: task is not active in the requested project", sharedErrors.ErrNotFound)
			}
			return err
		}
		if task.Version != request.Version {
			return ErrTaskVersionConflict
		}
		if task.DeletedAt != nil {
			return sharedErrors.ErrNotFound
		}
		nextTitle, nextDescription := task.Title, task.Description
		contentChanged := false
		if request.Title != nil {
			value, err := normalizedTaskTitle(*request.Title)
			if err != nil {
				return err
			}
			nextTitle = value
			contentChanged = contentChanged || value != task.Title
		}
		if request.Description != nil {
			value, err := normalizedTaskDescription(*request.Description)
			if err != nil {
				return err
			}
			nextDescription = value
			contentChanged = contentChanged || value != task.Description
		}
		nextAssignee, nextPriority, nextDue := task.AssigneeMemberID, task.Priority, task.DueDate
		metadataChanged, assignmentChanged := false, false
		if request.AssigneeMemberID.Set {
			nextAssignee = request.AssigneeMemberID.Value
			assignmentChanged = !equalUUIDPointer(task.AssigneeMemberID, nextAssignee)
			metadataChanged = metadataChanged || assignmentChanged
		}
		if request.Priority != nil {
			if *request.Priority != TaskPriorityLow && *request.Priority != TaskPriorityMedium && *request.Priority != TaskPriorityHigh {
				return sharedErrors.ErrBadRequest
			}
			nextPriority = *request.Priority
			metadataChanged = metadataChanged || nextPriority != task.Priority
		}
		if request.DueDate.Set {
			nextDue, err = parseTaskDate(request.DueDate.Value)
			if err != nil {
				return err
			}
			metadataChanged = metadataChanged || !equalDatePointer(task.DueDate, nextDue)
		}
		if !contentChanged && !metadataChanged && (request.ToStatus == nil || *request.ToStatus == task.Status) {
			response, err = taskResponse(tx, &task, access, actorID)
			return err
		}
		mutation := TaskMutation{HasContentChange: contentChanged, HasMetadataChange: metadataChanged, AssignmentChanged: assignmentChanged, ToStatus: request.ToStatus, CloseReason: request.CloseReason, ReopenConfirmed: request.ReopenConfirmed}
		decision, err := EvaluateTaskMutation(accessPolicy(access), TaskPolicyState{Status: task.Status, AssigneeMemberID: task.AssigneeMemberID, Version: task.Version, HasPendingReview: task.Status == TaskStatusReview}, mutation)
		if err != nil {
			return err
		}
		if !access.CanManage && metadataChanged {
			return sharedErrors.ErrForbidden
		}
		if !access.CanManage && nextAssignee != nil && (access.Member == nil || *nextAssignee != access.Member.ID) {
			return sharedErrors.ErrForbidden
		}
		if (decision.ToStatus == TaskStatusInProgress || decision.ToStatus == TaskStatusReview) && nextAssignee != nil {
			if err := ensureTaskAssignee(tx, access.Project.ID, nextAssignee); err != nil {
				return err
			}
		}
		if (decision.ToStatus == TaskStatusInProgress || decision.ToStatus == TaskStatusReview) && nextAssignee == nil {
			return sharedErrors.ErrConflict
		}
		now := time.Now().UTC()
		operationID := uuid.New()
		var snapshot *WorkspaceProjectContextSnapshot
		if contentChanged || metadataChanged || decision.ToStatus != task.Status {
			snapshot, err = captureTaskContext(tx, access, now)
			if err != nil {
				return err
			}
		}
		if task.Status == TaskStatusReview && decision.ResolveReviewAs != "" {
			if err := resolvePendingReview(tx, access.Project.ID, task.ID, actorID, access.Member, decision.ResolveReviewAs, now); err != nil {
				return err
			}
		}
		eventOrder := int16(1)
		if contentChanged {
			task.ContentRevision++
			revision := WorkspaceTaskRevision{ID: uuid.New(), ProjectID: access.Project.ID, TaskID: task.ID, RevisionNumber: task.ContentRevision, Title: nextTitle, Description: nextDescription, ProjectContextSnapshotID: snapshot.ID, ChangedByUserID: actorID, ChangedByMemberID: memberIDOf(access), ChangedAt: now}
			if err := tx.Create(&revision).Error; err != nil {
				return err
			}
			if err := appendTaskEvent(tx, access, actorID, "task", task.ID, &task.ID, snapshot.ID, operationID, eventOrder, "task.content_updated", map[string]any{"task_id": task.ID, "revision_number": task.ContentRevision, "task_version_before": task.Version, "task_version_after": task.Version + 1, "title_changed": task.Title != nextTitle, "description_changed": task.Description != nextDescription}, now); err != nil {
				return err
			}
			eventOrder++
		}
		if metadataChanged && (nextPriority != task.Priority || !equalDatePointer(nextDue, task.DueDate)) {
			if err := appendTaskEvent(tx, access, actorID, "task", task.ID, &task.ID, snapshot.ID, operationID, eventOrder, "task.metadata_updated", map[string]any{"task_id": task.ID, "task_version_before": task.Version, "task_version_after": task.Version + 1, "from_priority": task.Priority, "to_priority": nextPriority, "from_due_date": task.DueDate, "to_due_date": nextDue}, now); err != nil {
				return err
			}
			eventOrder++
		}
		if assignmentChanged {
			if err := appendTaskEvent(tx, access, actorID, "task", task.ID, &task.ID, snapshot.ID, operationID, eventOrder, "task.assignment_changed", map[string]any{"task_id": task.ID, "task_version_before": task.Version, "task_version_after": task.Version + 1, "from_member_id": task.AssigneeMemberID, "to_member_id": nextAssignee}, now); err != nil {
				return err
			}
			eventOrder++
		}
		previousStatus := task.Status
		previousCloseReason := task.CloseReason
		task.Title, task.Description, task.AssigneeMemberID, task.Priority, task.DueDate = nextTitle, nextDescription, nextAssignee, nextPriority, nextDue
		task.Status = decision.ToStatus
		if decision.ToStatus == TaskStatusReview && decision.CreateReviewCycle {
			if err := createTaskReview(tx, access, &task, actorID, snapshot, now); err != nil {
				return err
			}
		}
		if decision.ToStatus == TaskStatusDone {
			task.CompletedAt, task.CompletedByUserID = &now, &actorID
		} else if decision.ToStatus == TaskStatusClosed {
			reason := strings.TrimSpace(*request.CloseReason)
			task.CloseReason = &reason
			task.ClosedAt, task.ClosedByUserID = &now, &actorID
		} else if previousStatus == TaskStatusDone || previousStatus == TaskStatusClosed {
			task.CompletedAt, task.CompletedByUserID, task.ClosedAt, task.ClosedByUserID, task.CloseReason = nil, nil, nil, nil, nil
		}
		task.Version++
		task.UpdatedAt = now
		task.UpdatedByUserID = actorID
		if err := tx.Save(&task).Error; err != nil {
			return err
		}
		if previousStatus != task.Status {
			payload := map[string]any{"task_id": task.ID, "from": previousStatus, "to": task.Status, "task_version_before": task.Version - 1, "task_version_after": task.Version, "review_cycle_id": pendingOrLatestCycleID(tx, task.ID), "reason_code": taskReasonCode(previousStatus, task.Status)}
			if decision.ToStatus == TaskStatusClosed {
				payload["close_reason"] = task.CloseReason
			}
			if previousStatus == TaskStatusClosed && previousCloseReason != nil {
				payload["reopened_close_reason"] = previousCloseReason
			}
			if err := appendTaskEvent(tx, access, actorID, "task", task.ID, &task.ID, snapshot.ID, operationID, eventOrder, "task.status_changed", payload, now); err != nil {
				return err
			}
		}
		response, err = taskResponse(tx, &task, access, actorID)
		return err
	})
	if err != nil {
		return nil, mapTaskPersistenceError(err)
	}
	return response, nil
}

func (service *TaskService) Transition(ctx context.Context, actorID uuid.UUID, slug string, taskID uuid.UUID, request TransitionTaskRequest) (*TaskResponse, error) {
	return service.Update(ctx, actorID, slug, taskID, UpdateTaskRequest{Version: request.Version, ToStatus: &request.ToStatus, CloseReason: request.CloseReason, ReopenConfirmed: request.ReopenConfirmed})
}

func (service *TaskService) Delete(ctx context.Context, actorID uuid.UUID, slug string, taskID uuid.UUID, version int64) (*TaskTombstoneResponse, error) {
	if version < 1 {
		return nil, sharedErrors.ErrBadRequest
	}
	var tombstone *TaskTombstoneResponse
	err := service.repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		access, err := lockTaskProject(tx, ctx, slug, actorID)
		if err != nil {
			return err
		}
		var task WorkspaceTask
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("project_id = ? AND id = ? AND deleted_at IS NULL", access.Project.ID, taskID).Take(&task).Error; err != nil {
			return err
		}
		if task.Version != version {
			return ErrTaskVersionConflict
		}
		decision, err := EvaluateTaskMutation(accessPolicy(access), TaskPolicyState{Status: task.Status, HasPendingReview: task.Status == TaskStatusReview}, TaskMutation{IsDelete: true, ToStatus: &task.Status})
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		snapshot, err := captureTaskContext(tx, access, now)
		if err != nil {
			return err
		}
		if task.Status == TaskStatusReview && decision.ResolveReviewAs != "" {
			if err := resolvePendingReview(tx, access.Project.ID, task.ID, actorID, access.Member, decision.ResolveReviewAs, now); err != nil {
				return err
			}
		}
		task.Version++
		task.DeletedAt, task.DeletedByUserID, task.UpdatedAt, task.UpdatedByUserID = &now, &actorID, now, actorID
		if err := tx.Save(&task).Error; err != nil {
			return err
		}
		if err := appendTaskEvent(tx, access, actorID, "task", task.ID, &task.ID, snapshot.ID, uuid.New(), 1, "task.deleted", map[string]any{"task_id": task.ID, "version": task.Version, "reason_code": "user_deleted"}, now); err != nil {
			return err
		}
		tombstone = &TaskTombstoneResponse{ID: task.ID, Version: task.Version, DeletedAt: now}
		return nil
	})
	if err != nil {
		return nil, mapTaskPersistenceError(err)
	}
	return tombstone, nil
}

func (service *TaskService) ListComments(ctx context.Context, actorID uuid.UUID, slug string, taskID uuid.UUID, page, limit int) ([]TaskCommentResponse, project.ProjectPaginationMeta, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}
	if page-1 > math.MaxInt/limit {
		return nil, project.ProjectPaginationMeta{}, sharedErrors.ErrBadRequest
	}
	access, err := service.repository.ResolveAccess(ctx, slug, actorID)
	if err != nil {
		return nil, project.ProjectPaginationMeta{}, mapTaskPersistenceError(err)
	}
	var task WorkspaceTask
	if err := service.repository.db.WithContext(ctx).Where("project_id = ? AND id = ? AND deleted_at IS NULL", access.Project.ID, taskID).Take(&task).Error; err != nil {
		return nil, project.ProjectPaginationMeta{}, mapTaskPersistenceError(err)
	}
	type commentRow struct {
		ID                      uuid.UUID  `gorm:"column:id"`
		TaskID                  uuid.UUID  `gorm:"column:task_id"`
		AuthorUserID            uuid.UUID  `gorm:"column:author_user_id"`
		AuthorMemberID          *uuid.UUID `gorm:"column:author_member_id"`
		Body                    string     `gorm:"column:body"`
		TaskRevisionID          uuid.UUID  `gorm:"column:task_revision_id"`
		ReviewCycleIDAtCreation *uuid.UUID `gorm:"column:review_cycle_id_at_creation"`
		CreatedAt               time.Time  `gorm:"column:created_at"`
		Username                string     `gorm:"column:username"`
		FullName                *string    `gorm:"column:full_name"`
		Avatar                  *string    `gorm:"column:avatar"`
		MembershipStatus        *string    `gorm:"column:membership_status"`
	}
	var rows []commentRow
	var total int64
	err = service.repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		q := tx.Model(&WorkspaceTaskComment{}).Where("project_id = ? AND task_id = ? AND deleted_at IS NULL", access.Project.ID, taskID)
		if err := q.Count(&total).Error; err != nil {
			return err
		}
		return tx.Table("workspace_task_comments AS c").
			Select("c.id,c.task_id,c.author_user_id,c.author_member_id,c.body,c.task_revision_id,c.review_cycle_id_at_creation,c.created_at,p.username,p.full_name,p.avatar,pm.status AS membership_status").
			Joins("LEFT JOIN profiles p ON p.id=c.author_user_id").
			Joins("LEFT JOIN project_members pm ON pm.id=c.author_member_id AND pm.project_id=c.project_id").
			Where("c.project_id = ? AND c.task_id = ? AND c.deleted_at IS NULL", access.Project.ID, taskID).
			Order("c.created_at ASC, c.id ASC").Offset((page - 1) * limit).Limit(limit).Scan(&rows).Error
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, project.ProjectPaginationMeta{}, err
	}
	items := make([]TaskCommentResponse, 0, len(rows))
	for _, row := range rows {
		name := row.Username
		if row.FullName != nil && strings.TrimSpace(*row.FullName) != "" {
			name = *row.FullName
		}
		membershipStatus := "left"
		if row.MembershipStatus != nil {
			membershipStatus = *row.MembershipStatus
		}
		items = append(items, TaskCommentResponse{ID: row.ID, TaskID: row.TaskID, Body: row.Body, Author: TaskAssigneeResponse{MembershipID: row.AuthorMemberID, UserID: row.AuthorUserID, Name: name, Avatar: row.Avatar, Status: membershipStatus}, TaskRevisionID: row.TaskRevisionID, ReviewCycleID: row.ReviewCycleIDAtCreation, CreatedAt: row.CreatedAt})
	}
	return items, project.ProjectPaginationMeta{Page: page, Limit: limit, Total: total, TotalPages: int((total + int64(limit) - 1) / int64(limit))}, nil
}

func (service *TaskService) CreateComment(ctx context.Context, actorID uuid.UUID, slug string, taskID uuid.UUID, key string, request CreateTaskCommentRequest) (*TaskCommentResponse, bool, error) {
	body := strings.TrimSpace(request.Body)
	if body == "" || len([]rune(body)) > 2000 || key == "" || len(key) > 100 {
		return nil, false, sharedErrors.ErrBadRequest
	}
	hashBytes := sha256.Sum256([]byte(body))
	requestHash := hex.EncodeToString(hashBytes[:])
	var response *TaskCommentResponse
	replayed := false
	err := service.repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		access, err := lockTaskProject(tx, ctx, slug, actorID)
		if err != nil {
			return err
		}
		var task WorkspaceTask
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("project_id = ? AND id = ? AND deleted_at IS NULL", access.Project.ID, taskID).Take(&task).Error; err != nil {
			return err
		}
		var existing WorkspaceTaskComment
		err = tx.Where("project_id = ? AND author_user_id = ? AND idempotency_key = ?", access.Project.ID, actorID, key).Take(&existing).Error
		if err == nil {
			if existing.TaskID != task.ID {
				return ErrTaskIdempotencyConflict
			}
			if existing.RequestHash == nil || *existing.RequestHash != requestHash {
				return ErrTaskIdempotencyConflict
			}
			var profile struct {
				Username string  `gorm:"column:username"`
				FullName *string `gorm:"column:full_name"`
				Avatar   *string `gorm:"column:avatar"`
			}
			_ = tx.Table("profiles").Select("username,full_name,avatar").Where("id=?", actorID).Take(&profile).Error
			name := profile.Username
			if profile.FullName != nil && strings.TrimSpace(*profile.FullName) != "" {
				name = *profile.FullName
			}
			response = &TaskCommentResponse{ID: existing.ID, TaskID: existing.TaskID, Body: existing.Body, Author: TaskAssigneeResponse{MembershipID: existing.AuthorMemberID, UserID: actorID, Name: name, Avatar: profile.Avatar, Status: "active"}, TaskRevisionID: existing.TaskRevisionID, ReviewCycleID: existing.ReviewCycleIDAtCreation, CreatedAt: existing.CreatedAt}
			replayed = true
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if !access.ActiveMember && !access.CanManage {
			return sharedErrors.ErrForbidden
		}
		var revision WorkspaceTaskRevision
		if err := tx.Where("task_id = ? AND revision_number = ?", task.ID, task.ContentRevision).Take(&revision).Error; err != nil {
			return err
		}
		var cycle WorkspaceTaskReviewCycle
		cycleID := (*uuid.UUID)(nil)
		if err := tx.Where("task_id = ? AND state = 'pending'", task.ID).Take(&cycle).Error; err == nil {
			cycleID = &cycle.ID
		}
		now := time.Now().UTC()
		snapshot, err := captureTaskContext(tx, access, now)
		if err != nil {
			return err
		}
		memberID := memberIDOf(access)
		id := uuid.New()
		keyCopy := key
		hashCopy := requestHash
		comment := WorkspaceTaskComment{ID: id, ProjectID: access.Project.ID, TaskID: task.ID, AuthorUserID: actorID, AuthorMemberID: memberID, Body: body, TaskRevisionID: revision.ID, ReviewCycleIDAtCreation: cycleID, IdempotencyKey: &keyCopy, RequestHash: &hashCopy, CreatedAt: now}
		if err := tx.Create(&comment).Error; err != nil {
			return err
		}
		if err := appendTaskEvent(tx, access, actorID, "comment", id, &task.ID, snapshot.ID, uuid.New(), 1, "task.comment_created", map[string]any{"task_id": task.ID, "comment_id": id, "revision_id": revision.ID, "review_cycle_id": cycleID}, now); err != nil {
			return err
		}
		var profile struct {
			Username string  `gorm:"column:username"`
			FullName *string `gorm:"column:full_name"`
			Avatar   *string `gorm:"column:avatar"`
		}
		_ = tx.Table("profiles").Select("username, full_name, avatar").Where("id = ?", actorID).Take(&profile).Error
		name := profile.Username
		if profile.FullName != nil && strings.TrimSpace(*profile.FullName) != "" {
			name = *profile.FullName
		}
		response = &TaskCommentResponse{ID: id, TaskID: task.ID, Body: body, Author: TaskAssigneeResponse{MembershipID: memberID, UserID: actorID, Name: name, Avatar: profile.Avatar, Status: "active"}, TaskRevisionID: revision.ID, ReviewCycleID: cycleID, CreatedAt: now}
		return nil
	})
	if err != nil {
		return nil, false, mapTaskPersistenceError(err)
	}
	return response, replayed, nil
}

func (service *TaskService) UpdateTaskAccess(ctx context.Context, actorID uuid.UUID, slug string, memberID uuid.UUID, request UpdateTaskAccessRequest) (*TaskAccessResponse, error) {
	if request.Version < 1 {
		return nil, sharedErrors.ErrBadRequest
	}
	var response *TaskAccessResponse
	err := service.repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		access, err := lockTaskProject(tx, ctx, slug, actorID)
		if err != nil {
			return err
		}
		if !access.IsOwner {
			return sharedErrors.ErrForbidden
		}
		var member project.ProjectMember
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND project_id = ? AND status = 'active' AND membership_role = 'contributor'", memberID, access.Project.ID).Take(&member).Error; err != nil {
			return err
		}
		if member.TaskAccessVersion != request.Version {
			return sharedErrors.ErrConflict
		}
		if member.TaskManagementGranted == request.TaskManagementGranted {
			response = &TaskAccessResponse{MemberID: member.ID, TaskManagementGranted: member.TaskManagementGranted, TaskAccessVersion: member.TaskAccessVersion}
			return nil
		}
		now := time.Now().UTC()
		snapshot, err := captureTaskContext(tx, access, now)
		if err != nil {
			return err
		}
		member.TaskManagementGranted = request.TaskManagementGranted
		member.TaskAccessVersion++
		if err := tx.Model(&project.ProjectMember{}).Where("id = ?", member.ID).Updates(map[string]any{"task_management_granted": member.TaskManagementGranted, "task_access_version": member.TaskAccessVersion, "updated_at": now}).Error; err != nil {
			return err
		}
		if err := appendTaskEvent(tx, access, actorID, "project_member", member.ID, nil, snapshot.ID, uuid.New(), 1, "member.task_access_changed", map[string]any{"member_id": member.ID, "task_management_granted": member.TaskManagementGranted, "task_access_version": member.TaskAccessVersion}, now); err != nil {
			return err
		}
		response = &TaskAccessResponse{MemberID: member.ID, TaskManagementGranted: member.TaskManagementGranted, TaskAccessVersion: member.TaskAccessVersion}
		return nil
	})
	if err != nil {
		return nil, mapTaskPersistenceError(err)
	}
	return response, nil
}

func resolvePendingReview(tx *gorm.DB, projectID, taskID, actorID uuid.UUID, actorMember *project.ProjectMember, result string, now time.Time) error {
	var cycle WorkspaceTaskReviewCycle
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("project_id = ? AND task_id = ? AND state = 'pending'", projectID, taskID).Take(&cycle).Error; err != nil {
		return err
	}
	cycle.State = result
	cycle.ResolvedByUserID = &actorID
	cycle.ResolvedAt = &now
	if actorMember != nil {
		cycle.ResolvedByMemberID = &actorMember.ID
	}
	if result == "accepted" || result == "changes_requested" {
		if actorMember != nil {
			cycle.ReviewerIsAssignee = boolPointer(cycle.AssigneeMemberIDAtRequest == actorMember.ID)
			cycle.ReviewerIsRequester = boolPointer(cycle.RequestedByMemberID != nil && *cycle.RequestedByMemberID == actorMember.ID)
		} else {
			cycle.ReviewerIsAssignee = boolPointer(false)
			cycle.ReviewerIsRequester = boolPointer(false)
		}
	}
	return tx.Save(&cycle).Error
}

func createTaskReview(tx *gorm.DB, access *TaskProjectAccess, task *WorkspaceTask, actorID uuid.UUID, snapshot *WorkspaceProjectContextSnapshot, now time.Time) error {
	if task.AssigneeMemberID == nil {
		return sharedErrors.ErrConflict
	}
	var revision WorkspaceTaskRevision
	if err := tx.Where("task_id = ? AND revision_number = ?", task.ID, task.ContentRevision).Take(&revision).Error; err != nil {
		return err
	}
	var latest *WorkspaceTaskReviewCycle
	var prior WorkspaceTaskReviewCycle
	if err := tx.Where("task_id = ?", task.ID).Order("cycle_number DESC").Take(&prior).Error; err == nil {
		latest = &prior
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	cycleNumber := int64(1)
	if latest != nil {
		cycleNumber = latest.CycleNumber + 1
	}
	cycle := WorkspaceTaskReviewCycle{ID: uuid.New(), ProjectID: access.Project.ID, TaskID: task.ID, CycleNumber: cycleNumber, TaskRevisionID: revision.ID, ProjectContextSnapshotID: snapshot.ID, AssigneeMemberIDAtRequest: *task.AssigneeMemberID, RequestedByUserID: actorID, RequestedByMemberID: memberIDOf(access), RequestedAt: now, State: "pending"}
	var member project.ProjectMember
	if err := tx.Where("id = ? AND project_id = ?", *task.AssigneeMemberID, access.Project.ID).Take(&member).Error; err != nil {
		return err
	}
	roleID, roleName, err := effectiveMemberRole(tx, member)
	if err != nil {
		return err
	}
	cycle.AssigneeRoleIDAtRequest, cycle.AssigneeRoleNameAtRequest = roleID, roleName
	return tx.Create(&cycle).Error
}

func taskResponse(db *gorm.DB, task *WorkspaceTask, access *TaskProjectAccess, actorID uuid.UUID) (*TaskResponse, error) {
	item, err := taskListItem(db, task, access, actorID)
	if err != nil {
		return nil, err
	}
	return &TaskResponse{TaskListItemResponse: item, Description: task.Description, CloseReason: task.CloseReason}, nil
}

func taskListItem(db *gorm.DB, task *WorkspaceTask, access *TaskProjectAccess, actorID uuid.UUID) (TaskListItemResponse, error) {
	item := TaskListItemResponse{ID: task.ID, ProjectID: task.ProjectID, Title: task.Title, DescriptionPreview: runePrefix(task.Description, 240), Status: task.Status, Priority: task.Priority, DueDate: projectDate(task.DueDate), Version: task.Version, ContentRevision: task.ContentRevision, CreatedAt: task.CreatedAt, UpdatedAt: task.UpdatedAt, NeedsReassignment: task.AssigneeMemberID == nil && (task.Status == TaskStatusInProgress || task.Status == TaskStatusReview)}
	var commentCount int64
	if err := db.Model(&WorkspaceTaskComment{}).Where("task_id = ? AND deleted_at IS NULL", task.ID).Count(&commentCount).Error; err != nil {
		return item, err
	}
	item.CommentCount = commentCount
	if task.AssigneeMemberID != nil {
		var member struct {
			ID       uuid.UUID `gorm:"column:id"`
			UserID   uuid.UUID `gorm:"column:user_id"`
			Status   string    `gorm:"column:status"`
			Username string    `gorm:"column:username"`
			FullName *string   `gorm:"column:full_name"`
			Avatar   *string   `gorm:"column:avatar"`
		}
		err := db.Table("project_members AS pm").Select("pm.id,pm.user_id,pm.status,p.username,p.full_name,p.avatar").Joins("LEFT JOIN profiles p ON p.id=pm.user_id").Where("pm.id=? AND pm.project_id=?", *task.AssigneeMemberID, task.ProjectID).Take(&member).Error
		if err == nil {
			name := member.Username
			if member.FullName != nil && strings.TrimSpace(*member.FullName) != "" {
				name = *member.FullName
			}
			memberID := member.ID
			item.Assignee = &TaskAssigneeResponse{MembershipID: &memberID, UserID: member.UserID, Name: name, Avatar: member.Avatar, Status: member.Status}
			item.NeedsReassignment = member.Status != "active" && (task.Status == TaskStatusInProgress || task.Status == TaskStatusReview)
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return item, err
		}
	}
	item.Permissions = taskPermissions(*task, access)
	var cycle WorkspaceTaskReviewCycle
	err := db.Where("task_id=?", task.ID).Order("cycle_number DESC").Take(&cycle).Error
	if err == nil {
		summary := &TaskReviewSummaryResponse{State: cycle.State, RequestedAt: cycle.RequestedAt, ResolvedAt: cycle.ResolvedAt, SelfReview: cycle.ReviewerIsAssignee != nil && *cycle.ReviewerIsAssignee}
		if cycle.ResolvedByUserID != nil && (cycle.State == "accepted" || cycle.State == "changes_requested") {
			var profile struct {
				Username string  `gorm:"column:username"`
				FullName *string `gorm:"column:full_name"`
			}
			if db.Table("profiles").Select("username,full_name").Where("id=?", *cycle.ResolvedByUserID).Take(&profile).Error == nil {
				name := profile.Username
				if profile.FullName != nil && strings.TrimSpace(*profile.FullName) != "" {
					name = *profile.FullName
				}
				summary.ReviewerName = &name
			}
		}
		item.ReviewSummary = summary
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return item, err
	}
	return item, nil
}

func taskListItems(db *gorm.DB, tasks []WorkspaceTask, access *TaskProjectAccess) ([]TaskListItemResponse, error) {
	items := make([]TaskListItemResponse, 0, len(tasks))
	if len(tasks) == 0 {
		return items, nil
	}
	taskIDs := make([]uuid.UUID, 0, len(tasks))
	assigneeIDs := make([]uuid.UUID, 0, len(tasks))
	for _, task := range tasks {
		taskIDs = append(taskIDs, task.ID)
		if task.AssigneeMemberID != nil {
			assigneeIDs = append(assigneeIDs, *task.AssigneeMemberID)
		}
	}
	type countRow struct {
		TaskID uuid.UUID `gorm:"column:task_id"`
		Count  int64     `gorm:"column:count"`
	}
	var commentCounts []countRow
	if err := db.Model(&WorkspaceTaskComment{}).Select("task_id, COUNT(*) AS count").Where("task_id IN ? AND deleted_at IS NULL", taskIDs).Group("task_id").Scan(&commentCounts).Error; err != nil {
		return nil, err
	}
	counts := make(map[uuid.UUID]int64, len(commentCounts))
	for _, row := range commentCounts {
		counts[row.TaskID] = row.Count
	}
	type memberRow struct {
		ID       uuid.UUID `gorm:"column:id"`
		UserID   uuid.UUID `gorm:"column:user_id"`
		Status   string    `gorm:"column:status"`
		Username string    `gorm:"column:username"`
		FullName *string   `gorm:"column:full_name"`
		Avatar   *string   `gorm:"column:avatar"`
	}
	assignees := make(map[uuid.UUID]memberRow, len(assigneeIDs))
	if len(assigneeIDs) > 0 {
		var rows []memberRow
		if err := db.Table("project_members AS pm").Select("pm.id,pm.user_id,pm.status,p.username,p.full_name,p.avatar").Joins("LEFT JOIN profiles p ON p.id=pm.user_id").Where("pm.project_id=? AND pm.id IN ?", access.Project.ID, assigneeIDs).Scan(&rows).Error; err != nil {
			return nil, err
		}
		for _, row := range rows {
			assignees[row.ID] = row
		}
	}
	var cycles []WorkspaceTaskReviewCycle
	if err := db.Raw("SELECT DISTINCT ON (task_id) * FROM workspace_task_review_cycles WHERE task_id IN ? ORDER BY task_id, cycle_number DESC", taskIDs).Scan(&cycles).Error; err != nil {
		return nil, err
	}
	latestCycles := make(map[uuid.UUID]WorkspaceTaskReviewCycle, len(tasks))
	reviewerIDs := make([]uuid.UUID, 0, len(cycles))
	for _, cycle := range cycles {
		if _, exists := latestCycles[cycle.TaskID]; exists {
			continue
		}
		latestCycles[cycle.TaskID] = cycle
		if cycle.ResolvedByUserID != nil && (cycle.State == "accepted" || cycle.State == "changes_requested") {
			reviewerIDs = append(reviewerIDs, *cycle.ResolvedByUserID)
		}
	}
	type profileRow struct {
		ID       uuid.UUID `gorm:"column:id"`
		Username string    `gorm:"column:username"`
		FullName *string   `gorm:"column:full_name"`
	}
	reviewers := make(map[uuid.UUID]profileRow, len(reviewerIDs))
	if len(reviewerIDs) > 0 {
		var profiles []profileRow
		if err := db.Table("profiles").Select("id,username,full_name").Where("id IN ?", reviewerIDs).Scan(&profiles).Error; err != nil {
			return nil, err
		}
		for _, profile := range profiles {
			reviewers[profile.ID] = profile
		}
	}
	for _, task := range tasks {
		item := TaskListItemResponse{ID: task.ID, ProjectID: task.ProjectID, Title: task.Title, DescriptionPreview: runePrefix(task.Description, 240), Status: task.Status, Priority: task.Priority, DueDate: projectDate(task.DueDate), Version: task.Version, ContentRevision: task.ContentRevision, CommentCount: counts[task.ID], CreatedAt: task.CreatedAt, UpdatedAt: task.UpdatedAt, NeedsReassignment: task.AssigneeMemberID == nil && (task.Status == TaskStatusInProgress || task.Status == TaskStatusReview)}
		if task.AssigneeMemberID != nil {
			if member, exists := assignees[*task.AssigneeMemberID]; exists {
				name := member.Username
				if member.FullName != nil && strings.TrimSpace(*member.FullName) != "" {
					name = *member.FullName
				}
				memberID := member.ID
				item.Assignee = &TaskAssigneeResponse{MembershipID: &memberID, UserID: member.UserID, Name: name, Avatar: member.Avatar, Status: member.Status}
				item.NeedsReassignment = member.Status != "active" && (task.Status == TaskStatusInProgress || task.Status == TaskStatusReview)
			}
		}
		item.Permissions = taskPermissions(task, access)
		if cycle, exists := latestCycles[task.ID]; exists {
			item.ReviewSummary = &TaskReviewSummaryResponse{State: cycle.State, RequestedAt: cycle.RequestedAt, ResolvedAt: cycle.ResolvedAt, SelfReview: cycle.ReviewerIsAssignee != nil && *cycle.ReviewerIsAssignee}
			if cycle.ResolvedByUserID != nil && (cycle.State == "accepted" || cycle.State == "changes_requested") {
				if profile, exists := reviewers[*cycle.ResolvedByUserID]; exists {
					name := profile.Username
					if profile.FullName != nil && strings.TrimSpace(*profile.FullName) != "" {
						name = *profile.FullName
					}
					item.ReviewSummary.ReviewerName = &name
				}
			}
		}
		items = append(items, item)
	}
	return items, nil
}

func taskPermissions(task WorkspaceTask, access *TaskProjectAccess) TaskPermissionsResponse {
	permissions := TaskPermissionsResponse{AllowedTransitions: []TaskStatus{}}
	if access.Project.Status != project.ProjectStatusInProgress {
		return permissions
	}
	manager := access.CanManage
	assigned := access.Member != nil && task.AssigneeMemberID != nil && access.Member.ID == *task.AssigneeMemberID
	permissions.EditContent = manager || (assigned && (task.Status == TaskStatusTodo || task.Status == TaskStatusInProgress))
	permissions.ManageMetadata = manager
	permissions.Delete = manager || (access.Member != nil && access.Member.TaskManagementGranted && isActiveTaskStatus(task.Status))
	permissions.Reopen = manager && (task.Status == TaskStatusDone || task.Status == TaskStatusClosed)
	if task.Status == TaskStatusTodo {
		if manager || assigned {
			permissions.AllowedTransitions = []TaskStatus{TaskStatusInProgress, TaskStatusReview}
		}
		if manager {
			permissions.AllowedTransitions = append(permissions.AllowedTransitions, TaskStatusClosed)
		}
	}
	if task.Status == TaskStatusInProgress {
		if manager || assigned {
			permissions.AllowedTransitions = []TaskStatus{TaskStatusTodo, TaskStatusReview}
		}
		if manager {
			permissions.AllowedTransitions = append(permissions.AllowedTransitions, TaskStatusClosed)
		}
	}
	if task.Status == TaskStatusReview {
		if access.ActiveMember {
			permissions.AllowedTransitions = []TaskStatus{TaskStatusDone, TaskStatusInProgress}
		}
		if manager || assigned {
			permissions.AllowedTransitions = append(permissions.AllowedTransitions, TaskStatusTodo)
		}
	}
	if task.Status == TaskStatusDone || task.Status == TaskStatusClosed {
		if manager {
			permissions.AllowedTransitions = []TaskStatus{TaskStatusTodo, TaskStatusInProgress}
		}
	}
	return permissions
}

func memberIDOf(access *TaskProjectAccess) *uuid.UUID {
	if access == nil || access.Member == nil {
		return nil
	}
	return &access.Member.ID
}
func equalUUIDPointer(a, b *uuid.UUID) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}
func equalDatePointer(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Format("2006-01-02") == b.Format("2006-01-02")
}
func boolPointer(value bool) *bool { return &value }
func runePrefix(value string, max int) string {
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	return string(runes[:max])
}
func projectDate(value *time.Time) *project.ProjectDate {
	if value == nil {
		return nil
	}
	date := project.ProjectDate{Time: *value}
	return &date
}
func taskReasonCode(from, to TaskStatus) string {
	if to == TaskStatusClosed {
		return "scope_changed"
	}
	if from == TaskStatusDone || from == TaskStatusClosed {
		return "reopened"
	}
	if to == TaskStatusDone {
		return "review_accepted"
	}
	return "status_changed"
}
func pendingOrLatestCycleID(tx *gorm.DB, taskID uuid.UUID) *uuid.UUID {
	var cycle WorkspaceTaskReviewCycle
	if err := tx.Where("task_id=?", taskID).Order("cycle_number DESC").Take(&cycle).Error; err != nil {
		return nil
	}
	return &cycle.ID
}
