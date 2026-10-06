package profile

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type ExperienceLevel string

const (
	ExperienceLevelBeginner     ExperienceLevel = "Beginner"
	ExperienceLevelIntermediate ExperienceLevel = "Intermediate"
	ExperienceLevelAdvanced     ExperienceLevel = "Advanced"
	ExperienceLevelExpert       ExperienceLevel = "Expert"
)

func (level ExperienceLevel) IsValid() bool {
	switch level {
	case ExperienceLevelBeginner, ExperienceLevelIntermediate, ExperienceLevelAdvanced, ExperienceLevelExpert:
		return true
	default:
		return false
	}
}

type Profile struct {
	ID            uuid.UUID       `gorm:"column:id"`
	Username      string          `gorm:"column:username"`
	FullName      *string         `gorm:"column:full_name"`
	Headline      *string         `gorm:"column:headline"`
	Bio           *string         `gorm:"column:bio"`
	Avatar        *string         `gorm:"column:avatar"`
	Location      *string         `gorm:"column:location"`
	VillageID     *int64          `gorm:"column:village_id"`
	Address       *string         `gorm:"column:address"`
	AccountType   *string         `gorm:"column:account_type"`
	ExternalLinks json.RawMessage `gorm:"column:external_links;type:jsonb"`
	IsVerified    bool            `gorm:"column:is_verified"`
	CreatedAt     time.Time       `gorm:"column:created_at"`
	UpdatedAt     time.Time       `gorm:"column:updated_at"`
}

func (Profile) TableName() string {
	return "profiles"
}

type TalentProfile struct {
	UserID          uuid.UUID       `gorm:"column:user_id;type:uuid;primaryKey"`
	ExperienceLevel ExperienceLevel `gorm:"column:experience_level;type:varchar(20);not null"`
	Goal            *string         `gorm:"column:goal;type:text"`
	CreatedAt       time.Time       `gorm:"column:created_at;not null;autoCreateTime"`
	UpdatedAt       time.Time       `gorm:"column:updated_at;not null;autoUpdateTime"`
}

func (TalentProfile) TableName() string {
	return "talent_profiles"
}

type UserSkill struct {
	ID        uuid.UUID `gorm:"column:id;type:uuid;primaryKey;default:gen_random_uuid()"`
	UserID    uuid.UUID `gorm:"column:user_id;type:uuid;not null"`
	SkillID   uuid.UUID `gorm:"column:skill_id;type:uuid;not null"`
	IsPrimary bool      `gorm:"column:is_primary;not null;default:false"`
	CreatedAt time.Time `gorm:"column:created_at;not null;autoCreateTime"`
}

func (UserSkill) TableName() string {
	return "user_skills"
}

type UserTool struct {
	ID        uuid.UUID `gorm:"column:id;type:uuid;primaryKey;default:gen_random_uuid()"`
	UserID    uuid.UUID `gorm:"column:user_id;type:uuid;not null"`
	ToolID    uuid.UUID `gorm:"column:tool_id;type:uuid;not null"`
	CreatedAt time.Time `gorm:"column:created_at;not null;autoCreateTime"`
}

func (UserTool) TableName() string {
	return "user_tools"
}

type CareerHistory struct {
	ID          uuid.UUID `gorm:"column:id;type:uuid;primaryKey;default:gen_random_uuid()"`
	UserID      uuid.UUID `gorm:"column:user_id;type:uuid;not null"`
	Title       string    `gorm:"column:title;not null"`
	Company     string    `gorm:"column:company;not null"`
	StartYear   int       `gorm:"column:start_year;not null"`
	StartMonth  *int      `gorm:"column:start_month"`
	EndYear     *int      `gorm:"column:end_year"`
	EndMonth    *int      `gorm:"column:end_month"`
	Description *string   `gorm:"column:description"`
	CreatedAt   time.Time `gorm:"column:created_at;not null;autoCreateTime"`
	UpdatedAt   time.Time `gorm:"column:updated_at;not null;autoUpdateTime"`
}

func (CareerHistory) TableName() string { return "career_histories" }

type ProfileVillage struct {
	VillageID  int64  `gorm:"column:village_id"`
	Village    string `gorm:"column:village"`
	District   string `gorm:"column:district"`
	Regency    string `gorm:"column:regency"`
	Province   string `gorm:"column:province"`
	PostalCode string `gorm:"column:postal_code"`
}

type UserSkillDetail struct {
	UserSkill
	Name string `gorm:"column:name"`
	Slug string `gorm:"column:slug"`
}

type UserToolDetail struct {
	UserTool
	Name string `gorm:"column:name"`
	Slug string `gorm:"column:slug"`
}
