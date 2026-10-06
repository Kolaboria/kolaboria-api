package profile

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	sharedErrors "github.com/syahrulfahmi/kolaboria-api/internal/shared/errors"
	"gorm.io/gorm"
)

type ProfileUseCase interface {
	CheckOnboardingStatus(ctx context.Context, userID uuid.UUID) (*OnboardingStatusResponse, error)
	SubmitOnboarding(ctx context.Context, userID uuid.UUID, request SubmitOnboardingRequest) error
	GetMyProfile(ctx context.Context, userID uuid.UUID) (*MyProfileDetailResponse, error)
	UpdateProfile(ctx context.Context, userID uuid.UUID, request UpdateProfileRequest) (*MyProfileDetailResponse, error)
	ListSkills(ctx context.Context, userID uuid.UUID) ([]UserSkillResponse, error)
	AddSkill(ctx context.Context, userID uuid.UUID, request AddSkillRequest) (*UserSkillResponse, error)
	RemoveSkill(ctx context.Context, userID, userSkillID uuid.UUID) error
	SetPrimarySkill(ctx context.Context, userID, userSkillID uuid.UUID) error
	ListTools(ctx context.Context, userID uuid.UUID) ([]UserToolResponse, error)
	AddTool(ctx context.Context, userID uuid.UUID, request AddToolRequest) (*UserToolResponse, error)
	RemoveTool(ctx context.Context, userID, userToolID uuid.UUID) error
	ListCareerHistories(ctx context.Context, userID uuid.UUID) ([]CareerHistoryResponse, error)
	CreateCareerHistory(ctx context.Context, userID uuid.UUID, request CareerHistoryRequest) (*CareerHistoryResponse, error)
	UpdateCareerHistory(ctx context.Context, userID, careerID uuid.UUID, request CareerHistoryRequest) (*CareerHistoryResponse, error)
	DeleteCareerHistory(ctx context.Context, userID, careerID uuid.UUID) error
}

type ProfileService struct {
	repository ProfileRepository
}

func NewProfileService(repository ProfileRepository) *ProfileService {
	return &ProfileService{repository: repository}
}

func (s *ProfileService) CheckOnboardingStatus(ctx context.Context, userID uuid.UUID) (*OnboardingStatusResponse, error) {
	profile, err := s.repository.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("%w: profile", sharedErrors.ErrNotFound)
		}
		return nil, fmt.Errorf("find profile onboarding status: %w", err)
	}

	return &OnboardingStatusResponse{IsOnboarded: profile.AccountType != nil}, nil
}

func (s *ProfileService) SubmitOnboarding(ctx context.Context, userID uuid.UUID, request SubmitOnboardingRequest) error {
	request.Goal = strings.TrimSpace(request.Goal)
	request.Bio = strings.TrimSpace(request.Bio)
	request.ExternalLink = strings.TrimSpace(request.ExternalLink)

	if userID == uuid.Nil || request.PrimarySkillID == uuid.Nil || !request.ExperienceLevel.IsValid() {
		return sharedErrors.ErrBadRequest
	}
	if len(request.ToolIDs) > maxToolsPerProfile {
		return sharedErrors.ErrBadRequest
	}

	seenToolIDs := make(map[uuid.UUID]struct{}, len(request.ToolIDs))
	for _, toolID := range request.ToolIDs {
		if toolID == uuid.Nil {
			return sharedErrors.ErrBadRequest
		}
		if _, exists := seenToolIDs[toolID]; exists {
			return sharedErrors.ErrBadRequest
		}
		seenToolIDs[toolID] = struct{}{}
	}

	if utf8.RuneCountInString(request.Goal) > maxOnboardingGoalLength ||
		utf8.RuneCountInString(request.Bio) > maxOnboardingBioLength {
		return sharedErrors.ErrBadRequest
	}

	return s.repository.SubmitOnboarding(ctx, userID, request)
}

func (s *ProfileService) GetMyProfile(ctx context.Context, userID uuid.UUID) (*MyProfileDetailResponse, error) {
	record, err := s.repository.GetProfileEditRecord(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, sharedErrors.ErrNotFound
		}
		return nil, fmt.Errorf("get profile for edit: %w", err)
	}
	return mapProfileEditRecord(record), nil
}

func (s *ProfileService) UpdateProfile(ctx context.Context, userID uuid.UUID, request UpdateProfileRequest) (*MyProfileDetailResponse, error) {
	if userID == uuid.Nil {
		return nil, sharedErrors.ErrUnauthorized
	}
	trimRequestString(&request.FullName, request.Has("fullName"))
	trimRequestString(&request.Headline, request.Has("headline"))
	trimRequestString(&request.Bio, request.Has("bio"))
	trimRequestString(&request.Address, request.Has("address"))
	trimRequestString(&request.Goal, request.Has("goal"))
	if exceedsLength(request.FullName, request.Has("fullName"), 255) || exceedsLength(request.Headline, request.Has("headline"), 500) ||
		exceedsLength(request.Bio, request.Has("bio"), maxOnboardingBioLength) || exceedsLength(request.Address, request.Has("address"), 500) ||
		exceedsLength(request.Goal, request.Has("goal"), maxOnboardingGoalLength) {
		return nil, sharedErrors.ErrBadRequest
	}
	if request.Has("villageId") && request.VillageID != nil && *request.VillageID <= 0 {
		return nil, sharedErrors.ErrBadRequest
	}
	if request.Has("experienceLevel") && (request.ExperienceLevel == nil || !request.ExperienceLevel.IsValid()) {
		return nil, sharedErrors.ErrBadRequest
	}
	if err := s.repository.UpdateProfile(ctx, userID, request); err != nil {
		return nil, err
	}
	return s.GetMyProfile(ctx, userID)
}

func (s *ProfileService) ListSkills(ctx context.Context, userID uuid.UUID) ([]UserSkillResponse, error) {
	rows, err := s.repository.ListSkills(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list profile skills: %w", err)
	}
	result := make([]UserSkillResponse, 0, len(rows))
	for _, row := range rows {
		result = append(result, UserSkillResponse{ID: row.ID, SkillID: row.SkillID, Name: row.Name, Slug: row.Slug, IsPrimary: row.IsPrimary})
	}
	return result, nil
}

func (s *ProfileService) AddSkill(ctx context.Context, userID uuid.UUID, request AddSkillRequest) (*UserSkillResponse, error) {
	if userID == uuid.Nil || request.SkillID == uuid.Nil {
		return nil, sharedErrors.ErrBadRequest
	}
	if err := s.repository.AddSkill(ctx, userID, request); err != nil {
		return nil, err
	}
	rows, err := s.ListSkills(ctx, userID)
	if err != nil {
		return nil, err
	}
	for i := range rows {
		if rows[i].SkillID == request.SkillID {
			return &rows[i], nil
		}
	}
	return nil, sharedErrors.ErrNotFound
}

func (s *ProfileService) RemoveSkill(ctx context.Context, userID, skillID uuid.UUID) error {
	return s.repository.RemoveSkill(ctx, userID, skillID)
}
func (s *ProfileService) SetPrimarySkill(ctx context.Context, userID, skillID uuid.UUID) error {
	return s.repository.SetPrimarySkill(ctx, userID, skillID)
}

func (s *ProfileService) ListTools(ctx context.Context, userID uuid.UUID) ([]UserToolResponse, error) {
	rows, err := s.repository.ListTools(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list profile tools: %w", err)
	}
	result := make([]UserToolResponse, 0, len(rows))
	for _, row := range rows {
		result = append(result, UserToolResponse{ID: row.ID, ToolID: row.ToolID, Name: row.Name, Slug: row.Slug})
	}
	return result, nil
}

func (s *ProfileService) AddTool(ctx context.Context, userID uuid.UUID, request AddToolRequest) (*UserToolResponse, error) {
	if userID == uuid.Nil || request.ToolID == uuid.Nil {
		return nil, sharedErrors.ErrBadRequest
	}
	if err := s.repository.AddTool(ctx, userID, request); err != nil {
		return nil, err
	}
	rows, err := s.ListTools(ctx, userID)
	if err != nil {
		return nil, err
	}
	for i := range rows {
		if rows[i].ToolID == request.ToolID {
			return &rows[i], nil
		}
	}
	return nil, sharedErrors.ErrNotFound
}

func (s *ProfileService) RemoveTool(ctx context.Context, userID, toolID uuid.UUID) error {
	return s.repository.RemoveTool(ctx, userID, toolID)
}

func (s *ProfileService) ListCareerHistories(ctx context.Context, userID uuid.UUID) ([]CareerHistoryResponse, error) {
	rows, err := s.repository.ListCareerHistories(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list profile careers: %w", err)
	}
	result := make([]CareerHistoryResponse, 0, len(rows))
	for _, row := range rows {
		result = append(result, mapCareerHistory(row))
	}
	return result, nil
}

func (s *ProfileService) CreateCareerHistory(ctx context.Context, userID uuid.UUID, request CareerHistoryRequest) (*CareerHistoryResponse, error) {
	if userID == uuid.Nil {
		return nil, sharedErrors.ErrUnauthorized
	}
	if err := validateCareerRequest(&request); err != nil {
		return nil, err
	}
	row, err := s.repository.CreateCareerHistory(ctx, userID, request)
	if err != nil {
		return nil, err
	}
	result := mapCareerHistory(*row)
	return &result, nil
}

func (s *ProfileService) UpdateCareerHistory(ctx context.Context, userID, careerID uuid.UUID, request CareerHistoryRequest) (*CareerHistoryResponse, error) {
	if userID == uuid.Nil || careerID == uuid.Nil {
		return nil, sharedErrors.ErrBadRequest
	}
	if err := validateCareerRequest(&request); err != nil {
		return nil, err
	}
	row, err := s.repository.UpdateCareerHistory(ctx, userID, careerID, request)
	if err != nil {
		return nil, err
	}
	result := mapCareerHistory(*row)
	return &result, nil
}

func (s *ProfileService) DeleteCareerHistory(ctx context.Context, userID, careerID uuid.UUID) error {
	return s.repository.DeleteCareerHistory(ctx, userID, careerID)
}

func trimRequestString(field **string, provided bool) {
	if provided && *field != nil {
		value := strings.TrimSpace(**field)
		*field = &value
	}
}

func exceedsLength(field *string, provided bool, max int) bool {
	return provided && field != nil && utf8.RuneCountInString(*field) > max
}

func validateCareerRequest(request *CareerHistoryRequest) error {
	request.Title = strings.TrimSpace(request.Title)
	request.Company = strings.TrimSpace(request.Company)
	if request.Description != nil {
		description := strings.TrimSpace(*request.Description)
		request.Description = &description
	}
	now := time.Now()
	if utf8.RuneCountInString(request.Title) < 2 || utf8.RuneCountInString(request.Title) > 150 ||
		utf8.RuneCountInString(request.Company) < 2 || utf8.RuneCountInString(request.Company) > 150 ||
		!validCareerPeriod(request.StartYear, request.StartMonth, now) ||
		(request.Description != nil && utf8.RuneCountInString(*request.Description) > 2000) {
		return sharedErrors.ErrBadRequest
	}
	if request.EndYear == nil {
		if request.EndMonth != nil {
			return sharedErrors.ErrBadRequest
		}
		return nil
	}
	if !validCareerPeriod(*request.EndYear, request.EndMonth, now) || *request.EndYear < request.StartYear {
		return sharedErrors.ErrBadRequest
	}
	if *request.EndYear == request.StartYear && request.StartMonth != nil && request.EndMonth != nil && *request.EndMonth < *request.StartMonth {
		return sharedErrors.ErrBadRequest
	}
	return nil
}

func validCareerPeriod(year int, month *int, now time.Time) bool {
	if year < 1950 || year > now.Year() {
		return false
	}
	if month == nil {
		return true
	}
	return *month >= 1 && *month <= 12 && (year < now.Year() || *month <= int(now.Month()))
}
