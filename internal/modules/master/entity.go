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

type Province struct {
	ID        int64     `gorm:"column:id;primaryKey;autoIncrement"`
	Code      string    `gorm:"column:code;type:varchar(10);not null;unique"`
	Name      string    `gorm:"column:name;type:varchar(150);not null"`
	CreatedAt time.Time `gorm:"column:created_at;type:timestamptz;not null;default:now()"`
	UpdatedAt time.Time `gorm:"column:updated_at;type:timestamptz;not null;default:now()"`
}

func (Province) TableName() string {
	return "provinces"
}

type Regency struct {
	ID         int64     `gorm:"column:id;primaryKey;autoIncrement"`
	ProvinceID int64     `gorm:"column:province_id;not null"`
	Code       string    `gorm:"column:code;type:varchar(20);not null;unique"`
	Name       string    `gorm:"column:name;type:varchar(150);not null"`
	Type       string    `gorm:"column:type;type:varchar(20);not null"` // 'regency' or 'city'
	CreatedAt  time.Time `gorm:"column:created_at;type:timestamptz;not null;default:now()"`
	UpdatedAt  time.Time `gorm:"column:updated_at;type:timestamptz;not null;default:now()"`
}

func (Regency) TableName() string {
	return "regencies"
}

type District struct {
	ID        int64     `gorm:"column:id;primaryKey;autoIncrement"`
	RegencyID int64     `gorm:"column:regency_id;not null"`
	Code      string    `gorm:"column:code;type:varchar(20);not null;unique"`
	Name      string    `gorm:"column:name;type:varchar(150);not null"`
	CreatedAt time.Time `gorm:"column:created_at;type:timestamptz;not null;default:now()"`
	UpdatedAt time.Time `gorm:"column:updated_at;type:timestamptz;not null;default:now()"`
}

func (District) TableName() string {
	return "districts"
}

type Village struct {
	ID         int64     `gorm:"column:id;primaryKey;autoIncrement"`
	DistrictID int64     `gorm:"column:district_id;not null"`
	Code       string    `gorm:"column:code;type:varchar(20);not null;unique"`
	PostalCode string    `gorm:"column:postal_code;type:varchar(10);not null"`
	Name       string    `gorm:"column:name;type:varchar(150);not null"`
	CreatedAt  time.Time `gorm:"column:created_at;type:timestamptz;not null;default:now()"`
	UpdatedAt  time.Time `gorm:"column:updated_at;type:timestamptz;not null;default:now()"`
}

func (Village) TableName() string {
	return "villages"
}

