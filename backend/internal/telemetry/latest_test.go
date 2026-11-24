package telemetry

import "testing"

func TestValidateLatestQueryRequiresTenantAndValidServer(t *testing.T) {
	base := LatestQuery{OrganizationID: "org", ServerID: "2b6a8d10-9b4d-4e40-9a9c-0c8fc3af3b7f"}
	if err := ValidateLatestQuery(base); err != nil {
		t.Fatalf("valid latest query should pass: %v", err)
	}
	invalid := base
	invalid.ServerID = "not-a-uuid"
	if err := ValidateLatestQuery(invalid); err == nil {
		t.Fatal("invalid server ID must be rejected")
	}
}
