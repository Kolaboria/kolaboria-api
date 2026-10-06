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

type ProfileEditRecord struct {
	Profile Profile
	Talent  *TalentProfile
	Village *ProfileVillage
	Skills  []UserSkillDetail
	Tools   []UserToolDetail
}

func (r *gormProfileRepository) GetProfileEditRecord(ctx context.Context, userID uuid.UUID) (*ProfileEditRecord, error) {
	var profile Profile
	if err := r.db.WithContext(ctx).Where("id = ?", userID).First(&profile).Error; err != nil {
		return nil, err
	}
	record := &ProfileEditRecord{Profile: profile}
	if profile.VillageID != nil {
		var village ProfileVillage
		err := r.db.WithContext(ctx).Table("villages v").
			Select("v.id AS village_id, v.name AS village, d.name AS district, r.name AS regency, p.name AS province, v.postal_code").
			Joins("JOIN districts d ON d.id = v.district_id").
			Joins("JOIN regencies r ON r.id = d.regency_id").
			Joins("JOIN provinces p ON p.id = r.province_id").
			Where("v.id = ?", *profile.VillageID).Take(&village).Error
		if err != nil {
			return nil, fmt.Errorf("load profile village: %w", err)
		}
		record.Village = &village
	}
	var talent TalentProfile
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).Take(&talent).Error
	if err == nil {
		record.Talent = &talent
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("load talent profile: %w", err)
	}
	record.Skills, err = r.ListSkills(ctx, userID)
	if err != nil {
		return nil, err
	}
	record.Tools, err = r.ListTools(ctx, userID)
	if err != nil {
		return nil, err
	}
	return record, nil
}

func (r *gormProfileRepository) UpdateProfile(ctx context.Context, userID uuid.UUID, request UpdateProfileRequest) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var profile Profile
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id").Where("id = ?", userID).Take(&profile).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return sharedErrors.ErrNotFound
			}
			return fmt.Errorf("lock profile for edit: %w", err)
		}

		updates := make(map[string]any)
		if request.Has("fullName") {
			updates["full_name"] = request.FullName
		}
		if request.Has("headline") {
			updates["headline"] = request.Headline
		}
		if request.Has("bio") {
			updates["bio"] = request.Bio
		}
		if request.Has("address") {
			updates["address"] = request.Address
		}
		if request.Has("villageId") {
			updates["village_id"] = request.VillageID
			if request.VillageID == nil {
				updates["location"] = nil
			} else {
				var location string
				result := tx.Table("villages v").Select("CONCAT_WS(', ', v.name, d.name, r.name, p.name)").
					Joins("JOIN districts d ON d.id = v.district_id").
					Joins("JOIN regencies r ON r.id = d.regency_id").
					Joins("JOIN provinces p ON p.id = r.province_id").
					Where("v.id = ?", *request.VillageID).Scan(&location)
				if result.Error != nil {
					return fmt.Errorf("resolve profile village: %w", result.Error)
				}
				if result.RowsAffected == 0 || location == "" {
					return sharedErrors.ErrBadRequest
				}
				updates["location"] = location
			}
		}
		if len(updates) > 0 {
			updates["updated_at"] = gorm.Expr("NOW()")
			if err := tx.Model(&Profile{}).Where("id = ?", userID).Updates(updates).Error; err != nil {
				return fmt.Errorf("update profile fields: %w", err)
			}
		}
		talentUpdates := make(map[string]any)
		if request.Has("goal") {
			talentUpdates["goal"] = request.Goal
		}
		if request.Has("experienceLevel") {
			talentUpdates["experience_level"] = request.ExperienceLevel
		}
		if len(talentUpdates) > 0 {
			talentUpdates["updated_at"] = gorm.Expr("NOW()")
			result := tx.Model(&TalentProfile{}).Where("user_id = ?", userID).Updates(talentUpdates)
			if result.Error != nil {
				return fmt.Errorf("update talent profile: %w", result.Error)
			}
			if result.RowsAffected == 0 {
				return sharedErrors.ErrConflict
			}
		}
		return nil
	})
}

func (r *gormProfileRepository) ListSkills(ctx context.Context, userID uuid.UUID) ([]UserSkillDetail, error) {
	var skills []UserSkillDetail
	err := r.db.WithContext(ctx).Table("user_skills us").
		Select("us.id, us.user_id, us.skill_id, us.is_primary, us.created_at, s.name, s.slug").
		Joins("JOIN skills s ON s.id = us.skill_id").Where("us.user_id = ?", userID).
		Order("us.is_primary DESC, us.created_at ASC, us.id ASC").Scan(&skills).Error
	return skills, err
}

func (r *gormProfileRepository) AddSkill(ctx context.Context, userID uuid.UUID, request AddSkillRequest) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockProfile(tx, userID); err != nil {
			return err
		}
		var count int64
		if err := tx.Table("skills").Where("id = ?", request.SkillID).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return sharedErrors.ErrBadRequest
		}
		if err := tx.Model(&UserSkill{}).Where("user_id = ? AND skill_id = ?", userID, request.SkillID).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return sharedErrors.ErrConflict
		}
		var primaryCount int64
		if err := tx.Model(&UserSkill{}).Where("user_id = ? AND is_primary = true", userID).Count(&primaryCount).Error; err != nil {
			return err
		}
		primary := request.IsPrimary || primaryCount == 0
		if primary {
			if err := tx.Model(&UserSkill{}).Where("user_id = ? AND is_primary = true", userID).Update("is_primary", false).Error; err != nil {
				return err
			}
		}
		return tx.Create(&UserSkill{UserID: userID, SkillID: request.SkillID, IsPrimary: primary}).Error
	})
}

func (r *gormProfileRepository) RemoveSkill(ctx context.Context, userID, userSkillID uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockProfile(tx, userID); err != nil {
			return err
		}
		var skill UserSkill
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND user_id = ?", userSkillID, userID).Take(&skill).Error; err != nil {
			return notFoundIfMissing(err)
		}
		var count int64
		if err := tx.Model(&UserSkill{}).Where("user_id = ?", userID).Count(&count).Error; err != nil {
			return err
		}
		if skill.IsPrimary && count == 1 {
			return sharedErrors.ErrConflict
		}
		if err := tx.Delete(&skill).Error; err != nil {
			return err
		}
		if skill.IsPrimary {
			var next UserSkill
			if err := tx.Where("user_id = ?", userID).Order("created_at ASC, id ASC").Take(&next).Error; err == nil {
				return tx.Model(&next).Update("is_primary", true).Error
			} else if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		}
		return nil
	})
}

func (r *gormProfileRepository) SetPrimarySkill(ctx context.Context, userID, userSkillID uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockProfile(tx, userID); err != nil {
			return err
		}
		var skill UserSkill
		if err := tx.Where("id = ? AND user_id = ?", userSkillID, userID).Take(&skill).Error; err != nil {
			return notFoundIfMissing(err)
		}
		if err := tx.Model(&UserSkill{}).Where("user_id = ? AND is_primary = true", userID).Update("is_primary", false).Error; err != nil {
			return err
		}
		return tx.Model(&skill).Update("is_primary", true).Error
	})
}

func (r *gormProfileRepository) ListTools(ctx context.Context, userID uuid.UUID) ([]UserToolDetail, error) {
	var tools []UserToolDetail
	err := r.db.WithContext(ctx).Table("user_tools ut").
		Select("ut.id, ut.user_id, ut.tool_id, ut.created_at, t.name, t.slug").
		Joins("JOIN tools t ON t.id = ut.tool_id").Where("ut.user_id = ?", userID).
		Order("ut.created_at ASC, ut.id ASC").Scan(&tools).Error
	return tools, err
}

func (r *gormProfileRepository) AddTool(ctx context.Context, userID uuid.UUID, request AddToolRequest) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockProfile(tx, userID); err != nil {
			return err
		}
		var count int64
		if err := tx.Table("tools").Where("id = ?", request.ToolID).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return sharedErrors.ErrBadRequest
		}
		if err := tx.Model(&UserTool{}).Where("user_id = ? AND tool_id = ?", userID, request.ToolID).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return sharedErrors.ErrConflict
		}
		if err := tx.Model(&UserTool{}).Where("user_id = ?", userID).Count(&count).Error; err != nil {
			return err
		}
		if count >= maxToolsPerProfile {
			return sharedErrors.ErrConflict
		}
		return tx.Create(&UserTool{UserID: userID, ToolID: request.ToolID}).Error
	})
}

func (r *gormProfileRepository) RemoveTool(ctx context.Context, userID, userToolID uuid.UUID) error {
	result := r.db.WithContext(ctx).Where("id = ? AND user_id = ?", userToolID, userID).Delete(&UserTool{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return sharedErrors.ErrNotFound
	}
	return nil
}

func (r *gormProfileRepository) ListCareerHistories(ctx context.Context, userID uuid.UUID) ([]CareerHistory, error) {
	var careers []CareerHistory
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).Order("start_year DESC, start_month DESC NULLS LAST, created_at DESC, id ASC").Find(&careers).Error
	return careers, err
}

func (r *gormProfileRepository) CreateCareerHistory(ctx context.Context, userID uuid.UUID, request CareerHistoryRequest) (*CareerHistory, error) {
	career := CareerHistory{
		UserID: userID, Title: request.Title, Company: request.Company,
		StartYear: request.StartYear, StartMonth: request.StartMonth,
		EndYear: request.EndYear, EndMonth: request.EndMonth, Description: request.Description,
	}
	if err := r.db.WithContext(ctx).Create(&career).Error; err != nil {
		return nil, err
	}
	return &career, nil
}

func (r *gormProfileRepository) UpdateCareerHistory(ctx context.Context, userID, careerID uuid.UUID, request CareerHistoryRequest) (*CareerHistory, error) {
	var career CareerHistory
	err := r.db.WithContext(ctx).Where("id = ? AND user_id = ?", careerID, userID).Take(&career).Error
	if err != nil {
		return nil, notFoundIfMissing(err)
	}
	career.Title, career.Company = request.Title, request.Company
	career.StartYear, career.EndYear, career.Description = request.StartYear, request.EndYear, request.Description
	career.StartMonth, career.EndMonth = request.StartMonth, request.EndMonth
	if err := r.db.WithContext(ctx).Save(&career).Error; err != nil {
		return nil, err
	}
	return &career, nil
}

func (r *gormProfileRepository) DeleteCareerHistory(ctx context.Context, userID, careerID uuid.UUID) error {
	result := r.db.WithContext(ctx).Where("id = ? AND user_id = ?", careerID, userID).Delete(&CareerHistory{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return sharedErrors.ErrNotFound
	}
	return nil
}

func lockProfile(tx *gorm.DB, userID uuid.UUID) error {
	var profile Profile
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id").Where("id = ?", userID).Take(&profile).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return sharedErrors.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("lock profile: %w", err)
	}
	return nil
}

func notFoundIfMissing(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return sharedErrors.ErrNotFound
	}
	return err
}
