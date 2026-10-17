package workspace

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/syahrulfahmi/kolaboria-api/internal/infrastructure/workspaceactivity"
	"github.com/syahrulfahmi/kolaboria-api/internal/modules/project"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type DiscussionProjectAccess struct {
	ID             uuid.UUID
	Title          string
	Status         project.ProjectStatus
	OwnerID        *uuid.UUID
	IsActiveMember bool
}

func (access DiscussionProjectAccess) IsOwner(actorID uuid.UUID) bool {
	return access.OwnerID != nil && *access.OwnerID == actorID
}

type DiscussionProjection struct {
	Discussion
	ProjectTitle     string  `gorm:"column:project_title"`
	AuthorName       string  `gorm:"column:author_name"`
	AuthorAvatar     *string `gorm:"column:author_avatar"`
	ReplyCount       int     `gorm:"column:reply_count"`
	ParticipantCount int     `gorm:"column:participant_count"`
}

type DiscussionReplyProjection struct {
	DiscussionReply
	AuthorName   string                      `gorm:"column:author_name"`
	AuthorAvatar *string                     `gorm:"column:author_avatar"`
	Replies      []DiscussionReplyProjection `gorm:"-"`
}

type DiscussionRepository interface {
	ResolveDiscussionProject(context.Context, string, uuid.UUID) (*DiscussionProjectAccess, error)
	ListDiscussions(context.Context, uuid.UUID, int, int) ([]DiscussionProjection, int64, error)
	GetDiscussion(context.Context, uuid.UUID, uuid.UUID) (*DiscussionProjection, error)
	ListDiscussionReplies(context.Context, uuid.UUID, uuid.UUID, int, int) ([]DiscussionReplyProjection, int64, error)
	WithDiscussionTransaction(context.Context, func(DiscussionTx) error) error
}

type DiscussionTx interface {
	AppendActivity(context.Context, workspaceactivity.Input) error
	ValidateDiscussionWriter(context.Context, uuid.UUID, uuid.UUID) (*DiscussionProjectAccess, error)
	GetDiscussion(context.Context, uuid.UUID, uuid.UUID, bool) (*DiscussionProjection, error)
	FindDiscussionByIdempotency(context.Context, uuid.UUID, uuid.UUID, string) (*Discussion, error)
	InsertDiscussion(context.Context, *Discussion) (bool, error)
	UpdateDiscussion(context.Context, uuid.UUID, uuid.UUID, string, string, time.Time) error
	SoftDeleteDiscussion(context.Context, uuid.UUID, uuid.UUID, time.Time) error
	SoftDeleteDiscussionReplies(context.Context, uuid.UUID, uuid.UUID, time.Time) error
	GetDiscussionReply(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, bool) (*DiscussionReplyProjection, error)
	FindReplyByIdempotency(context.Context, uuid.UUID, uuid.UUID, string) (*DiscussionReply, error)
	InsertDiscussionReply(context.Context, *DiscussionReply) (bool, error)
	UpdateDiscussionReply(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, string, time.Time) error
	SoftDeleteDiscussionReply(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, time.Time) error
	TouchDiscussion(context.Context, uuid.UUID, uuid.UUID, time.Time) error
}

type discussionGormRepository struct{ db *gorm.DB }

func NewDiscussionRepository(db *gorm.DB) DiscussionRepository {
	return &discussionGormRepository{db: db}
}

func (repository *discussionGormRepository) ResolveDiscussionProject(ctx context.Context, slug string, actorID uuid.UUID) (*DiscussionProjectAccess, error) {
	var projectRecord project.Project
	if err := repository.db.WithContext(ctx).Where("slug = ?", slug).Take(&projectRecord).Error; err != nil {
		return nil, err
	}
	var count int64
	if err := repository.db.WithContext(ctx).Model(&project.ProjectMember{}).
		Where("project_id = ? AND user_id = ? AND status = 'active'", projectRecord.ID, actorID).
		Count(&count).Error; err != nil {
		return nil, err
	}
	return &DiscussionProjectAccess{
		ID: projectRecord.ID, Title: projectRecord.Title, Status: projectRecord.Status, OwnerID: projectRecord.OwnerID,
		IsActiveMember: count > 0,
	}, nil
}

func (repository *discussionGormRepository) ListDiscussions(ctx context.Context, projectID uuid.UUID, page, limit int) ([]DiscussionProjection, int64, error) {
	var rows []DiscussionProjection
	var total int64
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&Discussion{}).Where("project_id = ? AND deleted_at IS NULL", projectID).Count(&total).Error; err != nil {
			return err
		}
		return discussionProjectionQuery(tx).
			Where("d.project_id = ? AND d.deleted_at IS NULL", projectID).
			Order("d.last_activity_at DESC, d.id DESC").
			Offset((page - 1) * limit).Limit(limit).Scan(&rows).Error
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return rows, total, err
}

func (repository *discussionGormRepository) GetDiscussion(ctx context.Context, projectID, discussionID uuid.UUID) (*DiscussionProjection, error) {
	return findDiscussionProjection(repository.db.WithContext(ctx), projectID, discussionID, false)
}

func (repository *discussionGormRepository) ListDiscussionReplies(ctx context.Context, projectID, discussionID uuid.UUID, page, limit int) ([]DiscussionReplyProjection, int64, error) {
	var rows []DiscussionReplyProjection
	var total int64
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		rows, total, err = listDiscussionReplyGroups(tx, projectID, discussionID, page, limit)
		return err
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return rows, total, err
}

func (repository *discussionGormRepository) WithDiscussionTransaction(ctx context.Context, fn func(DiscussionTx) error) error {
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&discussionGormTx{db: tx})
	})
}

type discussionGormTx struct{ db *gorm.DB }

func (tx *discussionGormTx) ValidateDiscussionWriter(ctx context.Context, projectID, actorID uuid.UUID) (*DiscussionProjectAccess, error) {
	var projectRecord project.Project
	if err := tx.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", projectID).Take(&projectRecord).Error; err != nil {
		return nil, err
	}
	var membership project.ProjectMember
	err := tx.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("project_id = ? AND user_id = ? AND status = 'active'", projectID, actorID).
		Take(&membership).Error
	isActiveMember := err == nil
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}
	access := &DiscussionProjectAccess{ID: projectRecord.ID, Title: projectRecord.Title, Status: projectRecord.Status, OwnerID: projectRecord.OwnerID, IsActiveMember: isActiveMember}
	if !canReadWorkspaceDiscussions(isActiveMember, access.IsOwner(actorID)) {
		return nil, gorm.ErrRecordNotFound
	}
	return access, nil
}

func (tx *discussionGormTx) GetDiscussion(ctx context.Context, projectID, discussionID uuid.UUID, lock bool) (*DiscussionProjection, error) {
	db := tx.db.WithContext(ctx)
	if lock {
		db = db.Clauses(clause.Locking{Strength: "UPDATE", Table: clause.Table{Name: "d"}})
	}
	return findDiscussionProjection(db, projectID, discussionID, lock)
}

func (tx *discussionGormTx) FindDiscussionByIdempotency(ctx context.Context, projectID, actorID uuid.UUID, key string) (*Discussion, error) {
	var discussion Discussion
	err := tx.db.WithContext(ctx).Where("project_id = ? AND author_id = ? AND idempotency_key = ?", projectID, actorID, key).Take(&discussion).Error
	return &discussion, err
}

func (tx *discussionGormTx) InsertDiscussion(ctx context.Context, discussion *Discussion) (bool, error) {
	result := tx.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(discussion)
	return result.RowsAffected == 1, result.Error
}

func (tx *discussionGormTx) UpdateDiscussion(ctx context.Context, projectID, discussionID uuid.UUID, title, body string, now time.Time) error {
	result := tx.db.WithContext(ctx).Model(&Discussion{}).
		Where("project_id = ? AND id = ? AND deleted_at IS NULL", projectID, discussionID).
		Updates(map[string]any{"title": title, "body": body, "updated_at": now, "last_activity_at": now})
	return requireDiscussionRows(result)
}

func (tx *discussionGormTx) SoftDeleteDiscussion(ctx context.Context, projectID, discussionID uuid.UUID, now time.Time) error {
	result := tx.db.WithContext(ctx).Model(&Discussion{}).
		Where("project_id = ? AND id = ? AND deleted_at IS NULL", projectID, discussionID).
		Updates(map[string]any{"title": "", "body": "", "deleted_at": now, "updated_at": now, "last_activity_at": now})
	return requireDiscussionRows(result)
}

func (tx *discussionGormTx) SoftDeleteDiscussionReplies(ctx context.Context, projectID, discussionID uuid.UUID, now time.Time) error {
	return tx.db.WithContext(ctx).Model(&DiscussionReply{}).
		Where("project_id = ? AND discussion_id = ? AND deleted_at IS NULL", projectID, discussionID).
		Updates(map[string]any{"body": "", "deleted_at": now, "updated_at": now}).Error
}

func (tx *discussionGormTx) GetDiscussionReply(ctx context.Context, projectID, discussionID, replyID uuid.UUID, lock bool) (*DiscussionReplyProjection, error) {
	db := tx.db.WithContext(ctx)
	if lock {
		db = db.Clauses(clause.Locking{Strength: "UPDATE", Table: clause.Table{Name: "r"}})
	}
	return findDiscussionReplyProjection(db, projectID, discussionID, replyID, lock)
}

func (tx *discussionGormTx) FindReplyByIdempotency(ctx context.Context, projectID, actorID uuid.UUID, key string) (*DiscussionReply, error) {
	var reply DiscussionReply
	err := tx.db.WithContext(ctx).Where("project_id = ? AND author_id = ? AND idempotency_key = ?", projectID, actorID, key).Take(&reply).Error
	return &reply, err
}

func (tx *discussionGormTx) InsertDiscussionReply(ctx context.Context, reply *DiscussionReply) (bool, error) {
	result := tx.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(reply)
	return result.RowsAffected == 1, result.Error
}

func (tx *discussionGormTx) UpdateDiscussionReply(ctx context.Context, projectID, discussionID, replyID uuid.UUID, body string, now time.Time) error {
	result := tx.db.WithContext(ctx).Model(&DiscussionReply{}).
		Where("project_id = ? AND discussion_id = ? AND id = ? AND deleted_at IS NULL", projectID, discussionID, replyID).
		Updates(map[string]any{"body": body, "updated_at": now})
	return requireDiscussionRows(result)
}

func (tx *discussionGormTx) SoftDeleteDiscussionReply(ctx context.Context, projectID, discussionID, replyID uuid.UUID, now time.Time) error {
	result := tx.db.WithContext(ctx).Model(&DiscussionReply{}).
		Where("project_id = ? AND discussion_id = ? AND id = ? AND deleted_at IS NULL", projectID, discussionID, replyID).
		Updates(map[string]any{"body": "", "deleted_at": now, "updated_at": now})
	return requireDiscussionRows(result)
}

func (tx *discussionGormTx) TouchDiscussion(ctx context.Context, projectID, discussionID uuid.UUID, now time.Time) error {
	result := tx.db.WithContext(ctx).Model(&Discussion{}).
		Where("project_id = ? AND id = ? AND deleted_at IS NULL", projectID, discussionID).
		Update("last_activity_at", now)
	return requireDiscussionRows(result)
}

type discussionProjectionRow struct {
	Discussion
	ProjectTitle     string  `gorm:"column:project_title"`
	AuthorName       string  `gorm:"column:author_name"`
	AuthorAvatar     *string `gorm:"column:author_avatar"`
	ReplyCount       int     `gorm:"column:reply_count"`
	ParticipantCount int     `gorm:"column:participant_count"`
}

func discussionProjectionQuery(db *gorm.DB) *gorm.DB {
	return db.Table("workspace_discussions AS d").
		Select(`d.*, pr.title AS project_title,
			COALESCE(NULLIF(BTRIM(p.full_name), ''), NULLIF(p.username, ''), 'Anggota') AS author_name,
			p.avatar AS author_avatar,
			(SELECT COUNT(*) FROM workspace_discussion_replies r WHERE r.discussion_id = d.id AND r.deleted_at IS NULL) AS reply_count,
			(SELECT COUNT(DISTINCT participants.author_id) FROM (
				SELECT d.author_id UNION SELECT r.author_id FROM workspace_discussion_replies r WHERE r.discussion_id = d.id AND r.deleted_at IS NULL
			) participants) AS participant_count`).
		Joins("JOIN projects pr ON pr.id = d.project_id").
		Joins("LEFT JOIN profiles p ON p.id = d.author_id")
}

func findDiscussionProjection(db *gorm.DB, projectID, discussionID uuid.UUID, lock bool) (*DiscussionProjection, error) {
	var row discussionProjectionRow
	query := discussionProjectionQuery(db).Where("d.project_id = ? AND d.id = ? AND d.deleted_at IS NULL", projectID, discussionID)
	if lock {
		query = query.Clauses(clause.Locking{Strength: "UPDATE", Table: clause.Table{Name: "d"}})
	}
	if err := query.Take(&row).Error; err != nil {
		return nil, err
	}
	return &DiscussionProjection{Discussion: row.Discussion, ProjectTitle: row.ProjectTitle, AuthorName: row.AuthorName, AuthorAvatar: row.AuthorAvatar, ReplyCount: row.ReplyCount, ParticipantCount: row.ParticipantCount}, nil
}

func findDiscussionReplyProjection(db *gorm.DB, projectID, discussionID, replyID uuid.UUID, lock bool) (*DiscussionReplyProjection, error) {
	var row struct {
		DiscussionReply
		AuthorName   string  `gorm:"column:author_name"`
		AuthorAvatar *string `gorm:"column:author_avatar"`
	}
	query := db.Table("workspace_discussion_replies AS r").
		Select("r.*, COALESCE(NULLIF(BTRIM(p.full_name), ''), NULLIF(p.username, ''), 'Anggota') AS author_name, p.avatar AS author_avatar").
		Joins("LEFT JOIN profiles p ON p.id = r.author_id").
		Where("r.project_id = ? AND r.discussion_id = ? AND r.id = ?", projectID, discussionID, replyID)
	if lock {
		query = query.Clauses(clause.Locking{Strength: "UPDATE", Table: clause.Table{Name: "r"}})
	}
	if err := query.Take(&row).Error; err != nil {
		return nil, err
	}
	return &DiscussionReplyProjection{DiscussionReply: row.DiscussionReply, AuthorName: row.AuthorName, AuthorAvatar: row.AuthorAvatar}, nil
}

func listDiscussionReplyGroups(db *gorm.DB, projectID, discussionID uuid.UUID, page, limit int) ([]DiscussionReplyProjection, int64, error) {
	rootQuery := db.Table("workspace_discussion_replies AS r").
		Select("r.*, COALESCE(NULLIF(BTRIM(p.full_name), ''), NULLIF(p.username, ''), 'Anggota') AS author_name, p.avatar AS author_avatar").
		Joins("LEFT JOIN profiles p ON p.id = r.author_id").
		Where("r.project_id = ? AND r.discussion_id = ? AND r.parent_reply_id IS NULL", projectID, discussionID).
		Where("(r.deleted_at IS NULL OR EXISTS (SELECT 1 FROM workspace_discussion_replies child WHERE child.parent_reply_id = r.id AND child.deleted_at IS NULL))")
	var total int64
	if err := rootQuery.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var roots []struct {
		DiscussionReply
		AuthorName   string  `gorm:"column:author_name"`
		AuthorAvatar *string `gorm:"column:author_avatar"`
	}
	if err := rootQuery.Order("r.created_at ASC, r.id ASC").Offset((page - 1) * limit).Limit(limit).Scan(&roots).Error; err != nil {
		return nil, 0, err
	}
	result := make([]DiscussionReplyProjection, 0, len(roots))
	rootIDs := make([]uuid.UUID, 0, len(roots))
	for _, row := range roots {
		result = append(result, DiscussionReplyProjection{DiscussionReply: row.DiscussionReply, AuthorName: row.AuthorName, AuthorAvatar: row.AuthorAvatar})
		rootIDs = append(rootIDs, row.ID)
	}
	if len(rootIDs) == 0 {
		return result, total, nil
	}
	var children []struct {
		DiscussionReply
		AuthorName   string  `gorm:"column:author_name"`
		AuthorAvatar *string `gorm:"column:author_avatar"`
	}
	if err := db.Table("workspace_discussion_replies AS r").
		Select("r.*, COALESCE(NULLIF(BTRIM(p.full_name), ''), NULLIF(p.username, ''), 'Anggota') AS author_name, p.avatar AS author_avatar").
		Joins("LEFT JOIN profiles p ON p.id = r.author_id").
		Where("r.project_id = ? AND r.discussion_id = ? AND r.parent_reply_id IN ?", projectID, discussionID, rootIDs).
		Order("r.created_at ASC, r.id ASC").Scan(&children).Error; err != nil {
		return nil, 0, err
	}
	childrenByRoot := make(map[uuid.UUID][]DiscussionReplyProjection, len(rootIDs))
	for _, child := range children {
		childrenByRoot[*child.ParentReplyID] = append(childrenByRoot[*child.ParentReplyID], DiscussionReplyProjection{DiscussionReply: child.DiscussionReply, AuthorName: child.AuthorName, AuthorAvatar: child.AuthorAvatar})
	}
	for index := range result {
		result[index].Replies = childrenByRoot[result[index].ID]
	}
	return result, total, nil
}

func requireDiscussionRows(result *gorm.DB) error {
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (tx *discussionGormTx) AppendActivity(ctx context.Context, input workspaceactivity.Input) error {
	return workspaceactivity.Record(tx.db.WithContext(ctx), input)
}
