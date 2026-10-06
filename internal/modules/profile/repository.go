package profile

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	sharedErrors "github.com/syahrulfahmi/kolaboria-api/internal/shared/errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ProfileRepository interface {
	FindByID(ctx context.Context, id uuid.UUID) (*Profile, error)
	SubmitOnboarding(ctx context.Context, userID uuid.UUID, request SubmitOnboardingRequest) error
	GetProfileEditRecord(ctx context.Context, userID uuid.UUID) (*ProfileEditRecord, error)
	UpdateProfile(ctx context.Context, userID uuid.UUID, request UpdateProfileRequest) error
	ListSkills(ctx context.Context, userID uuid.UUID) ([]UserSkillDetail, error)
	AddSkill(ctx context.Context, userID uuid.UUID, request AddSkillRequest) error
	RemoveSkill(ctx context.Context, userID, userSkillID uuid.UUID) error
	SetPrimarySkill(ctx context.Context, userID, userSkillID uuid.UUID) error
	ListTools(ctx context.Context, userID uuid.UUID) ([]UserToolDetail, error)
	AddTool(ctx context.Context, userID uuid.UUID, request AddToolRequest) error
	RemoveTool(ctx context.Context, userID, userToolID uuid.UUID) error
	ListCareerHistories(ctx context.Context, userID uuid.UUID) ([]CareerHistory, error)
	CreateCareerHistory(ctx context.Context, userID uuid.UUID, request CareerHistoryRequest) (*CareerHistory, error)
	UpdateCareerHistory(ctx context.Context, userID, careerID uuid.UUID, request CareerHistoryRequest) (*CareerHistory, error)
	DeleteCareerHistory(ctx context.Context, userID, careerID uuid.UUID) error
}

type gormProfileRepository struct {
	db *gorm.DB
}

func NewProfileRepository(db *gorm.DB) ProfileRepository {
	return &gormProfileRepository{db: db}
}

func (r *gormProfileRepository) FindByID(ctx context.Context, id uuid.UUID) (*Profile, error) {
	var profile Profile
	err := r.db.WithContext(ctx).
		Select("id", "account_type").
		Where("id = ?", id).
		First(&profile).Error
	if err != nil {
		return nil, err
	}
	return &profile, nil
}

func (r *gormProfileRepository) SubmitOnboarding(ctx context.Context, userID uuid.UUID, request SubmitOnboardingRequest) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var profile Profile
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Select("id").
			Where("id = ?", userID).
			Take(&profile).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("lock onboarding profile: %w", sharedErrors.ErrNotFound)
			}
			return fmt.Errorf("lock onboarding profile: %w", err)
		}

		var skillCount int64
		if err := tx.Table("skills").Where("id = ?", request.PrimarySkillID).Count(&skillCount).Error; err != nil {
			return fmt.Errorf("validate onboarding skill: %w", err)
		}
		if skillCount != 1 {
			return sharedErrors.ErrBadRequest
		}

		if len(request.ToolIDs) > 0 {
			var toolCount int64
			if err := tx.Table("tools").Where("id IN ?", request.ToolIDs).Count(&toolCount).Error; err != nil {
				return fmt.Errorf("validate onboarding tools: %w", err)
			}
			if toolCount != int64(len(request.ToolIDs)) {
				return sharedErrors.ErrBadRequest
			}
		}

		var bio any
		if request.Bio != "" {
			bio = request.Bio
		}
		profileUpdates := map[string]any{
			"account_type": "talent",
			"bio":          bio,
			"external_links": gorm.Expr(
				"CASE WHEN ? = '' THEN external_links - 'primary' ELSE jsonb_set(external_links, '{primary}', to_jsonb(?::text), true) END",
				request.ExternalLink,
				request.ExternalLink,
			),
			"updated_at": gorm.Expr("NOW()"),
		}
		result := tx.Model(&Profile{}).Where("id = ?", userID).Updates(profileUpdates)
		if result.Error != nil {
			return fmt.Errorf("update onboarding profile: %w", result.Error)
		}
		if result.RowsAffected != 1 {
			return fmt.Errorf("update onboarding profile: %w", sharedErrors.ErrNotFound)
		}

		var goal *string
		if request.Goal != "" {
			goal = &request.Goal
		}
		talentProfile := TalentProfile{
			UserID:          userID,
			ExperienceLevel: request.ExperienceLevel,
			Goal:            goal,
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"experience_level", "goal", "updated_at"}),
		}).Create(&talentProfile).Error; err != nil {
			return fmt.Errorf("save onboarding talent profile: %w", err)
		}

		if err := tx.Where("user_id = ?", userID).Delete(&UserSkill{}).Error; err != nil {
			return fmt.Errorf("replace onboarding skills: %w", err)
		}
		if err := tx.Where("user_id = ?", userID).Delete(&UserTool{}).Error; err != nil {
			return fmt.Errorf("replace onboarding tools: %w", err)
		}

		primarySkill := UserSkill{UserID: userID, SkillID: request.PrimarySkillID, IsPrimary: true}
		if err := tx.Create(&primarySkill).Error; err != nil {
			return fmt.Errorf("save onboarding primary skill: %w", err)
		}

		if len(request.ToolIDs) > 0 {
			userTools := make([]UserTool, 0, len(request.ToolIDs))
			for _, toolID := range request.ToolIDs {
				userTools = append(userTools, UserTool{UserID: userID, ToolID: toolID})
			}
			if err := tx.Create(&userTools).Error; err != nil {
				return fmt.Errorf("save onboarding tools: %w", err)
			}
		}

		return nil
	})
}
