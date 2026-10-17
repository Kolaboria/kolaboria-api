package workspace

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/syahrulfahmi/kolaboria-api/internal/modules/project"
	"gorm.io/gorm"
)

type OverviewReadOptions struct {
	StatementTimeout time.Duration
	LockTimeout      time.Duration
}

type OverviewSnapshot struct {
	Access             *WorkspaceProjectAccess
	Tasks              OverviewTaskSummaryResponse
	ActiveMemberCount  int64
	MyTasks            []OverviewTaskPreviewResponse
	DiscussionReadable bool
	Discussions        []OverviewDiscussionPreviewResponse
	Activities         []ActivityResponse
}

type OverviewRepository interface {
	LoadSnapshot(context.Context, string, uuid.UUID) (*OverviewSnapshot, error)
}

type overviewGormRepository struct {
	db      *gorm.DB
	options OverviewReadOptions
}

func NewOverviewRepository(db *gorm.DB, options OverviewReadOptions) OverviewRepository {
	return &overviewGormRepository{db: db, options: options}
}

func (repository *overviewGormRepository) LoadSnapshot(ctx context.Context, slug string, actorID uuid.UUID) (*OverviewSnapshot, error) {
	snapshot := &OverviewSnapshot{MyTasks: []OverviewTaskPreviewResponse{}, Discussions: []OverviewDiscussionPreviewResponse{}, Activities: []ActivityResponse{}}
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := repository.setLocalTimeouts(tx); err != nil {
			return err
		}
		access, err := resolveWorkspaceReadAccess(tx, slug, actorID)
		if err != nil {
			return err
		}
		snapshot.Access = access
		if err := loadOverviewTaskSummary(tx, access.Project.ID, &snapshot.Tasks); err != nil {
			return err
		}
		if err := loadOverviewTeamCount(tx, access.Project.ID, &snapshot.ActiveMemberCount); err != nil {
			return err
		}
		if access.Member != nil {
			if err := loadOverviewMyTasks(tx, access.Project.ID, access.Member.ID, &snapshot.MyTasks); err != nil {
				return err
			}
		}
		snapshot.DiscussionReadable = canReadWorkspaceDiscussions(access.ActiveMember, access.IsOwner)
		if snapshot.DiscussionReadable {
			if err := loadOverviewDiscussions(tx, access.Project.ID, &snapshot.Discussions); err != nil {
				return err
			}
		}
		activities, err := loadOverviewActivities(tx, access.Project.ID)
		if err != nil {
			return err
		}
		snapshot.Activities = activities
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, fmt.Errorf("load workspace overview snapshot: %w", err)
	}
	return snapshot, nil
}

func (repository *overviewGormRepository) setLocalTimeouts(tx *gorm.DB) error {
	if repository.options.StatementTimeout > 0 {
		if err := tx.Exec("SELECT set_config('statement_timeout', ?, true)", repository.options.StatementTimeout.String()).Error; err != nil {
			return fmt.Errorf("set workspace overview statement timeout: %w", err)
		}
	}
	if repository.options.LockTimeout > 0 {
		if err := tx.Exec("SELECT set_config('lock_timeout', ?, true)", repository.options.LockTimeout.String()).Error; err != nil {
			return fmt.Errorf("set workspace overview lock timeout: %w", err)
		}
	}
	return nil
}

func loadOverviewTaskSummary(tx *gorm.DB, projectID uuid.UUID, summary *OverviewTaskSummaryResponse) error {
	return tx.Table("workspace_tasks").Select(`
		COUNT(*) AS total_tasks,
		COUNT(*) FILTER (WHERE status = 'todo') AS todo_tasks,
		COUNT(*) FILTER (WHERE status = 'in_progress') AS in_progress_tasks,
		COUNT(*) FILTER (WHERE status = 'review') AS review_tasks,
		COUNT(*) FILTER (WHERE status = 'done') AS done_tasks,
		COUNT(*) FILTER (WHERE status = 'closed') AS closed_tasks,
		COUNT(*) FILTER (WHERE status IN ('todo', 'in_progress', 'review')) AS outstanding_tasks`).
		Where("project_id = ? AND deleted_at IS NULL", projectID).Scan(summary).Error
}

func loadOverviewTeamCount(tx *gorm.DB, projectID uuid.UUID, count *int64) error {
	return tx.Table("project_members pm").
		Where("pm.project_id = ? AND pm.status = 'active'", projectID).
		Where("EXISTS (SELECT 1 FROM users u WHERE u.id = pm.user_id AND u.is_active = TRUE AND u.deleted_at IS NULL)").
		Count(count).Error
}

type overviewTaskRow struct {
	ID       uuid.UUID    `gorm:"column:id"`
	Title    string       `gorm:"column:title"`
	Status   TaskStatus   `gorm:"column:status"`
	Priority TaskPriority `gorm:"column:priority"`
	DueDate  *time.Time   `gorm:"column:due_date"`
}

func loadOverviewMyTasks(tx *gorm.DB, projectID, memberID uuid.UUID, rows *[]OverviewTaskPreviewResponse) error {
	var selected []overviewTaskRow
	err := tx.Table("workspace_tasks").Select("id, title, status, priority, due_date").
		Where("project_id = ? AND assignee_member_id = ? AND deleted_at IS NULL AND status IN ('todo', 'in_progress', 'review')", projectID, memberID).
		Order("due_date ASC NULLS LAST, CASE priority WHEN 'high' THEN 0 WHEN 'medium' THEN 1 ELSE 2 END, created_at ASC, id ASC").
		Limit(WorkspaceOverviewTaskPreviewLimit).Scan(&selected).Error
	if err != nil {
		return err
	}
	for _, row := range selected {
		var dueDate *project.ProjectDate
		if row.DueDate != nil {
			dueDate = &project.ProjectDate{Time: *row.DueDate}
		}
		*rows = append(*rows, OverviewTaskPreviewResponse{ID: row.ID, Title: row.Title, Status: row.Status, Priority: row.Priority, DueDate: dueDate})
	}
	return nil
}

type overviewDiscussionRow struct {
	ID             uuid.UUID `gorm:"column:id"`
	Title          string    `gorm:"column:title"`
	AuthorID       uuid.UUID `gorm:"column:author_id"`
	AuthorName     string    `gorm:"column:author_name"`
	AuthorAvatar   *string   `gorm:"column:author_avatar"`
	CreatedAt      time.Time `gorm:"column:created_at"`
	LastActivityAt time.Time `gorm:"column:last_activity_at"`
}

func loadOverviewDiscussions(tx *gorm.DB, projectID uuid.UUID, rows *[]OverviewDiscussionPreviewResponse) error {
	var selected []overviewDiscussionRow
	err := tx.Table("workspace_discussions d").Select(`d.id, d.title, d.author_id,
		COALESCE(NULLIF(BTRIM(p.full_name), ''), NULLIF(p.username, ''), 'Anggota') AS author_name,
		p.avatar AS author_avatar, d.created_at, d.last_activity_at`).
		Joins("LEFT JOIN profiles p ON p.id = d.author_id").
		Where("d.project_id = ? AND d.deleted_at IS NULL", projectID).
		Order("d.last_activity_at DESC, d.id DESC").Limit(WorkspaceOverviewDiscussionPreviewLimit).Scan(&selected).Error
	if err != nil {
		return err
	}
	for _, row := range selected {
		*rows = append(*rows, OverviewDiscussionPreviewResponse{ID: row.ID, Title: row.Title,
			Author:    DiscussionAuthorResponse{ID: row.AuthorID.String(), Name: row.AuthorName, AvatarURL: row.AuthorAvatar},
			CreatedAt: row.CreatedAt, LastActivityAt: row.LastActivityAt})
	}
	return nil
}
