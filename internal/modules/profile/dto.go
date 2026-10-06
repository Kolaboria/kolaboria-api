package profile

import (
	"bytes"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

const (
	maxToolsPerProfile      = 5
	maxOnboardingGoalLength = 200
	maxOnboardingBioLength  = 300
)

type SubmitOnboardingRequest struct {
	PrimarySkillID  uuid.UUID       `json:"primarySkillId" validate:"required"`
	ExperienceLevel ExperienceLevel `json:"experienceLevel" validate:"required,oneof=Beginner Intermediate Advanced Expert"`
	ToolIDs         []uuid.UUID     `json:"toolIds" validate:"max=5"`
	Goal            string          `json:"goal" validate:"omitempty,max=200"`
	Bio             string          `json:"bio" validate:"omitempty,max=300"`
	ExternalLink    string          `json:"externalLink" validate:"omitempty,url"`
}

type OnboardingStatusResponse struct {
	IsOnboarded bool `json:"is_onboarded"`
}

type UpdateProfileRequest struct {
	FullName        *string          `json:"fullName"`
	Headline        *string          `json:"headline"`
	Bio             *string          `json:"bio"`
	VillageID       *int64           `json:"villageId"`
	Address         *string          `json:"address"`
	Goal            *string          `json:"goal"`
	ExperienceLevel *ExperienceLevel `json:"experienceLevel"`
	present         map[string]struct{}
}

func (request *UpdateProfileRequest) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return errors.New("profile update must be a JSON object")
	}
	type profileUpdateFields UpdateProfileRequest
	var fields profileUpdateFields
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	var rawFields map[string]json.RawMessage
	if err := json.Unmarshal(data, &rawFields); err != nil {
		return err
	}
	request.FullName, request.Headline, request.Bio = fields.FullName, fields.Headline, fields.Bio
	request.VillageID, request.Address, request.Goal = fields.VillageID, fields.Address, fields.Goal
	request.ExperienceLevel = fields.ExperienceLevel
	request.present = make(map[string]struct{}, len(rawFields))
	for key := range rawFields {
		request.present[key] = struct{}{}
	}
	return nil
}

func (request UpdateProfileRequest) Has(field string) bool {
	_, exists := request.present[field]
	return exists
}

type MyProfileDetailResponse struct {
	ID            uuid.UUID               `json:"id"`
	Username      string                  `json:"username"`
	FullName      *string                 `json:"full_name"`
	Headline      *string                 `json:"headline"`
	Bio           *string                 `json:"bio"`
	Avatar        *string                 `json:"avatar"`
	Location      *string                 `json:"location"`
	VillageID     *int64                  `json:"village_id"`
	Address       *string                 `json:"address"`
	Village       *ProfileVillageResponse `json:"village"`
	AccountType   *string                 `json:"account_type"`
	ExternalLinks map[string]string       `json:"external_links"`
	IsVerified    bool                    `json:"is_verified"`
	CreatedAt     time.Time               `json:"created_at"`
	UpdatedAt     time.Time               `json:"updated_at"`
	TalentProfile *TalentProfileResponse  `json:"talent_profile"`
	Skills        []UserSkillResponse     `json:"skills"`
	Tools         []UserToolResponse      `json:"tools"`
}

type ProfileVillageResponse struct {
	VillageID  int64  `json:"village_id"`
	Village    string `json:"village"`
	District   string `json:"district"`
	Regency    string `json:"regency"`
	Province   string `json:"province"`
	PostalCode string `json:"postal_code"`
}

type TalentProfileResponse struct {
	ExperienceLevel ExperienceLevel `json:"experience_level"`
	Goal            *string         `json:"goal"`
}

type UserSkillResponse struct {
	ID        uuid.UUID `json:"id"`
	SkillID   uuid.UUID `json:"skill_id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	IsPrimary bool      `json:"is_primary"`
}

type UserToolResponse struct {
	ID     uuid.UUID `json:"id"`
	ToolID uuid.UUID `json:"tool_id"`
	Name   string    `json:"name"`
	Slug   string    `json:"slug"`
}

type AddSkillRequest struct {
	SkillID   uuid.UUID `json:"skillId" validate:"required"`
	IsPrimary bool      `json:"isPrimary"`
}

type AddToolRequest struct {
	ToolID uuid.UUID `json:"toolId" validate:"required"`
}

type CareerHistoryRequest struct {
	Title       string  `json:"title" validate:"required,min=2,max=150"`
	Company     string  `json:"company" validate:"required,min=2,max=150"`
	StartYear   int     `json:"startYear" validate:"required,min=1950"`
	StartMonth  *int    `json:"startMonth" validate:"omitempty,min=1,max=12" extensions:"x-nullable"`
	EndYear     *int    `json:"endYear" validate:"omitempty,min=1950" extensions:"x-nullable"`
	EndMonth    *int    `json:"endMonth" validate:"omitempty,min=1,max=12" extensions:"x-nullable"`
	Description *string `json:"description" validate:"omitempty,max=2000"`
}

type CareerHistoryResponse struct {
	ID          uuid.UUID `json:"id"`
	Title       string    `json:"title"`
	Company     string    `json:"company"`
	StartYear   int       `json:"start_year"`
	StartMonth  *int      `json:"start_month" extensions:"x-nullable"`
	EndYear     *int      `json:"end_year" extensions:"x-nullable"`
	EndMonth    *int      `json:"end_month" extensions:"x-nullable"`
	Description *string   `json:"description"`
}
