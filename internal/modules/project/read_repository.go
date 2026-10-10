package project

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func (r *gormProjectRepository) ListPublic(ctx context.Context, filter ProjectListFilter) ([]ProjectAggregate, int64, error) {
	query := r.db.WithContext(ctx).Model(&Project{}).
		Where("visibility = 'public' AND status IN ?", []ProjectStatus{ProjectStatusOpen, ProjectStatusAwaitingOwner, ProjectStatusInProgress, ProjectStatusCompleted})
	if search := strings.TrimSpace(filter.Search); search != "" {
		pattern := "%" + strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_").Replace(search) + "%"
		query = query.Where("(title ILIKE ? OR summary ILIKE ?)", pattern, pattern)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var projects []Project
	if err := query.Order("updated_at DESC, id ASC").Offset((filter.Page - 1) * filter.Limit).Limit(filter.Limit).Find(&projects).Error; err != nil {
		return nil, 0, err
	}
	return loadProjectPage(r.db.WithContext(ctx), projects, total)
}

func (r *gormProjectRepository) ListMine(ctx context.Context, actorID uuid.UUID, filter ProjectListFilter) ([]ProjectAggregate, int64, error) {
	query := r.db.WithContext(ctx).Model(&Project{})
	switch filter.Scope {
	case "owned":
		query = query.Where("(projects.owner_id = ? OR EXISTS (SELECT 1 FROM project_members pm WHERE pm.project_id = projects.id AND pm.user_id = ? AND pm.status = 'active'))", actorID, actorID)
	case "initiated":
		query = query.Where(`projects.creation_mode = 'organization_initiated'
			AND projects.created_by_user_id = ?
			AND EXISTS (
				SELECT 1 FROM organizations o
				JOIN organization_members om ON om.organization_id = o.id
				JOIN users u ON u.id = om.user_id
				WHERE o.id = projects.initiator_organization_id AND o.is_active = true
				AND om.user_id = ? AND om.is_active = true AND om.can_initiate_projects = true
				AND u.is_active = true AND u.deleted_at IS NULL AND u.system_role = 'admin'
			)`, actorID, actorID)
	default:
		return nil, 0, gorm.ErrInvalidData
	}
	if search := strings.TrimSpace(filter.Search); search != "" {
		pattern := "%" + strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_").Replace(search) + "%"
		query = query.Where("(projects.title ILIKE ? OR projects.summary ILIKE ?)", pattern, pattern)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var projects []Project
	if err := query.Order("projects.updated_at DESC, projects.id ASC").Offset((filter.Page - 1) * filter.Limit).Limit(filter.Limit).Find(&projects).Error; err != nil {
		return nil, 0, err
	}
	return loadProjectPage(r.db.WithContext(ctx), projects, total)
}

func loadProjectPage(db *gorm.DB, projects []Project, total int64) ([]ProjectAggregate, int64, error) {
	result := make([]ProjectAggregate, 0, len(projects))
	for _, project := range projects {
		aggregate, err := loadProjectAggregate(db, "projects.id = ?", project.ID)
		if err != nil {
			return nil, 0, err
		}
		result = append(result, *aggregate)
	}
	return result, total, nil
}

func (r *gormProjectRepository) FindByID(ctx context.Context, id uuid.UUID) (*ProjectAggregate, error) {
	return loadProjectAggregate(r.db.WithContext(ctx), "projects.id = ?", id)
}

func (r *gormProjectRepository) FindBySlug(ctx context.Context, slug string) (*ProjectAggregate, error) {
	return loadProjectAggregate(r.db.WithContext(ctx), "projects.slug = ?", slug)
}

func (r *gormProjectRepository) FindViewerApplication(ctx context.Context, projectID, applicantID uuid.UUID) (*ViewerApplicationResponse, error) {
	var application ViewerApplicationResponse
	err := r.db.WithContext(ctx).Table("project_applications").
		Select("id, status, applied_at").
		Where("project_id = ? AND applicant_user_id = ? AND status IN ?", projectID, applicantID, []string{"pending", "accepted"}).
		Order("applied_at DESC").Take(&application).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &application, nil
}

func loadProjectAggregate(db *gorm.DB, query string, args ...any) (*ProjectAggregate, error) {
	var aggregate ProjectAggregate
	if err := db.Model(&Project{}).Where(query, args...).Take(&aggregate.Project).Error; err != nil {
		return nil, err
	}
	var err error
	if aggregate.OwnerID != nil {
		aggregate.OwnerProfile, err = loadPublicProfile(db, *aggregate.OwnerID)
		if err != nil {
			return nil, err
		}
	}
	if aggregate.InitiatorUserID != nil {
		aggregate.InitiatorProfile, err = loadPublicProfile(db, *aggregate.InitiatorUserID)
		if err != nil {
			return nil, err
		}
	}
	if aggregate.InitiatorOrganizationID != nil {
		var organization ProjectOrganizationResponse
		loadErr := db.Table("organizations").Select("id, name, slug").
			Where("id = ?", *aggregate.InitiatorOrganizationID).Take(&organization).Error
		if loadErr != nil && !errors.Is(loadErr, gorm.ErrRecordNotFound) {
			return nil, loadErr
		}
		if loadErr == nil {
			aggregate.InitiatorOrganization = &organization
		}
	}
	aggregate.Roles = make([]ProjectRoleAggregate, 0)
	var roles []ProjectRole
	if err := db.Where("project_id = ? AND archived_at IS NULL", aggregate.ID).
		Order("position ASC, id ASC").Find(&roles).Error; err != nil {
		return nil, err
	}
	for _, role := range roles {
		item := ProjectRoleAggregate{ProjectRole: role, Skills: make([]MasterReference, 0), Tools: make([]MasterReference, 0)}
		if role.ContributionRoleID != nil {
			var reference MasterReference
			if err := db.Table("contribution_roles").Select("id, name, slug").Where("id = ?", *role.ContributionRoleID).Take(&reference).Error; err != nil {
				return nil, err
			}
			item.ContributionRole = &reference
		}
		var filled int64
		if err := db.Table("project_members").
			Where("project_role_id = ? AND membership_role = 'contributor' AND status = 'active'", role.ID).
			Count(&filled).Error; err != nil {
			return nil, err
		}
		item.FilledCapacity = int(filled)
		if err := db.Table("skills").Select("skills.id, skills.name, skills.slug").
			Joins("JOIN project_role_skills prs ON prs.skill_id = skills.id").
			Where("prs.project_role_id = ?", role.ID).
			Order("LOWER(skills.name), skills.id").Find(&item.Skills).Error; err != nil {
			return nil, err
		}
		if err := db.Table("tools").Select("tools.id, tools.name, tools.slug").
			Joins("JOIN project_role_tools prt ON prt.tool_id = tools.id").
			Where("prt.project_role_id = ?", role.ID).
			Order("LOWER(tools.name), tools.id").Find(&item.Tools).Error; err != nil {
			return nil, err
		}
		aggregate.Roles = append(aggregate.Roles, item)
	}
	aggregate.OwnerMember = nil
	aggregate.OwnerRole = nil
	if aggregate.OwnerID != nil {
		var member ProjectMember
		err := db.Where("project_id = ? AND membership_role = 'owner' AND status = 'active'", aggregate.ID).Take(&member).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		if err == nil {
			aggregate.OwnerMember = &member
			if member.ContributionRoleID != nil {
				var role MasterReference
				if err := db.Table("contribution_roles").Select("id, name, slug").
					Where("id = ?", *member.ContributionRoleID).Take(&role).Error; err != nil {
					return nil, err
				}
				aggregate.OwnerRole = &role
			}
		}
	}
	return &aggregate, nil
}

func loadPublicProfile(db *gorm.DB, userID uuid.UUID) (*ProjectPublicProfileResponse, error) {
	var profile ProjectPublicProfileResponse
	err := db.Table("profiles").
		Select("profiles.id, profiles.username, profiles.full_name, profiles.headline, profiles.avatar, (users.email_verified_at IS NOT NULL) AS is_verified").
		Joins("JOIN users ON users.id = profiles.id").
		Where("profiles.id = ?", userID).Take(&profile).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &profile, nil
}
