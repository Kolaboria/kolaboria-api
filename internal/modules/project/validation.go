package project

import (
	"context"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	sharedErrors "github.com/syahrulfahmi/kolaboria-api/internal/shared/errors"
)

const (
	maxProjectRoles       = 20
	maxProjectCapacity    = 20
	maxRoleSkills         = 20
	maxRoleTools          = 8
	maxProjectDescription = 20000
)

var projectSlugPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

func validateDefinition(input *ProjectDefinitionInput, creating bool) error {
	input.Title = strings.TrimSpace(input.Title)
	input.Summary = strings.TrimSpace(input.Summary)
	input.Description = strings.TrimSpace(input.Description)
	input.Slug = strings.TrimSpace(input.Slug)
	input.WhyCollaborative = strings.TrimSpace(input.WhyCollaborative)
	input.ContributorOutcome = strings.TrimSpace(input.ContributorOutcome)
	input.OwnerCommitment = strings.TrimSpace(input.OwnerCommitment)
	input.LeadExpectations = strings.TrimSpace(input.LeadExpectations)
	for i := range input.Roles {
		input.Roles[i].Description = strings.TrimSpace(input.Roles[i].Description)
	}

	if input.CreationMode != CreationModePersonal && input.CreationMode != CreationModeOrganizationInitiated {
		return sharedErrors.ErrBadRequest
	}
	if input.CreationMode == CreationModePersonal && input.InitiatorOrganizationID != nil {
		return sharedErrors.ErrBadRequest
	}
	if input.CreationMode == CreationModeOrganizationInitiated && (input.InitiatorOrganizationID == nil || *input.InitiatorOrganizationID == uuid.Nil) {
		return sharedErrors.ErrBadRequest
	}
	if input.ProjectCategory != "product" && input.ProjectCategory != "community" && input.ProjectCategory != "open_source" && input.ProjectCategory != "research" && input.ProjectCategory != "education" && input.ProjectCategory != "business" && input.ProjectCategory != "other" {
		return sharedErrors.ErrBadRequest
	}
	if input.Visibility != "public" && input.Visibility != "private" && input.Visibility != "invite_only" {
		return sharedErrors.ErrBadRequest
	}
	if input.Origin != "personal" && input.Origin != "community" && input.Origin != "experiment" && input.Origin != "client" {
		return sharedErrors.ErrBadRequest
	}
	if input.Availability != "flexible" && input.Availability != "part_time" && input.Availability != "weekends_only" && input.Availability != "full_time" {
		return sharedErrors.ErrBadRequest
	}
	if input.HoursPerWeek != 5 && input.HoursPerWeek != 10 && input.HoursPerWeek != 15 && input.HoursPerWeek != 20 {
		return sharedErrors.ErrBadRequest
	}
	if input.OwnerContributionRoleID != nil && *input.OwnerContributionRoleID == uuid.Nil {
		return sharedErrors.ErrBadRequest
	}
	if input.CreationMode == CreationModeOrganizationInitiated && input.OwnerContributionRoleID != nil {
		return sharedErrors.ErrBadRequest
	}
	if runeLength(input.Title) > 80 || runeLength(input.Summary) > 180 || runeLength(input.Description) > maxProjectDescription ||
		runeLength(input.WhyCollaborative) > 5000 || runeLength(input.ContributorOutcome) > 5000 ||
		runeLength(input.OwnerCommitment) > 5000 || runeLength(input.LeadExpectations) > 5000 {
		return sharedErrors.ErrBadRequest
	}
	if input.Slug == "" {
		if !creating {
			return sharedErrors.ErrBadRequest
		}
	} else if runeLength(input.Slug) > 100 || !projectSlugPattern.MatchString(input.Slug) {
		return sharedErrors.ErrBadRequest
	}
	if input.StartDate != nil && input.Deadline != nil && input.Deadline.Before(input.StartDate.Time) {
		return sharedErrors.ErrBadRequest
	}
	if len(input.Roles) > maxProjectRoles {
		return sharedErrors.ErrBadRequest
	}
	capacity := 0
	seenRoleIDs := make(map[uuid.UUID]struct{}, len(input.Roles))
	for _, role := range input.Roles {
		if role.ID != nil {
			if *role.ID == uuid.Nil {
				return sharedErrors.ErrBadRequest
			}
			if _, exists := seenRoleIDs[*role.ID]; exists {
				return sharedErrors.ErrBadRequest
			}
			seenRoleIDs[*role.ID] = struct{}{}
		}
		if creating && role.ID != nil {
			return sharedErrors.ErrBadRequest
		}
		if role.ContributionRoleID != nil && *role.ContributionRoleID == uuid.Nil {
			return sharedErrors.ErrBadRequest
		}
		if role.Capacity < 1 || role.Capacity > 20 || runeLength(role.Description) > 5000 {
			return sharedErrors.ErrBadRequest
		}
		capacity += role.Capacity
		if len(role.SkillIDs) > maxRoleSkills || len(role.ToolIDs) > maxRoleTools ||
			!uniqueNonZeroUUIDs(role.SkillIDs) || !uniqueNonZeroUUIDs(role.ToolIDs) {
			return sharedErrors.ErrBadRequest
		}
	}
	if capacity > maxProjectCapacity {
		return sharedErrors.ErrBadRequest
	}
	return nil
}

func validatePublishDefinition(input ProjectDefinitionInput) error {
	if input.Title == "" || input.Summary == "" || input.Description == "" || len(input.Roles) == 0 || !input.CollaborationAgreement {
		return sharedErrors.ErrBadRequest
	}
	for _, role := range input.Roles {
		if role.ContributionRoleID == nil || role.Description == "" || len(role.SkillIDs) == 0 {
			return sharedErrors.ErrBadRequest
		}
	}
	if input.WhyCollaborative == "" || input.ContributorOutcome == "" {
		return sharedErrors.ErrBadRequest
	}
	if input.CreationMode == CreationModePersonal && input.OwnerCommitment == "" {
		return sharedErrors.ErrBadRequest
	}
	if input.CreationMode == CreationModeOrganizationInitiated && input.LeadExpectations == "" {
		return sharedErrors.ErrBadRequest
	}
	if input.Origin == "client" && !input.ClientAcknowledgement {
		return sharedErrors.ErrBadRequest
	}
	if input.Origin != "client" && input.ClientAcknowledgement {
		return sharedErrors.ErrBadRequest
	}
	return nil
}

func ValidateRoleChange(existing ProjectRole, input ProjectRoleWriteInput, usage RoleUsage) error {
	if usage.PendingApplications > 0 && !sameUUIDPointer(existing.ContributionRoleID, input.ContributionRoleID) {
		return sharedErrors.ErrConflict
	}
	if usage.ActiveContributors > 0 {
		if !sameUUIDPointer(existing.ContributionRoleID, input.ContributionRoleID) || input.Capacity < usage.ActiveContributors {
			return sharedErrors.ErrConflict
		}
	}
	return nil
}

func validateMasterReferences(tx ProjectTx, ctx context.Context, input ProjectDefinitionInput) (map[uuid.UUID]MasterReference, map[uuid.UUID]MasterReference, map[uuid.UUID]MasterReference, error) {
	roleIDs := make([]uuid.UUID, 0, len(input.Roles)+1)
	skillIDs := make([]uuid.UUID, 0)
	toolIDs := make([]uuid.UUID, 0)
	if input.OwnerContributionRoleID != nil {
		roleIDs = append(roleIDs, *input.OwnerContributionRoleID)
	}
	for _, role := range input.Roles {
		if role.ContributionRoleID != nil {
			roleIDs = append(roleIDs, *role.ContributionRoleID)
		}
		skillIDs = append(skillIDs, role.SkillIDs...)
		toolIDs = append(toolIDs, role.ToolIDs...)
	}
	roleIDs = uniqueUUIDs(roleIDs)
	skillIDs = uniqueUUIDs(skillIDs)
	toolIDs = uniqueUUIDs(toolIDs)
	roles, err := tx.FindMasterReferences(ctx, "contribution_roles", roleIDs)
	if err != nil {
		return nil, nil, nil, err
	}
	skills, err := tx.FindMasterReferences(ctx, "skills", skillIDs)
	if err != nil {
		return nil, nil, nil, err
	}
	tools, err := tx.FindMasterReferences(ctx, "tools", toolIDs)
	if err != nil {
		return nil, nil, nil, err
	}
	if len(roles) != len(roleIDs) || len(skills) != len(skillIDs) || len(tools) != len(toolIDs) {
		return nil, nil, nil, sharedErrors.ErrBadRequest
	}
	return referencesByID(roles), referencesByID(skills), referencesByID(tools), nil
}

func runeLength(value string) int { return utf8.RuneCountInString(value) }

func uniqueNonZeroUUIDs(values []uuid.UUID) bool {
	seen := make(map[uuid.UUID]struct{}, len(values))
	for _, value := range values {
		if value == uuid.Nil {
			return false
		}
		if _, exists := seen[value]; exists {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func uniqueUUIDs(values []uuid.UUID) []uuid.UUID {
	seen := make(map[uuid.UUID]struct{}, len(values))
	result := make([]uuid.UUID, 0, len(values))
	for _, value := range values {
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func referencesByID(values []MasterReference) map[uuid.UUID]MasterReference {
	result := make(map[uuid.UUID]MasterReference, len(values))
	for _, value := range values {
		result[value.ID] = value
	}
	return result
}

func sameUUIDPointer(left, right *uuid.UUID) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}
