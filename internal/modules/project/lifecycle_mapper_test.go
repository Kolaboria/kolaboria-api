package project

import (
	"github.com/google/uuid"
	"testing"
)

func TestClaimedOrganizationDefinitionKeepsOwnerRoleReadOnly(t *testing.T) {
	ownerID, roleID := uuid.New(), uuid.New()
	aggregate := &ProjectAggregate{
		Project:     Project{CreationMode: CreationModeOrganizationInitiated, OwnerID: &ownerID},
		OwnerMember: &ProjectMember{UserID: ownerID, MembershipRole: "owner", ContributionRoleID: &roleID},
	}
	definition := definitionFromAggregate(aggregate)
	if definition.OwnerContributionRoleID != nil {
		t.Fatal("organization owner membership role must not become a writable definition field")
	}
	if aggregate.OwnerMember.ContributionRoleID == nil || *aggregate.OwnerMember.ContributionRoleID != roleID {
		t.Fatal("definition mapping must retain the owner membership role")
	}
}
