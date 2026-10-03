package master

import (
	"time"

	"github.com/google/uuid"
)

type Catalog string

const (
	CatalogContributionRoles Catalog = "contribution_roles"
	CatalogSkills            Catalog = "skills"
	CatalogTools             Catalog = "tools"
)

type CatalogItem struct {
	ID        uuid.UUID `gorm:"column:id;type:uuid;primaryKey"`
	Name      string    `gorm:"column:name"`
	Slug      string    `gorm:"column:slug"`
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func catalogTable(catalog Catalog) (string, bool) {
	switch catalog {
	case CatalogContributionRoles:
		return "contribution_roles", true
	case CatalogSkills:
		return "skills", true
	case CatalogTools:
		return "tools", true
	default:
		return "", false
	}
}
