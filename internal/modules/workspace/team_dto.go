package workspace

import (
	"time"

	"github.com/google/uuid"
)

type TeamMemberProfileResponse struct {
	ID         uuid.UUID `json:"id"`
	Username   string    `json:"username"`
	FullName   *string   `json:"full_name"`
	Headline   *string   `json:"headline"`
	Avatar     *string   `json:"avatar"`
	IsVerified bool      `json:"is_verified"`
}

type TeamMemberRoleResponse struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

type TeamMemberResponse struct {
	ID                    uuid.UUID                  `json:"id"`
	UserID                uuid.UUID                  `json:"user_id"`
	MembershipRole        string                     `json:"membership_role"`
	MembershipStatus      string                     `json:"membership_status"`
	Profile               *TeamMemberProfileResponse `json:"profile"`
	ProjectRole           *TeamMemberRoleResponse    `json:"project_role"`
	ContributionRole      *TeamMemberRoleResponse    `json:"contribution_role"`
	JoinedAt              time.Time                  `json:"joined_at"`
	ActiveTaskCount       int64                      `json:"active_task_count"`
	TaskManagementGranted bool                       `json:"task_management_granted"`
	TaskAccessVersion     int64                      `json:"task_access_version"`
}
