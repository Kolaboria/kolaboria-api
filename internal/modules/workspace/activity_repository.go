package workspace

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ActivityProjection struct {
	WorkspaceActivityEvent
	ActorName        string
	ActorAvatar      *string
	SubjectLabel     string
	CurrentTaskTitle string
	SubjectAvailable bool
	SubjectDeleted   bool
	SelfReview       *bool
}

type ActivityRepository interface {
	LoadPage(context.Context, string, uuid.UUID, int, int) ([]ActivityProjection, int64, error)
}

type activityGormRepository struct{ db *gorm.DB }

func NewActivityRepository(db *gorm.DB) ActivityRepository { return &activityGormRepository{db: db} }

// The payload is internal but older/malformed records must not turn a read into
// a UUID cast exception. Keys here are constants supplied by this repository.
func activityPayloadUUID(key string) string {
	value := "e.payload->>'" + key + "'"
	return "CASE WHEN " + value + " ~ '^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$' THEN (" + value + ")::uuid END"
}

func (repository *activityGormRepository) LoadPage(ctx context.Context, slug string, actorID uuid.UUID, page, limit int) ([]ActivityProjection, int64, error) {
	rows := []ActivityProjection{}
	var total int64
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		access, err := resolveWorkspaceReadAccess(tx, slug, actorID)
		if err != nil {
			return err
		}
		types := visibleActivityTypes()
		base := func() *gorm.DB {
			return tx.Table("workspace_activity_events e").Where("e.project_id = ? AND e.event_type IN ?", access.Project.ID, types)
		}
		if err := base().Count(&total).Error; err != nil {
			return err
		}
		// Paginate before resolving presentation sources. Every join is optional
		// and project-scoped so deleted resources cannot hide or replace events.
		query := tx.Table("(?) AS e", base().Select("e.*").
			Order("e.occurred_at DESC, e.operation_id DESC, e.event_order DESC, e.id DESC").Offset((page-1)*limit).Limit(limit))
		return workspaceActivityProjectionQuery(query).Scan(&rows).Error
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, 0, fmt.Errorf("load workspace activities: %w", err)
	}
	return rows, total, nil
}

func workspaceActivityProjectionQuery(query *gorm.DB) *gorm.DB {
	return query.Select(`e.*,
			CASE WHEN u.deleted_at IS NULL THEN COALESCE(NULLIF(p.full_name, ''), NULLIF(p.username, ''), 'Pengguna tidak tersedia') ELSE 'Pengguna tidak tersedia' END AS actor_name,
			CASE WHEN u.deleted_at IS NULL THEN p.avatar END AS actor_avatar,
			t.title AS current_task_title,
			CASE WHEN e.task_id IS NOT NULL THEN COALESCE(NULLIF(e.payload->>'title_snapshot', ''), rev.title, '')
			WHEN e.subject_type IN ('discussion', 'discussion_reply') THEN COALESCE(NULLIF(e.payload->>'title_snapshot', ''), d.title, 'diskusi terkait')
			WHEN e.subject_type = 'project_member' THEN COALESCE(NULLIF(mp.full_name, ''), NULLIF(mp.username, ''), 'anggota proyek')
			ELSE 'proyek' END AS subject_label,
			CASE WHEN e.task_id IS NOT NULL THEN t.id IS NOT NULL
			WHEN e.subject_type = 'discussion' THEN d.id IS NOT NULL
			WHEN e.subject_type = 'discussion_reply' THEN dr.id IS NOT NULL AND d.id IS NOT NULL
			WHEN e.subject_type = 'project_member' THEN pm.id IS NOT NULL ELSE TRUE END AS subject_available,
			(t.deleted_at IS NOT NULL OR d.deleted_at IS NOT NULL OR dr.deleted_at IS NOT NULL) AS subject_deleted,
			rc.reviewer_is_assignee AS self_review`).
		Joins("LEFT JOIN users u ON u.id = e.actor_user_id").
		Joins("LEFT JOIN profiles p ON p.id = e.actor_user_id AND u.deleted_at IS NULL").
		Joins("LEFT JOIN workspace_tasks t ON t.id = e.task_id AND t.project_id = e.project_id").
		Joins("LEFT JOIN workspace_task_revisions rev ON rev.id = " + activityPayloadUUID("revision_id") + " AND rev.task_id = e.task_id AND rev.project_id = e.project_id").
		Joins("LEFT JOIN workspace_task_review_cycles rc ON rc.id = " + activityPayloadUUID("review_cycle_id") + " AND rc.task_id = e.task_id AND rc.project_id = e.project_id").
		Joins("LEFT JOIN workspace_discussion_replies dr ON e.subject_type = 'discussion_reply' AND dr.id = e.subject_id AND dr.project_id = e.project_id").
		Joins("LEFT JOIN workspace_discussions d ON d.project_id = e.project_id AND d.id = CASE WHEN e.subject_type = 'discussion' THEN e.subject_id ELSE dr.discussion_id END").
		Joins("LEFT JOIN project_members pm ON e.subject_type = 'project_member' AND pm.id = e.subject_id AND pm.project_id = e.project_id").
		Joins("LEFT JOIN users mu ON mu.id = pm.user_id AND mu.deleted_at IS NULL").
		Joins("LEFT JOIN profiles mp ON mp.id = mu.id").
		Order("e.occurred_at DESC, e.operation_id DESC, e.event_order DESC, e.id DESC")
}
