package workspace

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/syahrulfahmi/kolaboria-api/internal/modules/project"
	sharedErrors "github.com/syahrulfahmi/kolaboria-api/internal/shared/errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type WorkspaceProjectAccess struct {
	Project      project.Project
	Member       *project.ProjectMember
	ActiveMember bool
	CanManage    bool
	IsOwner      bool
	CanInitiate  bool
}

// Shared by task and activity readers. Unlike mutation guards, historical reads
// do not require an in-progress project or a currently active subject/author.
func resolveWorkspaceReadAccess(tx *gorm.DB, slug string, actorID uuid.UUID) (*WorkspaceProjectAccess, error) {
	access := &WorkspaceProjectAccess{}
	if err := tx.Where("slug = ?", slug).Take(&access.Project).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("%w: workspace project does not exist", sharedErrors.ErrNotFound)
		}
		return nil, err
	}
	var active bool
	if err := tx.Raw("SELECT EXISTS (SELECT 1 FROM users WHERE id = ? AND is_active = TRUE AND deleted_at IS NULL)", actorID).Scan(&active).Error; err != nil {
		return nil, err
	}
	if !active {
		return nil, sharedErrors.ErrUnauthorized
	}
	var member project.ProjectMember
	err := tx.Where("project_id = ? AND user_id = ? AND status = 'active'", access.Project.ID, actorID).Take(&member).Error
	if err == nil {
		access.Member = &member
		access.ActiveMember = true
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if access.Project.OwnerID == nil && access.Project.InitiatorOrganizationID != nil {
		access.CanInitiate, err = canInitiateOrganization(tx, tx.Statement.Context, actorID, *access.Project.InitiatorOrganizationID)
		if err != nil {
			return nil, err
		}
	}
	access.IsOwner = access.Project.OwnerID != nil && *access.Project.OwnerID == actorID
	access.CanManage = canManageWorkspace(actorID, &access.Project, access.CanInitiate)
	if !access.ActiveMember && !access.CanManage {
		return nil, fmt.Errorf("%w: actor cannot access this workspace", sharedErrors.ErrNotFound)
	}
	return access, nil
}

// Every Workspace writer locks the project before its task/member rows. Closure
// uses the same lock so no task mutation can race the final eligibility check.
func lockWorkspaceProject(tx *gorm.DB, ctx context.Context, slug string, actorID uuid.UUID) (*WorkspaceProjectAccess, error) {
	access := &WorkspaceProjectAccess{}
	if err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("slug = ?", slug).Take(&access.Project).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("%w: workspace project does not exist", sharedErrors.ErrNotFound)
		}
		return nil, err
	}
	var userActive bool
	if err := tx.WithContext(ctx).Raw("SELECT EXISTS (SELECT 1 FROM users WHERE id = ? AND is_active = TRUE AND deleted_at IS NULL)", actorID).Scan(&userActive).Error; err != nil {
		return nil, err
	}
	if !userActive {
		return nil, sharedErrors.ErrUnauthorized
	}
	var member project.ProjectMember
	err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("project_id = ? AND user_id = ? AND status = 'active'", access.Project.ID, actorID).Take(&member).Error
	if err == nil {
		access.Member = &member
		access.ActiveMember = true
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if access.Project.OwnerID == nil && access.Project.InitiatorOrganizationID != nil {
		access.CanInitiate, err = canInitiateOrganization(tx, ctx, actorID, *access.Project.InitiatorOrganizationID)
		if err != nil {
			return nil, err
		}
	}
	access.IsOwner = access.Project.OwnerID != nil && *access.Project.OwnerID == actorID
	access.CanManage = canManageWorkspace(actorID, &access.Project, access.CanInitiate)
	if !access.ActiveMember && !access.CanManage {
		return nil, fmt.Errorf("%w: actor is not an active project member or workspace manager", sharedErrors.ErrNotFound)
	}
	return access, nil
}
