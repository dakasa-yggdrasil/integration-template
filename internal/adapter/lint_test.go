package adapter

import (
	"strings"
	"testing"

	"github.com/dakasa-yggdrasil/integration-template/internal/protocol"
)

// TestLintDescribeContractPassesOnTemplate is the actual prevention check
// that any adapter inheriting from this template should keep green. It
// runs the lint on the live Describe() output — if it ever fails, the
// adapter has the same drift pattern that caused the integration-aws
// v1.22.0 and integration-grafana incidents.
func TestLintDescribeContractPassesOnTemplate(t *testing.T) {
	if err := LintDescribeContract(Describe(), SupportedExecuteOperations); err != nil {
		t.Fatalf("LintDescribeContract drift on template adapter:\n%v", err)
	}
}

func TestLintDescribeContractFlagsMissingActionForOperation(t *testing.T) {
	spec := Describe()
	// Inject an extra supported op that does not appear in the catalog.
	withExtra := append([]string{}, SupportedExecuteOperations...)
	withExtra = append(withExtra, "ghost_operation")

	err := LintDescribeContract(spec, withExtra)
	if err == nil {
		t.Fatal("expected drift error, got nil")
	}
	if !strings.Contains(err.Error(), `operation "ghost_operation" in SupportedExecuteOperations is missing from ActionCatalog`) {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestLintDescribeContractFlagsUnknownResourceTypeReference(t *testing.T) {
	spec := Describe()
	spec.ActionCatalog = append([]protocol.IntegrationActionDefinition{}, spec.ActionCatalog...)
	spec.ActionCatalog = append(spec.ActionCatalog, protocol.IntegrationActionDefinition{
		Name:          "phantom_action",
		ResourceTypes: []string{"phantom_resource"},
	})
	supported := append([]string{}, SupportedExecuteOperations...)
	supported = append(supported, "phantom_action")

	err := LintDescribeContract(spec, supported)
	if err == nil {
		t.Fatal("expected drift error, got nil")
	}
	if !strings.Contains(err.Error(), `action "phantom_action" references unknown resource_type "phantom_resource"`) {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestLintDescribeContractFlagsActionWithoutResourceTypes(t *testing.T) {
	spec := Describe()
	spec.ActionCatalog = append([]protocol.IntegrationActionDefinition{}, spec.ActionCatalog...)
	spec.ActionCatalog = append(spec.ActionCatalog, protocol.IntegrationActionDefinition{
		Name: "orphan_action",
	})
	supported := append([]string{}, SupportedExecuteOperations...)
	supported = append(supported, "orphan_action")

	err := LintDescribeContract(spec, supported)
	if err == nil {
		t.Fatal("expected drift error, got nil")
	}
	if !strings.Contains(err.Error(), `action "orphan_action" has no resource_types`) {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestLintDescribeContractFlagsActionMissingFromSupportedOperations(t *testing.T) {
	spec := Describe()
	spec.ActionCatalog = append([]protocol.IntegrationActionDefinition{}, spec.ActionCatalog...)
	spec.ActionCatalog = append(spec.ActionCatalog, protocol.IntegrationActionDefinition{
		Name:          "rogue_action",
		ResourceTypes: []string{"component"},
	})

	err := LintDescribeContract(spec, SupportedExecuteOperations)
	if err == nil {
		t.Fatal("expected drift error, got nil")
	}
	if !strings.Contains(err.Error(), `action "rogue_action" in ActionCatalog is missing from SupportedExecuteOperations`) {
		t.Fatalf("unexpected error message: %v", err)
	}
}
