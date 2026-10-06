package profile

import "encoding/json"

func mapProfileEditRecord(record *ProfileEditRecord) *MyProfileDetailResponse {
	profile := record.Profile
	links := make(map[string]string)
	if len(profile.ExternalLinks) > 0 {
		_ = json.Unmarshal(profile.ExternalLinks, &links)
	}
	response := &MyProfileDetailResponse{
		ID: profile.ID, Username: profile.Username, FullName: profile.FullName,
		Headline: profile.Headline, Bio: profile.Bio, Avatar: profile.Avatar,
		Location: profile.Location, VillageID: profile.VillageID, Address: profile.Address,
		AccountType: profile.AccountType, ExternalLinks: links, IsVerified: profile.IsVerified,
		CreatedAt: profile.CreatedAt, UpdatedAt: profile.UpdatedAt,
		Skills: make([]UserSkillResponse, 0, len(record.Skills)),
		Tools:  make([]UserToolResponse, 0, len(record.Tools)),
	}
	if record.Village != nil {
		village := record.Village
		response.Village = &ProfileVillageResponse{
			VillageID: village.VillageID, Village: village.Village, District: village.District,
			Regency: village.Regency, Province: village.Province, PostalCode: village.PostalCode,
		}
	}
	if record.Talent != nil {
		response.TalentProfile = &TalentProfileResponse{ExperienceLevel: record.Talent.ExperienceLevel, Goal: record.Talent.Goal}
	}
	for _, skill := range record.Skills {
		response.Skills = append(response.Skills, UserSkillResponse{ID: skill.ID, SkillID: skill.SkillID, Name: skill.Name, Slug: skill.Slug, IsPrimary: skill.IsPrimary})
	}
	for _, tool := range record.Tools {
		response.Tools = append(response.Tools, UserToolResponse{ID: tool.ID, ToolID: tool.ToolID, Name: tool.Name, Slug: tool.Slug})
	}
	return response
}

func mapCareerHistory(career CareerHistory) CareerHistoryResponse {
	return CareerHistoryResponse{
		ID: career.ID, Title: career.Title, Company: career.Company,
		StartYear: career.StartYear, StartMonth: career.StartMonth,
		EndYear: career.EndYear, EndMonth: career.EndMonth, Description: career.Description,
	}
}
