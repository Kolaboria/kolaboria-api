package project

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ProjectRepository interface {
	WithTransaction(ctx context.Context, fn func(ProjectTx) error) error
	FindByID(ctx context.Context, id uuid.UUID) (*ProjectAggregate, error)
	FindBySlug(ctx context.Context, slug string) (*ProjectAggregate, error)
	FindViewerApplication(ctx context.Context, projectID, applicantID uuid.UUID) (*ViewerApplicationResponse, error)
	ListPublic(ctx context.Context, filter ProjectListFilter) ([]ProjectAggregate, int64, error)
	ListMine(ctx context.Context, actorID uuid.UUID, filter ProjectListFilter) ([]ProjectAggregate, int64, error)
}

type ProjectTx interface {
	FindActor(ctx context.Context, actorID uuid.UUID) (*ProjectActor, error)
	FindInitiableOrganization(ctx context.Context, actorID, organizationID uuid.UUID) (*ProjectOrganization, error)
	FindByClientRequest(ctx context.Context, actorID, requestID uuid.UUID) (*ProjectAggregate, error)
	LockProject(ctx context.Context, projectID uuid.UUID) (*ProjectAggregate, error)
	SlugExists(ctx context.Context, slug string, except *uuid.UUID) (bool, error)
	LockRole(ctx context.Context, projectID, roleID uuid.UUID) (*ProjectRole, error)
	IsActiveProjectMember(ctx context.Context, projectID, userID uuid.UUID) (bool, error)
	FindMasterReferences(ctx context.Context, catalog string, ids []uuid.UUID) ([]MasterReference, error)
	CountRoleUsage(ctx context.Context, roleID uuid.UUID) (RoleUsage, error)
	FindActiveApplication(ctx context.Context, projectID, applicantID uuid.UUID) (*ProjectApplication, error)
	CreateApplication(ctx context.Context, application *ProjectApplication) error
	SaveAggregate(ctx context.Context, aggregate *ProjectAggregate, create bool) error
	UpdateStatus(ctx context.Context, projectID uuid.UUID, status ProjectStatus, publishedAt *time.Time, agreementAt *time.Time, version int64) error
}

type gormProjectRepository struct{ db *gorm.DB }

func NewProjectRepository(db *gorm.DB) ProjectRepository {
	return &gormProjectRepository{db: db}
}

func (r *gormProjectRepository) WithTransaction(ctx context.Context, fn func(ProjectTx) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&gormProjectTx{db: tx})
	})
}

type gormProjectTx struct{ db *gorm.DB }

func (tx *gormProjectTx) FindActor(ctx context.Context, actorID uuid.UUID) (*ProjectActor, error) {
	var actor ProjectActor
	err := tx.db.WithContext(ctx).Table("users AS u").
		Select("u.id, u.is_active, u.deleted_at, u.email_verified_at, p.account_type, u.system_role").
		Joins("LEFT JOIN profiles AS p ON p.id = u.id").
		Clauses(clause.Locking{Strength: "UPDATE", Table: clause.Table{Name: "u"}}).
		Where("u.id = ?", actorID).Take(&actor).Error
	if err != nil {
		return nil, err
	}
	return &actor, nil
}

func (tx *gormProjectTx) FindInitiableOrganization(ctx context.Context, actorID, organizationID uuid.UUID) (*ProjectOrganization, error) {
	var organization ProjectOrganization
	err := tx.db.WithContext(ctx).Table("organizations AS o").
		Select("o.id, o.name, o.slug").
		Joins("JOIN organization_members AS m ON m.organization_id = o.id").
		Joins("JOIN users AS u ON u.id = m.user_id").
		Clauses(clause.Locking{Strength: "UPDATE", Table: clause.Table{Name: "m"}}).
		Where("o.id = ? AND o.is_active = true AND m.user_id = ? AND m.is_active = true AND m.can_initiate_projects = true", organizationID, actorID).
		Where("u.is_active = true AND u.deleted_at IS NULL AND u.system_role = ?", "admin").
		Take(&organization).Error
	if err != nil {
		return nil, err
	}
	return &organization, nil
}

func (tx *gormProjectTx) FindByClientRequest(ctx context.Context, actorID, requestID uuid.UUID) (*ProjectAggregate, error) {
	return loadProjectAggregate(tx.db.WithContext(ctx), "projects.created_by_user_id = ? AND projects.client_request_id = ?", actorID, requestID)
}

func (tx *gormProjectTx) LockProject(ctx context.Context, projectID uuid.UUID) (*ProjectAggregate, error) {
	var project Project
	err := tx.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ?", projectID).Take(&project).Error
	if err != nil {
		return nil, err
	}
	return loadProjectAggregate(tx.db.WithContext(ctx), "projects.id = ?", projectID)
}

func (tx *gormProjectTx) SlugExists(ctx context.Context, slug string, except *uuid.UUID) (bool, error) {
	query := tx.db.WithContext(ctx).Model(&Project{}).Where("slug = ?", slug)
	if except != nil {
		query = query.Where("id <> ?", *except)
	}
	var count int64
	err := query.Count(&count).Error
	return count > 0, err
}

func (tx *gormProjectTx) LockRole(ctx context.Context, projectID, roleID uuid.UUID) (*ProjectRole, error) {
	var role ProjectRole
	err := tx.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("project_id = ? AND id = ? AND archived_at IS NULL", projectID, roleID).Take(&role).Error
	if err != nil {
		return nil, err
	}
	return &role, nil
}

func (tx *gormProjectTx) IsActiveProjectMember(ctx context.Context, projectID, userID uuid.UUID) (bool, error) {
	var count int64
	err := tx.db.WithContext(ctx).Model(&ProjectMember{}).
		Where("project_id = ? AND user_id = ? AND status = 'active'", projectID, userID).
		Count(&count).Error
	return count > 0, err
}

func (tx *gormProjectTx) FindMasterReferences(ctx context.Context, catalog string, ids []uuid.UUID) ([]MasterReference, error) {
	if len(ids) == 0 {
		return []MasterReference{}, nil
	}
	table := ""
	switch catalog {
	case "contribution_roles":
		table = "contribution_roles"
	case "skills":
		table = "skills"
	case "tools":
		table = "tools"
	default:
		return nil, fmt.Errorf("unsupported master catalog %q", catalog)
	}
	items := make([]MasterReference, 0, len(ids))
	err := tx.db.WithContext(ctx).Table(table).
		Select("id, name, slug").Where("id IN ?", ids).Order("id").Find(&items).Error
	return items, err
}

func (tx *gormProjectTx) CountRoleUsage(ctx context.Context, roleID uuid.UUID) (RoleUsage, error) {
	var usage RoleUsage
	var activeCount int64
	err := tx.db.WithContext(ctx).Table("project_members").
		Where("project_role_id = ? AND membership_role = 'contributor' AND status = 'active'", roleID).
		Count(&activeCount).Error
	if err != nil {
		return RoleUsage{}, err
	}
	usage.ActiveContributors = int(activeCount)
	// Applications are not part of the active schema yet.
	return usage, nil
}

func (tx *gormProjectTx) FindActiveApplication(ctx context.Context, projectID, applicantID uuid.UUID) (*ProjectApplication, error) {
	var application ProjectApplication
	err := tx.db.WithContext(ctx).
		Where("project_id = ? AND applicant_user_id = ? AND status IN ?", projectID, applicantID, []string{"pending", "accepted"}).
		Take(&application).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &application, nil
}

func (tx *gormProjectTx) CreateApplication(ctx context.Context, application *ProjectApplication) error {
	return tx.db.WithContext(ctx).Create(application).Error
}

func (tx *gormProjectTx) SaveAggregate(ctx context.Context, aggregate *ProjectAggregate, create bool) error {
	db := tx.db.WithContext(ctx)
	if create {
		if err := db.Create(&aggregate.Project).Error; err != nil {
			return err
		}
	} else {
		updates := projectDefinitionUpdates(&aggregate.Project)
		if err := db.Model(&Project{}).Where("id = ?", aggregate.ID).Updates(updates).Error; err != nil {
			return err
		}
		// Temporarily archive the current rows so a reorder cannot collide with
		// the active (project_id, position) uniqueness index mid-transaction.
		if err := db.Model(&ProjectRole{}).
			Where("project_id = ? AND archived_at IS NULL", aggregate.ID).
			Updates(map[string]any{"archived_at": time.Now().UTC(), "updated_at": time.Now().UTC()}).Error; err != nil {
			return err
		}
	}

	for i := range aggregate.Roles {
		role := &aggregate.Roles[i].ProjectRole
		role.ProjectID = aggregate.ID
		if role.ID == uuid.Nil {
			role.ID = uuid.New()
		}
		if role.CreatedAt.IsZero() {
			role.CreatedAt = time.Now().UTC()
		}
		role.UpdatedAt = time.Now().UTC()
		if err := saveRole(db, role); err != nil {
			return err
		}
		if err := replaceRoleReferences(db, role.ID, aggregate.Roles[i].Skills, aggregate.Roles[i].Tools); err != nil {
			return err
		}
	}
	if aggregate.OwnerMember != nil {
		member := aggregate.OwnerMember
		member.ProjectID = aggregate.ID
		member.MembershipRole = "owner"
		member.Status = "active"
		member.ProjectRoleID = nil
		if member.ID == uuid.Nil {
			member.ID = uuid.New()
		}
		if member.CreatedAt.IsZero() {
			member.CreatedAt = time.Now().UTC()
		}
		member.UpdatedAt = time.Now().UTC()
		if member.JoinedAt.IsZero() {
			member.JoinedAt = member.CreatedAt
		}
		if err := saveOwnerMember(db, member); err != nil {
			return err
		}
	}
	return nil
}

func (tx *gormProjectTx) UpdateStatus(ctx context.Context, projectID uuid.UUID, status ProjectStatus, publishedAt *time.Time, agreementAt *time.Time, version int64) error {
	return tx.db.WithContext(ctx).Model(&Project{}).Where("id = ?", projectID).Updates(map[string]any{
		"status": status, "published_at": publishedAt, "agreement_at": agreementAt,
		"version": version, "updated_at": time.Now().UTC(),
	}).Error
}

func projectDefinitionUpdates(project *Project) map[string]any {
	return map[string]any{
		"slug":                    project.Slug,
		"title":                   project.Title,
		"summary":                 project.Summary,
		"description":             project.Description,
		"project_category":        project.ProjectCategory,
		"visibility":              project.Visibility,
		"origin":                  project.Origin,
		"why_collaborative":       project.WhyCollaborative,
		"contributor_outcome":     project.ContributorOutcome,
		"owner_commitment":        project.OwnerCommitment,
		"lead_expectations":       project.LeadExpectations,
		"client_acknowledgement":  project.ClientAcknowledgement,
		"start_date":              project.StartDate,
		"deadline":                project.Deadline,
		"availability":            project.Availability,
		"hours_per_week":          project.HoursPerWeek,
		"collaboration_agreement": project.CollaborationAgreement,
		"agreement_at":            project.AgreementAt,
		"version":                 project.Version,
		"updated_at":              time.Now().UTC(),
	}
}

func saveRole(db *gorm.DB, role *ProjectRole) error {
	var existing int64
	if err := db.Model(&ProjectRole{}).Where("id = ? AND project_id = ?", role.ID, role.ProjectID).Count(&existing).Error; err != nil {
		return err
	}
	if existing == 0 {
		return db.Create(role).Error
	}
	result := db.Model(&ProjectRole{}).Where("id = ? AND project_id = ?", role.ID, role.ProjectID).Updates(map[string]any{
		"contribution_role_id": role.ContributionRoleID,
		"description":          role.Description,
		"capacity":             role.Capacity,
		"position":             role.Position,
		"archived_at":          nil,
		"updated_at":           role.UpdatedAt,
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func replaceRoleReferences(db *gorm.DB, roleID uuid.UUID, skills, tools []MasterReference) error {
	if err := db.Where("project_role_id = ?", roleID).Delete(&ProjectRoleSkill{}).Error; err != nil {
		return err
	}
	if err := db.Where("project_role_id = ?", roleID).Delete(&ProjectRoleTool{}).Error; err != nil {
		return err
	}
	if len(skills) > 0 {
		rows := make([]ProjectRoleSkill, 0, len(skills))
		for _, item := range skills {
			rows = append(rows, ProjectRoleSkill{ProjectRoleID: roleID, SkillID: item.ID})
		}
		if err := db.Create(&rows).Error; err != nil {
			return err
		}
	}
	if len(tools) > 0 {
		rows := make([]ProjectRoleTool, 0, len(tools))
		for _, item := range tools {
			rows = append(rows, ProjectRoleTool{ProjectRoleID: roleID, ToolID: item.ID})
		}
		if err := db.Create(&rows).Error; err != nil {
			return err
		}
	}
	return nil
}

func saveOwnerMember(db *gorm.DB, member *ProjectMember) error {
	var existing ProjectMember
	err := db.Where("project_id = ? AND membership_role = 'owner'", member.ProjectID).Take(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return db.Create(member).Error
	}
	if err != nil {
		return err
	}
	return db.Model(&ProjectMember{}).Where("id = ?", existing.ID).Updates(map[string]any{
		"contribution_role_id": member.ContributionRoleID,
		"updated_at":           member.UpdatedAt,
	}).Error
}
