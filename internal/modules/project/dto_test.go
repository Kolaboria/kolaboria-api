package project

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestProjectDefinitionRequestRequiresFullStrictContract(t *testing.T) {
	var definition ProjectDefinitionInput
	valid := `{"creation_mode":"personal","initiator_organization_id":null,"title":"","summary":"","description":"","slug":"","project_category":"product","visibility":"public","owner_contribution_role_id":null,"origin":"personal","why_collaborative":"","contributor_outcome":"","owner_commitment":"","lead_expectations":"","client_acknowledgement":false,"start_date":null,"deadline":null,"availability":"flexible","hours_per_week":5,"collaboration_agreement":false,"roles":[]}`
	if err := json.Unmarshal([]byte(valid), &definition); err != nil {
		t.Fatalf("valid full draft contract rejected: %v", err)
	}
	if definition.Roles == nil || definition.OwnerContributionRoleID != nil {
		t.Fatalf("empty arrays and null owner role should remain explicit: %#v", definition)
	}

	missing := strings.Replace(valid, `,"owner_contribution_role_id":null`, "", 1)
	if err := json.Unmarshal([]byte(missing), &definition); err == nil {
		t.Fatal("missing nullable owner_contribution_role_id was accepted")
	}
	withLegacyField := strings.Replace(valid, `,"roles":[]`, `,"tool_ids":[],"roles":[]`, 1)
	if err := json.Unmarshal([]byte(withLegacyField), &definition); err == nil {
		t.Fatal("project-level tools field was accepted")
	}
	withCustomRole := strings.Replace(valid, `"roles":[]`, `"roles":[{"id":null,"contribution_role_id":null,"description":"","capacity":1,"skill_ids":[],"tool_ids":[],"custom_title":"Lead"}]`, 1)
	if err := json.Unmarshal([]byte(withCustomRole), &definition); err == nil {
		t.Fatal("legacy custom contributor role title was accepted")
	}
}

func TestProjectDateUsesCalendarDateWireFormat(t *testing.T) {
	var date ProjectDate
	if err := json.Unmarshal([]byte(`"2026-10-08"`), &date); err != nil {
		t.Fatal("valid calendar date rejected:", err)
	}
	if err := json.Unmarshal([]byte(`"2026-02-31"`), &date); err == nil {
		t.Fatal("invalid calendar date accepted")
	}
}
