package project

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestProjectReadDTOsExposePublicOwnerAndInitiatorProjections(t *testing.T) {
	ownerID := uuid.New()
	organizationID := uuid.New()
	ownerName := "Project Owner"
	initiatorName := "Organization Admin"
	aggregate := &ProjectAggregate{Project: Project{
		ID: uuid.New(), Slug: "organization-project", Title: "Project", Summary: "Summary",
		Status: ProjectStatusAwaitingOwner, CreationMode: CreationModeOrganizationInitiated,
		OwnerID: &ownerID, InitiatorOrganizationID: &organizationID,
	}, OwnerProfile: &ProjectPublicProfileResponse{
		ID: ownerID, Username: "project-owner", FullName: &ownerName,
	}, InitiatorProfile: &ProjectPublicProfileResponse{
		ID: uuid.New(), Username: "organization-admin", FullName: &initiatorName,
	}, InitiatorOrganization: &ProjectOrganizationResponse{
		ID: organizationID, Name: "Example Organization", Slug: "example-organization",
	}}
	detailBytes, err := json.Marshal(mapProjectDetail(aggregate, uuid.Nil, false, false))
	if err != nil {
		t.Fatal(err)
	}
	summaryBytes, err := json.Marshal(mapProjectSummary(aggregate))
	if err != nil {
		t.Fatal(err)
	}
	for _, payload := range [][]byte{detailBytes, summaryBytes} {
		for _, key := range []string{`"owner_profile"`, `"initiator_organization"`} {
			if !strings.Contains(string(payload), key) {
				t.Errorf("project read response omitted public identity projection %s: %s", key, payload)
			}
		}
	}
	if !strings.Contains(string(detailBytes), `"initiator_profile":{"id":`) ||
		!strings.Contains(string(detailBytes), `"username":"organization-admin"`) ||
		!strings.Contains(string(detailBytes), `"full_name":"Project Owner"`) ||
		strings.Contains(string(detailBytes), `"email"`) {
		t.Errorf("detail response did not expose only the expected public-safe identity fields: %s", detailBytes)
	}
	if !strings.Contains(string(summaryBytes), `"full_name":"Project Owner"`) ||
		!strings.Contains(string(summaryBytes), `"name":"Example Organization"`) {
		t.Errorf("summary response omitted public owner/organization labels: %s", summaryBytes)
	}
}
