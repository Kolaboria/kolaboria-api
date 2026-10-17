package workspace

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/syahrulfahmi/kolaboria-api/internal/infrastructure/workspaceactivity"
	"github.com/syahrulfahmi/kolaboria-api/internal/modules/project"
	sharedErrors "github.com/syahrulfahmi/kolaboria-api/internal/shared/errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type TaskProjectAccess = WorkspaceProjectAccess

type TaskRepository struct{ db *gorm.DB }

func NewTaskRepository(db *gorm.DB) *TaskRepository { return &TaskRepository{db: db} }

func (repository *TaskRepository) ResolveAccess(ctx context.Context, slug string, actorID uuid.UUID) (*TaskProjectAccess, error) {
	var access *TaskProjectAccess
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		access, err = resolveWorkspaceReadAccess(tx, slug, actorID)
		return err
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return access, err
}

func lockTaskProject(tx *gorm.DB, ctx context.Context, slug string, actorID uuid.UUID) (*TaskProjectAccess, error) {
	access, err := lockWorkspaceProject(tx, ctx, slug, actorID)
	if err != nil {
		return nil, err
	}
	if access.Project.Status != project.ProjectStatusInProgress {
		return nil, ErrTaskProjectNotWritable
	}
	return access, nil
}

func accessPolicy(access *TaskProjectAccess) TaskAccess {
	policy := TaskAccess{
		ActorActive:           access != nil,
		ActiveMember:          access != nil && access.ActiveMember,
		IsManager:             access != nil && access.CanManage,
		TaskManagementGranted: access != nil && access.Member != nil && access.Member.TaskManagementGranted,
	}
	if access != nil && access.Member != nil {
		memberID := access.Member.ID
		policy.MemberID = &memberID
	}
	return policy
}

func ensureTaskAssignee(tx *gorm.DB, projectID uuid.UUID, memberID *uuid.UUID) error {
	if memberID == nil {
		return nil
	}
	var member project.ProjectMember
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND project_id = ? AND status = 'active'", *memberID, projectID).
		Where("EXISTS (SELECT 1 FROM users WHERE users.id = project_members.user_id AND users.is_active = TRUE AND users.deleted_at IS NULL)").
		Take(&member).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return sharedErrors.ErrBadRequest
		}
		return err
	}
	return nil
}

func captureTaskContext(tx *gorm.DB, access *TaskProjectAccess, now time.Time) (*WorkspaceProjectContextSnapshot, error) {
	return workspaceactivity.CaptureContext(tx, access.Project.ID, now)
}

func appendTaskEvent(tx *gorm.DB, access *TaskProjectAccess, actorID uuid.UUID, subjectType string, subjectID uuid.UUID, taskID *uuid.UUID, snapshotID uuid.UUID, operationID uuid.UUID, order int16, eventType string, payload any, now time.Time) error {
	// Bind every new task fact to the content revision that existed at this
	// operation, including metadata/status events and soft-deleted tasks.
	if taskID != nil {
		var revision WorkspaceTaskRevision
		if err := tx.Where("project_id = ? AND task_id = ?", access.Project.ID, *taskID).Order("revision_number DESC").Take(&revision).Error; err != nil {
			return err
		}
		if fields, ok := payload.(map[string]any); ok {
			fields["revision_id"] = revision.ID
			fields["title_snapshot"] = revision.Title
		}
	}
	return workspaceactivity.Record(tx, workspaceactivity.Input{
		ProjectID: access.Project.ID, ActorID: actorID, SubjectType: subjectType, SubjectID: subjectID,
		TaskID: taskID, SnapshotID: snapshotID, OperationID: operationID, Order: order, Type: eventType, Payload: payload, At: now,
	})
}

func effectiveMemberRole(tx *gorm.DB, member project.ProjectMember) (*uuid.UUID, *string, error) {
	if member.ContributionRoleID != nil {
		var role struct {
			Name string `gorm:"column:name"`
		}
		if err := tx.Table("contribution_roles").Select("name").Where("id = ?", *member.ContributionRoleID).Take(&role).Error; err != nil {
			return nil, nil, fmt.Errorf("load member contribution role: %w", err)
		}
		return member.ContributionRoleID, &role.Name, nil
	}
	if member.ProjectRoleID == nil {
		return nil, nil, nil
	}
	var role struct {
		ID                 uuid.UUID  `gorm:"column:id"`
		ContributionRoleID *uuid.UUID `gorm:"column:contribution_role_id"`
	}
	if err := tx.Table("project_roles").Select("id,name,contribution_role_id").Where("id = ? AND project_id = ?", *member.ProjectRoleID, member.ProjectID).Take(&role).Error; err != nil {
		return nil, nil, fmt.Errorf("load member project role: %w", err)
	}
	if role.ContributionRoleID != nil {
		var contribution struct {
			Name string `gorm:"column:name"`
		}
		if err := tx.Table("contribution_roles").Select("name").Where("id = ?", *role.ContributionRoleID).Take(&contribution).Error; err != nil {
			return nil, nil, fmt.Errorf("load project role contribution label: %w", err)
		}
		return role.ContributionRoleID, &contribution.Name, nil
	}
	return &role.ID, nil, nil
}

func normalizedTaskTitle(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || len([]rune(value)) > 160 {
		return "", sharedErrors.ErrBadRequest
	}
	return value, nil
}

func normalizedTaskDescription(value string) (string, error) {
	value = strings.TrimSpace(value)
	if len([]rune(value)) > 20000 {
		return "", sharedErrors.ErrBadRequest
	}
	return value, nil
}

func parseTaskDate(value *string) (*time.Time, error) {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil, nil
	}
	parsed, err := time.Parse("2006-01-02", strings.TrimSpace(*value))
	if err != nil {
		return nil, sharedErrors.ErrBadRequest
	}
	return &parsed, nil
}

func mapTaskPersistenceError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("%w: %v", sharedErrors.ErrNotFound, err)
	}
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "unique") || strings.Contains(message, "duplicate key") || strings.Contains(message, "serialization") {
		return fmt.Errorf("%w: task changed concurrently", sharedErrors.ErrConflict)
	}
	return err
}
