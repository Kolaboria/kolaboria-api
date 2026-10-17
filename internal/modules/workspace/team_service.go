package workspace

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/syahrulfahmi/kolaboria-api/internal/modules/project"
	sharedErrors "github.com/syahrulfahmi/kolaboria-api/internal/shared/errors"
	"gorm.io/gorm"
)

type TeamService struct{ repository TeamRepository }

func NewTeamService(repository TeamRepository) *TeamService {
	return &TeamService{repository: repository}
}

func (service *TeamService) ListMembers(ctx context.Context, actorID uuid.UUID, slug string, page, limit int) ([]TeamMemberResponse, project.ProjectPaginationMeta, error) {
	if actorID == uuid.Nil || slug == "" || page < 1 || limit < 1 {
		return nil, project.ProjectPaginationMeta{}, sharedErrors.ErrBadRequest
	}
	if limit > 50 {
		limit = 50
	}
	snapshot, err := service.repository.LoadTeamPage(ctx, slug, actorID, page, limit)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, project.ProjectPaginationMeta{}, sharedErrors.ErrNotFound
		}
		return nil, project.ProjectPaginationMeta{}, fmt.Errorf("load workspace team: %w", err)
	}
	if snapshot == nil || snapshot.Project == nil {
		return nil, project.ProjectPaginationMeta{}, sharedErrors.ErrNotFound
	}
	canManage := canManageWorkspace(actorID, snapshot.Project, snapshot.CanInitiate)
	if !canManage && !snapshot.ActiveMember {
		return nil, project.ProjectPaginationMeta{}, sharedErrors.ErrNotFound
	}

	members := make([]TeamMemberResponse, 0, len(snapshot.Members))
	for _, record := range snapshot.Members {
		item := TeamMemberResponse{
			ID: record.ID, UserID: record.UserID, MembershipRole: record.MembershipRole,
			MembershipStatus:      record.MembershipStatus,
			JoinedAt:              record.JoinedAt.Time,
			ActiveTaskCount:       record.ActiveTaskCount,
			TaskManagementGranted: record.TaskManagementGranted,
			TaskAccessVersion:     record.TaskAccessVersion,
		}
		if record.ProfileID != nil && record.Username != nil {
			item.Profile = &TeamMemberProfileResponse{
				ID: *record.ProfileID, Username: *record.Username, FullName: record.FullName,
				Headline: record.Headline, Avatar: record.Avatar, IsVerified: record.IsVerified,
			}
		}
		if record.ProjectRoleID != nil && record.ProjectRoleName != nil {
			item.ProjectRole = &TeamMemberRoleResponse{ID: *record.ProjectRoleID, Name: *record.ProjectRoleName}
		}
		if record.ContributionRoleID != nil && record.ContributionName != nil {
			item.ContributionRole = &TeamMemberRoleResponse{ID: *record.ContributionRoleID, Name: *record.ContributionName}
		}
		members = append(members, item)
	}

	pages := int((snapshot.Total + int64(limit) - 1) / int64(limit))
	meta := project.ProjectPaginationMeta{Page: page, Limit: limit, Total: snapshot.Total, TotalPages: pages}
	return members, meta, nil
}
