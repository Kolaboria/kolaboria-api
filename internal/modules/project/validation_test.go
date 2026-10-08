package project

import (
	"testing"

	"github.com/google/uuid"
	sharedErrors "github.com/syahrulfahmi/kolaboria-api/internal/shared/errors"
)

func TestDraftAndPublishHaveDifferentCompletenessRequirements(t *testing.T) {
	draft := ProjectDefinitionInput{
		CreationMode: CreationModePersonal, Title: "", Summary: "", Description: "", ProjectCategory: "product",
		Visibility: "public", Origin: "personal", Availability: "flexible", HoursPerWeek: 5,
	}
	if err := validateDefinition(&draft, true); err != nil {
		t.Fatalf("structurally valid incomplete draft rejected: %v", err)
	}
	if err := validatePublishDefinition(draft); err == nil {
		t.Fatal("incomplete draft unexpectedly passed publish validation")
	}

	roleID := uuid.New()
	skillID := uuid.New()
	draft.Title = "Project title"
	draft.Summary = "Project summary"
	draft.Description = "Project description"
	draft.WhyCollaborative = "Shared effort"
	draft.ContributorOutcome = "A useful result"
	draft.OwnerCommitment = "I will coordinate"
	draft.CollaborationAgreement = true
	draft.Roles = []ProjectRoleWriteInput{{ContributionRoleID: &roleID, Description: "Build the feature", Capacity: 1, SkillIDs: []uuid.UUID{skillID}}}
	if err := validateDefinition(&draft, true); err != nil {
		t.Fatal("complete draft failed structural validation:", err)
	}
	if err := validatePublishDefinition(draft); err != nil {
		t.Fatal("complete personal project failed publish validation:", err)
	}
}

func TestOrganizationCannotSetOwnerProfessionalRole(t *testing.T) {
	roleID := uuid.New()
	definition := ProjectDefinitionInput{
		CreationMode: CreationModeOrganizationInitiated, InitiatorOrganizationID: uuidPointer(uuid.New()),
		ProjectCategory: "product", Visibility: "public", OwnerContributionRoleID: &roleID,
		Origin: "personal", Availability: "flexible", HoursPerWeek: 5,
	}
	if err := validateDefinition(&definition, true); err != sharedErrors.ErrBadRequest {
		t.Fatalf("organization owner role should be rejected, got %v", err)
	}
}

func TestRoleWithActiveMembersKeepsMasterIdentityAndCapacity(t *testing.T) {
	oldRoleID := uuid.New()
	newRoleID := uuid.New()
	existing := ProjectRole{ContributionRoleID: &oldRoleID}
	usage := RoleUsage{ActiveContributors: 2}
	if err := ValidateRoleChange(existing, ProjectRoleWriteInput{ContributionRoleID: &newRoleID, Capacity: 2}, usage); err != sharedErrors.ErrConflict {
		t.Fatalf("changing identity for an active role should conflict, got %v", err)
	}
	if err := ValidateRoleChange(existing, ProjectRoleWriteInput{ContributionRoleID: &oldRoleID, Capacity: 1}, usage); err != sharedErrors.ErrConflict {
		t.Fatalf("capacity below active membership should conflict, got %v", err)
	}
	if err := ValidateRoleChange(existing, ProjectRoleWriteInput{ContributionRoleID: &oldRoleID, Capacity: 2}, usage); err != nil {
		t.Fatalf("unchanged role identity/capacity should remain editable: %v", err)
	}
}
