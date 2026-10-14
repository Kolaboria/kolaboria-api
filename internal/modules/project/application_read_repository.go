package project

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type gormApplicationRepository struct{ db *gorm.DB }

func NewApplicationRepository(db *gorm.DB) ApplicationRepository {
	return &gormApplicationRepository{db: db}
}

func (r *gormApplicationRepository) WithApplicationTransaction(ctx context.Context, fn func(ApplicationTx) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&gormProjectTx{db: tx})
	})
}

func (r *gormApplicationRepository) FindActorForRead(ctx context.Context, id uuid.UUID) (*ProjectActor, error) {
	return findApplicationActor(r.db.WithContext(ctx), id, false)
}

func findApplicationActor(db *gorm.DB, id uuid.UUID, lock bool) (*ProjectActor, error) {
	var actor ProjectActor
	query := db.Table("users AS u").Select("u.id, u.is_active, u.deleted_at, u.email_verified_at, p.account_type, u.system_role").
		Joins("LEFT JOIN profiles AS p ON p.id = u.id").Where("u.id = ?", id)
	if lock {
		query = query.Clauses(clause.Locking{Strength: "UPDATE", Table: clause.Table{Name: "u"}})
	}
	err := query.Take(&actor).Error
	if err != nil {
		return nil, err
	}
	return &actor, nil
}

func (r *gormApplicationRepository) FindOwnerProject(ctx context.Context, actorID, projectID uuid.UUID) (*Project, error) {
	var project Project
	err := r.db.WithContext(ctx).Where("id = ? AND owner_id = ?", projectID, actorID).Take(&project).Error
	if err != nil {
		return nil, err
	}
	return &project, nil
}

func (r *gormApplicationRepository) IsActiveProjectMember(ctx context.Context, projectID, userID uuid.UUID) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&ProjectMember{}).Where("project_id = ? AND user_id = ? AND status = 'active'", projectID, userID).Count(&count).Error
	return count > 0, err
}

func (r *gormApplicationRepository) CountProjectRoleIDs(ctx context.Context, projectID uuid.UUID, ids []uuid.UUID) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	var count int64
	err := r.db.WithContext(ctx).Model(&ProjectRole{}).Where("project_id = ? AND id IN ?", projectID, ids).Count(&count).Error
	return count, err
}

func (r *gormApplicationRepository) GetOwnerSummary(ctx context.Context, actorID, projectID uuid.UUID) (*ApplicationManagementReadModel, error) {
	var result *ApplicationManagementReadModel
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var project Project
		if err := tx.Where("id = ? AND owner_id = ?", projectID, actorID).Take(&project).Error; err != nil {
			return err
		}
		var err error
		result, err = loadApplicationManagement(tx, project)
		return err
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return result, err
}

func loadApplicationManagement(db *gorm.DB, project Project) (*ApplicationManagementReadModel, error) {
	model := &ApplicationManagementReadModel{Project: project, Roles: []ApplicationRoleReadModel{}}
	var stats ApplicationStatsReadModel
	if err := db.Table("project_applications").Select(`COUNT(*) AS total,
		COUNT(*) FILTER (WHERE status = 'pending') AS pending,
		COUNT(*) FILTER (WHERE status = 'accepted') AS accepted,
		COUNT(*) FILTER (WHERE status = 'rejected') AS rejected,
		COUNT(*) FILTER (WHERE status = 'withdrawn') AS withdrawn`).
		Where("project_id = ?", project.ID).Scan(&stats).Error; err != nil {
		return nil, err
	}
	model.Stats = stats
	roles, err := loadApplicationRoles(db, []uuid.UUID{project.ID})
	if err != nil {
		return nil, err
	}
	for _, role := range roles {
		model.Roles = append(model.Roles, role)
	}
	profiles, err := loadApplicationProfiles(db, uuidPointers(project.OwnerID))
	if err != nil {
		return nil, err
	}
	model.Owner = profiles[project.OwnerIDValue()]
	if project.InitiatorOrganizationID != nil {
		var org ProjectOrganizationResponse
		if err := db.Table("organizations").Select("id, name, slug").Where("id = ?", *project.InitiatorOrganizationID).Take(&org).Error; err != nil {
			return nil, err
		}
		model.Org = &org
	}
	return model, nil
}

func (r *gormApplicationRepository) ListOwnerApplications(ctx context.Context, actorID, projectID uuid.UUID, filter ApplicationListFilter) ([]ApplicationReadModel, int64, error) {
	query := r.db.WithContext(ctx).Table("project_applications AS a").
		Joins("JOIN projects p ON p.id = a.project_id").
		Joins("LEFT JOIN profiles ap ON ap.id = a.applicant_user_id").
		Joins("LEFT JOIN project_roles pr ON pr.id = a.project_role_id AND pr.project_id = a.project_id").
		Joins("LEFT JOIN contribution_roles cr ON cr.id = pr.contribution_role_id").
		Where("a.project_id = ? AND p.owner_id = ?", projectID, actorID)
	query = applyApplicationFilters(query, filter, true)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var applications []ProjectApplication
	order := "a.applied_at DESC, a.id ASC"
	if filter.Sort == ApplicationSortOldest {
		order = "a.applied_at ASC, a.id ASC"
	}
	err := query.Select("a.*").Order(order).Offset((filter.Page - 1) * filter.Limit).Limit(filter.Limit).Scan(&applications).Error
	if err != nil {
		return nil, 0, err
	}
	items, err := hydrateApplications(r.db.WithContext(ctx), applications)
	return items, total, err
}

func (r *gormApplicationRepository) GetVisibleApplication(ctx context.Context, actorID, applicationID uuid.UUID) (*ApplicationReadModel, error) {
	var application ProjectApplication
	err := r.db.WithContext(ctx).Table("project_applications AS a").
		Joins("JOIN projects p ON p.id = a.project_id").
		Where("a.id = ? AND (p.owner_id = ? OR a.applicant_user_id = ?)", applicationID, actorID, actorID).
		Select("a.*").Take(&application).Error
	if err != nil {
		return nil, err
	}
	items, err := hydrateApplications(r.db.WithContext(ctx), []ProjectApplication{application})
	if err != nil {
		return nil, err
	}
	if len(items) != 1 {
		return nil, gorm.ErrRecordNotFound
	}
	return &items[0], nil
}

func (r *gormApplicationRepository) GetMySummary(ctx context.Context, actorID uuid.UUID) (*ApplicationStatsReadModel, error) {
	var stats ApplicationStatsReadModel
	err := r.db.WithContext(ctx).Table("project_applications").Select(`COUNT(*) AS total,
		COUNT(*) FILTER (WHERE status = 'pending') AS pending,
		COUNT(*) FILTER (WHERE status = 'accepted') AS accepted,
		COUNT(*) FILTER (WHERE status = 'rejected') AS rejected,
		COUNT(*) FILTER (WHERE status = 'withdrawn') AS withdrawn`).
		Where("applicant_user_id = ?", actorID).Scan(&stats).Error
	return &stats, err
}

func (r *gormApplicationRepository) ListApplicantApplications(ctx context.Context, actorID uuid.UUID, filter ApplicationListFilter) ([]ApplicationReadModel, int64, error) {
	query := r.db.WithContext(ctx).Table("project_applications AS a").
		Joins("JOIN projects p ON p.id = a.project_id").
		Joins("LEFT JOIN profiles ap ON ap.id = a.applicant_user_id").
		Joins("LEFT JOIN profiles op ON op.id = p.owner_id").
		Joins("LEFT JOIN organizations o ON o.id = p.initiator_organization_id").
		Joins("LEFT JOIN project_roles pr ON pr.id = a.project_role_id AND pr.project_id = a.project_id").
		Joins("LEFT JOIN contribution_roles cr ON cr.id = pr.contribution_role_id").
		Where("a.applicant_user_id = ?", actorID)
	query = applyApplicationFilters(query, filter, false)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var applications []ProjectApplication
	order := "a.applied_at DESC, a.id ASC"
	if filter.Sort == ApplicationSortOldest {
		order = "a.applied_at ASC, a.id ASC"
	}
	err := query.Select("a.*").Order(order).Offset((filter.Page - 1) * filter.Limit).Limit(filter.Limit).Scan(&applications).Error
	if err != nil {
		return nil, 0, err
	}
	items, err := hydrateApplications(r.db.WithContext(ctx), applications)
	return items, total, err
}

func (r *gormApplicationRepository) FindApplicationIdentity(ctx context.Context, id uuid.UUID) (*ApplicationIdentity, error) {
	var identity ApplicationIdentity
	err := r.db.WithContext(ctx).Table("project_applications").Select("id AS application_id, project_id, project_role_id, applicant_user_id").Where("id = ?", id).Take(&identity).Error
	if err != nil {
		return nil, err
	}
	return &identity, nil
}

func applyApplicationFilters(query *gorm.DB, filter ApplicationListFilter, owner bool) *gorm.DB {
	if len(filter.Statuses) > 0 {
		values := make([]string, 0, len(filter.Statuses))
		for _, status := range filter.Statuses {
			values = append(values, string(status))
		}
		query = query.Where("a.status IN ?", values)
	}
	if owner && len(filter.ProjectRoleIDs) > 0 {
		query = query.Where("a.project_role_id IN ?", filter.ProjectRoleIDs)
	}
	if !owner && len(filter.Origins) > 0 {
		query = query.Where("p.origin IN ?", filter.Origins)
	}
	if search := strings.TrimSpace(filter.Search); search != "" {
		pattern := applicationSearchPattern(search)
		var searchQuery string
		var args []any
		if owner {
			searchQuery = `(
				COALESCE(NULLIF(BTRIM(ap.full_name), ''), NULLIF(BTRIM(ap.username), ''), 'Talent Kolaboria') ILIKE ? ESCAPE '\'
				OR ap.username ILIKE ? ESCAPE '\' OR ap.headline ILIKE ? ESCAPE '\'
				OR cr.name ILIKE ? ESCAPE '\' OR a.motivation ILIKE ? ESCAPE '\'
				OR a.expected_contribution ILIKE ? ESCAPE '\'
				OR EXISTS (SELECT 1 FROM project_role_skills prs JOIN skills s ON s.id = prs.skill_id WHERE prs.project_role_id = a.project_role_id AND s.name ILIKE ? ESCAPE '\')
				OR EXISTS (SELECT 1 FROM project_role_tools prt JOIN tools t ON t.id = prt.tool_id WHERE prt.project_role_id = a.project_role_id AND t.name ILIKE ? ESCAPE '\')
			)`
			args = []any{pattern, pattern, pattern, pattern, pattern, pattern, pattern, pattern}
		} else {
			searchQuery = `(
				p.title ILIKE ? ESCAPE '\' OR p.summary ILIKE ? ESCAPE '\'
				OR COALESCE(NULLIF(BTRIM(op.full_name), ''), NULLIF(BTRIM(op.username), ''), o.name) ILIKE ? ESCAPE '\'
				OR op.username ILIKE ? ESCAPE '\' OR op.headline ILIKE ? ESCAPE '\' OR o.name ILIKE ? ESCAPE '\'
				OR cr.name ILIKE ? ESCAPE '\' OR a.motivation ILIKE ? ESCAPE '\' OR a.expected_contribution ILIKE ? ESCAPE '\'
			)`
			args = []any{pattern, pattern, pattern, pattern, pattern, pattern, pattern, pattern, pattern}
		}
		query = query.Where(searchQuery, args...)
	}
	return query
}

func applicationSearchPattern(value string) string {
	return "%" + strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_").Replace(value) + "%"
}

type applicationProfileRow struct {
	ID       uuid.UUID `gorm:"column:id"`
	Username string    `gorm:"column:username"`
	FullName *string   `gorm:"column:full_name"`
	Headline *string   `gorm:"column:headline"`
	Avatar   *string   `gorm:"column:avatar"`
}

func loadApplicationProfiles(db *gorm.DB, ids []uuid.UUID) (map[uuid.UUID]*ApplicationProfileResponse, error) {
	result := make(map[uuid.UUID]*ApplicationProfileResponse, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	rows := make([]applicationProfileRow, 0, len(ids))
	err := db.Table("profiles").Select("id, username, full_name, headline, avatar").Where("id IN ?", ids).Find(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.ID] = &ApplicationProfileResponse{ID: row.ID, Username: row.Username, FullName: row.FullName, Headline: row.Headline, Avatar: row.Avatar}
	}
	return result, nil
}

func loadApplicationRoles(db *gorm.DB, projectIDs []uuid.UUID) (map[uuid.UUID]ApplicationRoleReadModel, error) {
	result := make(map[uuid.UUID]ApplicationRoleReadModel)
	if len(projectIDs) == 0 {
		return result, nil
	}
	var roles []ProjectRole
	query := db.Where("project_id IN ?", projectIDs)
	query = query.Where("archived_at IS NULL OR EXISTS (SELECT 1 FROM project_applications a WHERE a.project_role_id = project_roles.id)")
	if err := query.Order("project_id, archived_at NULLS FIRST, position, id").Find(&roles).Error; err != nil {
		return nil, err
	}
	roleIDs := make([]uuid.UUID, 0, len(roles))
	masterIDs := make([]uuid.UUID, 0, len(roles))
	for _, role := range roles {
		roleIDs = append(roleIDs, role.ID)
		if role.ContributionRoleID != nil {
			masterIDs = append(masterIDs, *role.ContributionRoleID)
		}
	}
	masters := make(map[uuid.UUID]MasterReference)
	var masterRows []MasterReference
	if len(masterIDs) > 0 {
		if err := db.Table("contribution_roles").Select("id, name, slug").Where("id IN ?", masterIDs).Find(&masterRows).Error; err != nil {
			return nil, err
		}
		for _, row := range masterRows {
			masters[row.ID] = row
		}
	}
	counts := make(map[uuid.UUID]int)
	type countRow struct {
		RoleID uuid.UUID `gorm:"column:project_role_id"`
		Count  int       `gorm:"column:count"`
	}
	var countRows []countRow
	if len(roleIDs) > 0 {
		if err := db.Table("project_members").Select("project_role_id, COUNT(*) AS count").Where("project_role_id IN ? AND membership_role = 'contributor' AND status = 'active'", roleIDs).Group("project_role_id").Scan(&countRows).Error; err != nil {
			return nil, err
		}
	}
	for _, row := range countRows {
		counts[row.RoleID] = row.Count
	}
	skills := make(map[uuid.UUID][]MasterReference)
	tools := make(map[uuid.UUID][]MasterReference)
	if len(roleIDs) > 0 {
		var skillRows []struct {
			RoleID uuid.UUID `gorm:"column:project_role_id"`
			MasterReference
		}
		if err := db.Table("project_role_skills AS prs").Select("prs.project_role_id, s.id, s.name, s.slug").Joins("JOIN skills s ON s.id = prs.skill_id").Where("prs.project_role_id IN ?", roleIDs).Order("lower(s.name), s.id").Scan(&skillRows).Error; err != nil {
			return nil, err
		}
		for _, row := range skillRows {
			skills[row.RoleID] = append(skills[row.RoleID], row.MasterReference)
		}
		var toolRows []struct {
			RoleID uuid.UUID `gorm:"column:project_role_id"`
			MasterReference
		}
		if err := db.Table("project_role_tools AS prt").Select("prt.project_role_id, t.id, t.name, t.slug").Joins("JOIN tools t ON t.id = prt.tool_id").Where("prt.project_role_id IN ?", roleIDs).Order("lower(t.name), t.id").Scan(&toolRows).Error; err != nil {
			return nil, err
		}
		for _, row := range toolRows {
			tools[row.RoleID] = append(tools[row.RoleID], row.MasterReference)
		}
	}
	for _, role := range roles {
		var contribution *MasterReference
		if role.ContributionRoleID != nil {
			if ref, ok := masters[*role.ContributionRoleID]; ok {
				copy := ref
				contribution = &copy
			}
		}
		result[role.ID] = ApplicationRoleReadModel{Role: role, ContributionRole: contribution, Skills: skills[role.ID], Tools: tools[role.ID], ActiveMembers: counts[role.ID]}
	}
	return result, nil
}

func hydrateApplications(db *gorm.DB, applications []ProjectApplication) ([]ApplicationReadModel, error) {
	if applications == nil {
		applications = []ProjectApplication{}
	}
	if len(applications) == 0 {
		return []ApplicationReadModel{}, nil
	}
	projectIDs, userIDs := make([]uuid.UUID, 0), make([]uuid.UUID, 0)
	roleIDs := make([]uuid.UUID, 0, len(applications))
	pairs := make([][2]uuid.UUID, 0, len(applications))
	for _, app := range applications {
		projectIDs = append(projectIDs, app.ProjectID)
		userIDs = append(userIDs, app.ApplicantUserID)
		roleIDs = append(roleIDs, app.ProjectRoleID)
		pairs = append(pairs, [2]uuid.UUID{app.ProjectID, app.ApplicantUserID})
	}
	projects := make(map[uuid.UUID]Project)
	var projectRows []Project
	if err := db.Where("id IN ?", uniqueUUIDs(projectIDs)).Find(&projectRows).Error; err != nil {
		return nil, err
	}
	ownerIDs, orgIDs := make([]uuid.UUID, 0), make([]uuid.UUID, 0)
	for _, project := range projectRows {
		projects[project.ID] = project
		if project.OwnerID != nil {
			ownerIDs = append(ownerIDs, *project.OwnerID)
		}
		if project.InitiatorOrganizationID != nil {
			orgIDs = append(orgIDs, *project.InitiatorOrganizationID)
		}
	}
	for _, id := range ownerIDs {
		userIDs = append(userIDs, id)
	}
	profiles, err := loadApplicationProfiles(db, uniqueUUIDs(userIDs))
	if err != nil {
		return nil, err
	}
	organizations := make(map[uuid.UUID]*ProjectOrganizationResponse)
	if len(orgIDs) > 0 {
		var rows []ProjectOrganizationResponse
		if err := db.Table("organizations").Select("id, name, slug").Where("id IN ?", uniqueUUIDs(orgIDs)).Find(&rows).Error; err != nil {
			return nil, err
		}
		for i := range rows {
			organizations[rows[i].ID] = &rows[i]
		}
	}
	roles, err := loadApplicationRoles(db, uniqueUUIDs(projectIDs))
	if err != nil {
		return nil, err
	}
	for id := range roles {
		if !containsUUID(roleIDs, id) {
			delete(roles, id)
		}
	}
	retries, err := loadLatestRejected(db, pairs)
	if err != nil {
		return nil, err
	}
	result := make([]ApplicationReadModel, 0, len(applications))
	for _, app := range applications {
		project := projects[app.ProjectID]
		var owner *ApplicationProfileResponse
		if project.OwnerID != nil {
			owner = profiles[*project.OwnerID]
		}
		var org *ProjectOrganizationResponse
		if project.InitiatorOrganizationID != nil {
			org = organizations[*project.InitiatorOrganizationID]
		}
		role, ok := roles[app.ProjectRoleID]
		if !ok {
			return nil, fmt.Errorf("application %s references missing project role", app.ID)
		}
		model := ApplicationReadModel{Application: app, Profile: profiles[app.ApplicantUserID], Project: mapApplicationProject(project, owner, org), Role: role}
		key := applicationPair{ProjectID: app.ProjectID, ApplicantID: app.ApplicantUserID}
		if rejected, exists := retries[key]; exists {
			retry, estimated := rejectionRetryAfter(rejected)
			model.RetryAfter = &retry
			model.RetryAfterEstimated = estimated
		}
		result = append(result, model)
	}
	return result, nil
}

type applicationPair struct{ ProjectID, ApplicantID uuid.UUID }

func loadLatestRejected(db *gorm.DB, pairs [][2]uuid.UUID) (map[applicationPair]ProjectApplication, error) {
	result := make(map[applicationPair]ProjectApplication, len(pairs))
	if len(pairs) == 0 {
		return result, nil
	}
	conditions := make([]string, 0, len(pairs))
	args := make([]any, 0, len(pairs)*2)
	for _, pair := range pairs {
		conditions = append(conditions, "(project_id = ? AND applicant_user_id = ?)")
		args = append(args, pair[0], pair[1])
	}
	var rows []ProjectApplication
	err := db.Where("status = 'rejected' AND ("+strings.Join(conditions, " OR ")+")", args...).
		Order("project_id, applicant_user_id, applied_at DESC, id ASC").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		key := applicationPair{row.ProjectID, row.ApplicantUserID}
		if _, exists := result[key]; !exists {
			result[key] = row
		}
	}
	return result, nil
}

func uuidPointers(value *uuid.UUID) []uuid.UUID {
	if value == nil {
		return []uuid.UUID{}
	}
	return []uuid.UUID{*value}
}
func (project Project) OwnerIDValue() uuid.UUID {
	if project.OwnerID == nil {
		return uuid.Nil
	}
	return *project.OwnerID
}
func containsUUID(values []uuid.UUID, id uuid.UUID) bool {
	for _, value := range values {
		if value == id {
			return true
		}
	}
	return false
}

var _ ApplicationRepository = (*gormApplicationRepository)(nil)
