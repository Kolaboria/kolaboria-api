package project

import (
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

func mapApplicationStats(stats ApplicationStatsReadModel) ApplicantStatsResponse {
	return ApplicantStatsResponse{Total: stats.Total, Pending: stats.Pending, Accepted: stats.Accepted, Rejected: stats.Rejected, Withdrawn: stats.Withdrawn}
}

func mapApplicationProject(project Project, owner *ApplicationProfileResponse, organization *ProjectOrganizationResponse) ApplicationProjectResponse {
	return ApplicationProjectResponse{
		ID: project.ID, Title: project.Title, Slug: project.Slug, Status: project.Status,
		Summary: project.Summary, Origin: project.Origin, CreationMode: project.CreationMode,
		OwnerProfile: owner, InitiatorOrganization: organization,
	}
}

func mapApplicationRole(model ApplicationRoleReadModel) ApplicationRoleResponse {
	role := model.Role
	contribution := mapMasterReference(model.ContributionRole)
	remaining := role.Capacity - model.ActiveMembers
	if remaining < 0 {
		remaining = 0
	}
	status := "open"
	if role.ArchivedAt != nil {
		status = "archived"
	} else if remaining == 0 {
		status = "full"
	}
	return ApplicationRoleResponse{
		ID: role.ID, ContributionRoleID: cloneUUIDPointer(role.ContributionRoleID), ContributionRole: contribution,
		Description: role.Description, Capacity: role.Capacity, FilledCapacity: model.ActiveMembers,
		RemainingCapacity: remaining, Status: status, IsArchived: role.ArchivedAt != nil,
		Skills: mapMasterReferences(model.Skills), Tools: mapMasterReferences(model.Tools),
	}
}

func mapApplicationListItem(model ApplicationReadModel) ApplicantListItemResponse {
	application := model.Application
	return ApplicantListItemResponse{
		ID: application.ID, ProjectID: application.ProjectID, ApplicantUserID: application.ApplicantUserID,
		ProjectRoleID: application.ProjectRoleID, Status: ApplicationStatus(application.Status), AppliedAt: application.AppliedAt,
		Availability: application.Availability, EstimatedHoursPerWeek: application.EstimatedHoursPerWeek,
		Motivation: application.Motivation, ExpectedContribution: application.ExpectedContribution,
		ApplicantProfile: model.Profile, ProjectRole: mapApplicationRole(model.Role),
	}
}

func mapApplicationDetail(model ApplicationReadModel, permissions ApplicationPermissionsResponse) ApplicationDetailResponse {
	links := safePortfolioLinks(model.Application.PortfolioLinks)
	if links == nil {
		links = []string{}
	}
	return ApplicationDetailResponse{
		ApplicantListItemResponse: mapApplicationListItem(model),
		PortfolioLinks:            links, ReviewerNote: cloneStringPointer(model.Application.ReviewerNote),
		ReviewedBy: cloneUUIDPointer(model.Application.ReviewedBy), ReviewedAt: cloneTimePointer(model.Application.ReviewedAt),
		WithdrawnAt: cloneTimePointer(model.Application.WithdrawnAt), RetryAfter: cloneTimePointer(model.RetryAfter), RetryAfterEstimated: model.RetryAfterEstimated,
		ReapplyBlockReason: cloneStringPointer(model.BlockReason), Project: model.Project, Permissions: permissions,
	}
}

func mapMyApplicationListItem(model ApplicationReadModel) MyApplicationListItemResponse {
	item := MyApplicationListItemResponse{
		ApplicantListItemResponse: mapApplicationListItem(model),
		ReviewerNote:              cloneStringPointer(model.Application.ReviewerNote),
		ReviewedAt:                cloneTimePointer(model.Application.ReviewedAt), WithdrawnAt: cloneTimePointer(model.Application.WithdrawnAt),
		RetryAfter: cloneTimePointer(model.RetryAfter), RetryAfterEstimated: model.RetryAfterEstimated, ReapplyBlockReason: cloneStringPointer(model.BlockReason),
		Project: model.Project,
	}
	return item
}

func mapApplicantSummary(model ApplicationManagementReadModel, canReview bool, reason *string) ApplicantSummaryResponse {
	roles := make([]ApplicantRoleSummaryResponse, 0, len(model.Roles))
	capacity, filled := 0, 0
	for _, role := range model.Roles {
		mapped := mapApplicationRole(role)
		if role.Role.ArchivedAt == nil {
			capacity += role.Role.Capacity
			filled += role.ActiveMembers
		}
		name := "Kontributor"
		if mapped.ContributionRole != nil && strings.TrimSpace(mapped.ContributionRole.Name) != "" {
			name = mapped.ContributionRole.Name
		}
		roles = append(roles, ApplicantRoleSummaryResponse{
			ID: mapped.ID, Name: name, Capacity: mapped.Capacity, FilledCapacity: mapped.FilledCapacity,
			RemainingCapacity: mapped.RemainingCapacity, Status: mapped.Status, IsArchived: mapped.IsArchived,
		})
	}
	sort.SliceStable(roles, func(i, j int) bool {
		if roles[i].IsArchived != roles[j].IsArchived {
			return !roles[i].IsArchived
		}
		left, right := rolePosition(model.Roles, roles[i].ID), rolePosition(model.Roles, roles[j].ID)
		if left != right {
			return left < right
		}
		return roles[i].ID.String() < roles[j].ID.String()
	})
	remaining := capacity - filled
	if remaining < 0 {
		remaining = 0
	}
	return ApplicantSummaryResponse{
		Project:  mapApplicationProject(model.Project, model.Owner, model.Org),
		Capacity: ApplicantCapacityResponse{TotalCapacity: capacity, FilledCapacity: filled, RemainingCapacity: remaining},
		Stats:    mapApplicationStats(model.Stats), Roles: roles,
		Permissions: ApplicantSummaryPermissionsResponse{CanReviewApplications: canReview, ReviewBlockReason: cloneStringPointer(reason)},
	}
}

func rolePosition(roles []ApplicationRoleReadModel, id uuid.UUID) int {
	for _, role := range roles {
		if role.Role.ID == id {
			return role.Role.Position
		}
	}
	return 0
}

func mapMasterReferences(references []MasterReference) []ProjectMasterResponse {
	result := make([]ProjectMasterResponse, 0, len(references))
	for _, reference := range references {
		result = append(result, ProjectMasterResponse{ID: reference.ID, Name: reference.Name, Slug: reference.Slug})
	}
	return result
}

func safePortfolioLinks(links []string) []string {
	result := make([]string, 0, len(links))
	for _, link := range links {
		parsed, err := url.Parse(strings.TrimSpace(link))
		if err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != "" && len(link) <= 2048 {
			result = append(result, strings.TrimSpace(link))
		}
	}
	return result
}

func cloneTimePointer(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func cloneStringPointer(value *string) *string {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}
