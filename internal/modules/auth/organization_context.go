package auth

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type SystemRole string

const (
	SystemRoleUser  SystemRole = "user"
	SystemRoleAdmin SystemRole = "admin"
)

// Unknown or legacy zero-value roles never imply administrative privileges.
func (u *User) EffectiveSystemRole() SystemRole {
	if u != nil && u.SystemRole == SystemRoleAdmin {
		return SystemRoleAdmin
	}
	return SystemRoleUser
}

type Organization struct {
	ID        uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	Name      string    `gorm:"type:varchar(255);not null"`
	Slug      string    `gorm:"type:varchar(100);not null;uniqueIndex"`
	IsActive  bool      `gorm:"not null;default:true"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

type OrganizationMember struct {
	OrganizationID      uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID              uuid.UUID `gorm:"type:uuid;primaryKey"`
	IsActive            bool      `gorm:"not null;default:true"`
	CanInitiateProjects bool      `gorm:"not null;default:false"`
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

func (OrganizationMember) TableName() string { return "organization_members" }

// Always read current account/membership/organization state. A JWT role or a
// consumer-provided organization ID cannot replace these predicates.
func (r *authRepository) ListInitiableOrganizations(ctx context.Context, userID uuid.UUID) ([]InitiableOrganizationResponse, error) {
	organizations := make([]InitiableOrganizationResponse, 0)
	err := r.db.WithContext(ctx).
		Table("organizations AS o").
		Select("o.id, o.name, o.slug").
		Joins("JOIN organization_members AS m ON m.organization_id = o.id").
		Joins("JOIN users AS u ON u.id = m.user_id").
		Where("u.id = ? AND u.system_role = ? AND u.is_active = true AND u.deleted_at IS NULL", userID, SystemRoleAdmin).
		Where("o.is_active = true AND m.is_active = true AND m.can_initiate_projects = true").
		Order("LOWER(o.name), o.id").
		Scan(&organizations).Error
	return organizations, err
}
