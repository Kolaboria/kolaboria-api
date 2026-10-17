package workspace

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
	"github.com/syahrulfahmi/kolaboria-api/internal/modules/project"
	"gorm.io/gorm"
)

type TeamMemberRecord struct {
	ID                    uuid.UUID    `gorm:"column:id"`
	UserID                uuid.UUID    `gorm:"column:user_id"`
	MembershipRole        string       `gorm:"column:membership_role"`
	MembershipStatus      string       `gorm:"column:membership_status"`
	JoinedAt              sql.NullTime `gorm:"column:joined_at"`
	ProfileID             *uuid.UUID   `gorm:"column:profile_id"`
	Username              *string      `gorm:"column:username"`
	FullName              *string      `gorm:"column:full_name"`
	Headline              *string      `gorm:"column:headline"`
	Avatar                *string      `gorm:"column:avatar"`
	IsVerified            bool         `gorm:"column:is_verified"`
	ProjectRoleID         *uuid.UUID   `gorm:"column:project_role_id"`
	ProjectRoleName       *string      `gorm:"column:project_role_name"`
	ContributionRoleID    *uuid.UUID   `gorm:"column:contribution_role_id"`
	ContributionName      *string      `gorm:"column:contribution_role_name"`
	ActiveTaskCount       int64        `gorm:"column:active_task_count"`
	TaskManagementGranted bool         `gorm:"column:task_management_granted"`
	TaskAccessVersion     int64        `gorm:"column:task_access_version"`
}

type TeamPageSnapshot struct {
	Project      *project.Project
	ActiveMember bool
	CanInitiate  bool
	Total        int64
	Members      []TeamMemberRecord
}

type TeamRepository interface {
	LoadTeamPage(context.Context, string, uuid.UUID, int, int) (*TeamPageSnapshot, error)
}

type GormTeamRepository struct{ db *gorm.DB }

func NewTeamRepository(db *gorm.DB) *GormTeamRepository { return &GormTeamRepository{db: db} }

func (repository *GormTeamRepository) LoadTeamPage(ctx context.Context, slug string, actorID uuid.UUID, page, limit int) (*TeamPageSnapshot, error) {
	snapshot := &TeamPageSnapshot{Members: make([]TeamMemberRecord, 0)}
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var record project.Project
		if err := tx.Where("slug = ?", slug).Take(&record).Error; err != nil {
			return err
		}
		snapshot.Project = &record

		var actorMembershipCount int64
		if err := tx.Model(&project.ProjectMember{}).
			Where("project_id = ? AND user_id = ? AND status = 'active'", record.ID, actorID).
			Count(&actorMembershipCount).Error; err != nil {
			return err
		}
		snapshot.ActiveMember = actorMembershipCount > 0
		var actorActive bool
		if err := tx.Raw("SELECT EXISTS (SELECT 1 FROM users WHERE id = ? AND is_active = TRUE AND deleted_at IS NULL)", actorID).Scan(&actorActive).Error; err != nil {
			return err
		}
		if !actorActive {
			return gorm.ErrRecordNotFound
		}
		if record.OwnerID == nil && record.InitiatorOrganizationID != nil {
			var err error
			snapshot.CanInitiate, err = canInitiateOrganization(tx, ctx, actorID, *record.InitiatorOrganizationID)
			if err != nil {
				return err
			}
		}

		members := tx.Table("project_members AS pm").
			Joins("LEFT JOIN profiles p ON p.id = pm.user_id").
			Joins("LEFT JOIN project_roles pr ON pr.id = pm.project_role_id").
			Joins("LEFT JOIN contribution_roles pcr ON pcr.id = pr.contribution_role_id").
			Joins("LEFT JOIN contribution_roles mcr ON mcr.id = pm.contribution_role_id").
			Where("pm.project_id = ? AND pm.status = 'active'", record.ID).
			Where("EXISTS (SELECT 1 FROM users u WHERE u.id = pm.user_id AND u.is_active = true AND u.deleted_at IS NULL)")
		if err := members.Count(&snapshot.Total).Error; err != nil {
			return err
		}

		return members.Select(`pm.id, pm.user_id, pm.membership_role, pm.status AS membership_status, pm.joined_at,
			p.id AS profile_id, p.username, p.full_name, p.headline, p.avatar, COALESCE(p.is_verified, false) AS is_verified,
			pr.id AS project_role_id, pcr.name AS project_role_name,
			pm.contribution_role_id, mcr.name AS contribution_role_name,
			pm.task_management_granted, pm.task_access_version,
			(SELECT COUNT(*) FROM workspace_tasks wt WHERE wt.project_id = pm.project_id AND wt.assignee_member_id = pm.id AND wt.deleted_at IS NULL AND wt.status IN ('todo','in_progress','review')) AS active_task_count`).
			Order("CASE WHEN pm.membership_role = 'owner' THEN 0 ELSE 1 END, pm.joined_at ASC, pm.id ASC").
			Limit(limit).Offset((page - 1) * limit).Scan(&snapshot.Members).Error
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, err
	}
	return snapshot, nil
}
