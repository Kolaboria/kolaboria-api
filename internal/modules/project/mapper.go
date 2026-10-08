package project

import (
	"time"

	"github.com/google/uuid"
)

func mapProjectDetail(aggregate *ProjectAggregate, actorID uuid.UUID, canEdit, canPublish bool) *ProjectDetailResponse {
	_ = actorID
	if aggregate == nil {
		return nil
	}
	roles := make([]ProjectRoleResponse, 0, len(aggregate.Roles))
	for _, item := range aggregate.Roles {
		contributionRole := mapMasterReference(item.ContributionRole)
		if item.ArchivedAt != nil {
			continue
		}
		skillIDs := make([]uuid.UUID, 0, len(item.Skills))
		skills := make([]ProjectMasterResponse, 0, len(item.Skills))
		for _, skill := range item.Skills {
			skillIDs = append(skillIDs, skill.ID)
			skills = append(skills, ProjectMasterResponse{ID: skill.ID, Name: skill.Name, Slug: skill.Slug})
		}
		toolIDs := make([]uuid.UUID, 0, len(item.Tools))
		tools := make([]ProjectMasterResponse, 0, len(item.Tools))
		for _, tool := range item.Tools {
			toolIDs = append(toolIDs, tool.ID)
			tools = append(tools, ProjectMasterResponse{ID: tool.ID, Name: tool.Name, Slug: tool.Slug})
		}
		status := "open"
		if item.FilledCapacity >= item.Capacity {
			status = "filled"
		}
		remaining := item.Capacity - item.FilledCapacity
		if remaining < 0 {
			remaining = 0
		}
		roles = append(roles, ProjectRoleResponse{
			ID: item.ID, ContributionRoleID: cloneUUIDPointer(item.ContributionRoleID), ContributionRole: contributionRole,
			Description: item.Description, Capacity: item.Capacity, Position: item.Position,
			SkillIDs: skillIDs, Skills: skills, ToolIDs: toolIDs, Tools: tools,
			FilledCapacity: item.FilledCapacity, RemainingCapacity: remaining, Status: status,
		})
	}
	var ownerRole *ProjectOwnerRoleResponse
	if aggregate.OwnerRole != nil {
		ownerRole = &ProjectOwnerRoleResponse{
			ContributionRoleID: aggregate.OwnerRole.ID,
			ContributionRole:   ProjectMasterResponse{ID: aggregate.OwnerRole.ID, Name: aggregate.OwnerRole.Name, Slug: aggregate.OwnerRole.Slug},
		}
	}
	allowedActions := make([]string, 0)
	canAcceptContributors := aggregate.Status == ProjectStatusOpen && aggregate.Visibility == "public"
	capabilities := ProjectCapabilitiesResponse{
		CanEditDefinition:     canEdit,
		CanPublish:            canPublish && aggregate.Status == ProjectStatusDraft,
		AllowedStatusActions:  allowedActions,
		CanApply:              false,
		CanClaim:              false,
		CanManageApplications: false,
		CanManageWorkspace:    false,
	}
	return &ProjectDetailResponse{
		ID: aggregate.ID, Slug: aggregate.Slug, Status: aggregate.Status, Version: aggregate.Version,
		CreatedAt: aggregate.CreatedAt, UpdatedAt: aggregate.UpdatedAt, PublishedAt: aggregate.PublishedAt,
		CreationMode: aggregate.CreationMode, InitiatorOrganizationID: cloneUUIDPointer(aggregate.InitiatorOrganizationID),
		Title: aggregate.Title, Summary: aggregate.Summary, Description: aggregate.Description,
		ProjectCategory: aggregate.ProjectCategory, Visibility: aggregate.Visibility, Origin: aggregate.Origin,
		WhyCollaborative: aggregate.WhyCollaborative, ContributorOutcome: aggregate.ContributorOutcome,
		OwnerCommitment: aggregate.OwnerCommitment, LeadExpectations: aggregate.LeadExpectations,
		ClientAcknowledgement: aggregate.ClientAcknowledgement,
		StartDate:             projectDateFromTime(aggregate.StartDate), Deadline: projectDateFromTime(aggregate.Deadline),
		Availability: aggregate.Availability, HoursPerWeek: aggregate.HoursPerWeek,
		CollaborationAgreement: aggregate.CollaborationAgreement,
		Ownership: ProjectOwnershipResponse{
			CreatedByUserID:         aggregate.CreatedByUserID,
			InitiatorUserID:         cloneUUIDPointer(aggregate.InitiatorUserID),
			InitiatorOrganizationID: cloneUUIDPointer(aggregate.InitiatorOrganizationID),
			OwnerID:                 cloneUUIDPointer(aggregate.OwnerID),
			OwnerProfile:            aggregate.OwnerProfile,
			InitiatorProfile:        publicInitiatorProfile(aggregate),
			InitiatorOrganization:   aggregate.InitiatorOrganization,
			ShowInitiator:           aggregate.InitiatorOrganizationID != nil,
			CanAcceptContributors:   canAcceptContributors,
		},
		OwnerRole: ownerRole, Roles: roles, Capabilities: capabilities,
		created: false,
	}
}

func publicInitiatorProfile(aggregate *ProjectAggregate) *ProjectPublicProfileResponse {
	if aggregate == nil || aggregate.InitiatorOrganizationID == nil {
		return nil
	}
	return aggregate.InitiatorProfile
}

func mapMasterReference(reference *MasterReference) *ProjectMasterResponse {
	if reference == nil {
		return nil
	}
	return &ProjectMasterResponse{ID: reference.ID, Name: reference.Name, Slug: reference.Slug}
}

func definitionFromAggregate(aggregate *ProjectAggregate) ProjectDefinitionInput {
	definition := ProjectDefinitionInput{
		CreationMode:            aggregate.CreationMode,
		InitiatorOrganizationID: cloneUUIDPointer(aggregate.InitiatorOrganizationID),
		Title:                   aggregate.Title, Summary: aggregate.Summary, Description: aggregate.Description,
		Slug: aggregate.Slug, ProjectCategory: aggregate.ProjectCategory, Visibility: aggregate.Visibility,
		Origin: aggregate.Origin, WhyCollaborative: aggregate.WhyCollaborative,
		ContributorOutcome: aggregate.ContributorOutcome, OwnerCommitment: aggregate.OwnerCommitment,
		LeadExpectations: aggregate.LeadExpectations, ClientAcknowledgement: aggregate.ClientAcknowledgement,
		StartDate: projectDateFromTime(aggregate.StartDate), Deadline: projectDateFromTime(aggregate.Deadline),
		Availability: aggregate.Availability, HoursPerWeek: aggregate.HoursPerWeek,
		CollaborationAgreement: aggregate.CollaborationAgreement, Roles: make([]ProjectRoleWriteInput, 0, len(aggregate.Roles)),
	}
	if aggregate.OwnerMember != nil {
		definition.OwnerContributionRoleID = cloneUUIDPointer(aggregate.OwnerMember.ContributionRoleID)
	}
	for _, item := range aggregate.Roles {
		if item.ArchivedAt != nil {
			continue
		}
		input := ProjectRoleWriteInput{
			ID: uuidPointer(item.ID), ContributionRoleID: cloneUUIDPointer(item.ContributionRoleID),
			Description: item.Description, Capacity: item.Capacity,
			SkillIDs: make([]uuid.UUID, 0, len(item.Skills)), ToolIDs: make([]uuid.UUID, 0, len(item.Tools)),
		}
		for _, skill := range item.Skills {
			input.SkillIDs = append(input.SkillIDs, skill.ID)
		}
		for _, tool := range item.Tools {
			input.ToolIDs = append(input.ToolIDs, tool.ID)
		}
		definition.Roles = append(definition.Roles, input)
	}
	return definition
}

func projectDateFromTime(value *time.Time) *ProjectDate {
	if value == nil {
		return nil
	}
	return &ProjectDate{Time: *value}
}
