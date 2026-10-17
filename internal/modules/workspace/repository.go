package workspace

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
	"github.com/syahrulfahmi/kolaboria-api/internal/modules/project"
	"gorm.io/gorm"
)

type GormRepository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *GormRepository {
	return &GormRepository{db: db}
}

func (repository *GormRepository) LoadProjectInfo(ctx context.Context, slug string, actorID uuid.UUID) (*ProjectInfoSnapshot, error) {
	snapshot := &ProjectInfoSnapshot{}
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		access, err := resolveWorkspaceReadAccess(tx, slug, actorID)
		if err != nil {
			return err
		}
		snapshot.Project, err = project.NewProjectRepository(tx).FindBySlug(ctx, slug)
		if err != nil {
			return err
		}
		snapshot.ActiveMember, snapshot.CanInitiate = access.ActiveMember, access.CanInitiate
		if access.Member != nil {
			snapshot.TaskAccess = TaskViewerAccess{ActiveMemberID: &access.Member.ID, TaskManagementGranted: access.Member.TaskManagementGranted}
		}
		snapshot.Closure.TaskSummary, err = loadProjectTaskSummary(tx, access.Project.ID)
		if err != nil {
			return err
		}
		completion, err := findProjectCompletion(tx, access.Project.ID)
		if err != nil {
			return err
		}
		snapshot.Closure.Completion = mapProjectCompletion(completion)
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return snapshot, err
}

func canInitiateOrganization(db *gorm.DB, ctx context.Context, actorID, organizationID uuid.UUID) (bool, error) {
	var count int64
	err := db.WithContext(ctx).Table("organization_members AS m").
		Joins("JOIN organizations o ON o.id = m.organization_id").
		Joins("JOIN users u ON u.id = m.user_id").
		Where("o.id = ? AND o.is_active = true AND m.user_id = ? AND m.is_active = true AND m.can_initiate_projects = true", organizationID, actorID).
		Where("u.is_active = true AND u.deleted_at IS NULL AND u.system_role = ?", "admin").
		Count(&count).Error
	return count > 0, err
}
