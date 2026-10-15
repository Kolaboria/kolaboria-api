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

func (repository *GormRepository) LoadProjectInfo(ctx context.Context, slug string, actorID uuid.UUID) (*project.ProjectAggregate, bool, bool, error) {
	var aggregate *project.ProjectAggregate
	var activeMember, canInitiate bool
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		aggregate, err = project.NewProjectRepository(tx).FindBySlug(ctx, slug)
		if err != nil {
			return err
		}
		var membershipCount int64
		if err := tx.Model(&project.ProjectMember{}).
			Where("project_id = ? AND user_id = ? AND status = 'active'", aggregate.ID, actorID).
			Count(&membershipCount).Error; err != nil {
			return err
		}
		activeMember = membershipCount > 0
		if aggregate.OwnerID == nil && aggregate.InitiatorOrganizationID != nil {
			canInitiate, err = canInitiateOrganization(tx, ctx, actorID, *aggregate.InitiatorOrganizationID)
			if err != nil {
				return err
			}
		}
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return aggregate, activeMember, canInitiate, err
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
