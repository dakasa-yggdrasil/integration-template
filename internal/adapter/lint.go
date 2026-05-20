package adapter

import (
	"fmt"
	"sort"
	"strings"

	"github.com/dakasa-yggdrasil/integration-template/internal/protocol"
)

// LintDescribeContract cross-validates the adapter spec returned by
// Describe(). It catches the most common manifest drift pattern that has
// repeatedly bitten DaKasa integrations (e.g. integration-aws v1.22.0 and
// integration-grafana): the SupportedExecuteOperations list, the
// ResourceTypes slice and the ActionCatalog drift out of sync, and the
// describe contract validator in yggdrasil-core rejects the integration
// at runtime with `version_mismatch` or `action_catalog_mismatch`.
//
// The lint is deterministic and runs against the live result of
// Describe(); it does not parse Go source. Any adapter — regardless of
// its package layout (internal/adapter, providers/runtime/adapter, etc.)
// — can call this from a unit test or from a cmd/lint binary.
//
// Validations performed:
//   1. Every operation in supportedOps appears in spec.ActionCatalog.
//   2. Every spec.ActionCatalog entry name appears in supportedOps.
//   3. Every spec.ActionCatalog entry references at least one ResourceType.
//   4. Every spec.ActionCatalog ResourceType refers to a name that exists
//      in spec.ResourceTypes.
//   5. Every spec.ResourceTypes default_actions entry exists in supportedOps.
//   6. Every spec.Execution.IdempotentActions entry exists in supportedOps.
//
// Returns a non-nil error whose message is a human-readable, sorted diff.
func LintDescribeContract(spec protocol.AdapterDescribeResponse, supportedOps []string) error {
	var issues []string

	supportedSet := newStringSet(supportedOps)
	catalogSet := make(map[string]protocol.IntegrationActionDefinition, len(spec.ActionCatalog))
	for _, action := range spec.ActionCatalog {
		catalogSet[action.Name] = action
	}
	resourceSet := make(map[string]struct{}, len(spec.ResourceTypes))
	for _, rt := range spec.ResourceTypes {
		resourceSet[rt.Name] = struct{}{}
	}

	// 1 — Every SupportedExecuteOperations entry must appear in action_catalog.
	for _, op := range supportedOps {
		if _, ok := catalogSet[op]; !ok {
			issues = append(issues, fmt.Sprintf("operation %q in SupportedExecuteOperations is missing from ActionCatalog", op))
		}
	}

	// 2 — Every action_catalog entry must appear in SupportedExecuteOperations.
	for name := range catalogSet {
		if _, ok := supportedSet[name]; !ok {
			issues = append(issues, fmt.Sprintf("action %q in ActionCatalog is missing from SupportedExecuteOperations", name))
		}
	}

	// 3 + 4 — Every action_catalog entry must declare resource_types, and
	// each must exist in spec.ResourceTypes.
	for _, action := range spec.ActionCatalog {
		if len(action.ResourceTypes) == 0 {
			issues = append(issues, fmt.Sprintf("action %q has no resource_types", action.Name))
			continue
		}
		for _, ref := range action.ResourceTypes {
			if _, ok := resourceSet[ref]; !ok {
				issues = append(issues, fmt.Sprintf("action %q references unknown resource_type %q", action.Name, ref))
			}
		}
	}

	// 5 — Every default_actions entry on a resource_type must be in supportedOps.
	for _, rt := range spec.ResourceTypes {
		for _, op := range rt.DefaultActions {
			if _, ok := supportedSet[op]; !ok {
				issues = append(issues, fmt.Sprintf("resource_type %q default_actions references unknown operation %q", rt.Name, op))
			}
		}
	}

	// 6 — Every idempotent_actions entry must be in supportedOps.
	for _, op := range spec.Execution.IdempotentActions {
		if _, ok := supportedSet[op]; !ok {
			issues = append(issues, fmt.Sprintf("idempotent_actions references unknown operation %q", op))
		}
	}

	if len(issues) == 0 {
		return nil
	}
	sort.Strings(issues)
	return fmt.Errorf("describe contract drift detected:\n  - %s", strings.Join(issues, "\n  - "))
}

func newStringSet(values []string) map[string]struct{} {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		set[value] = struct{}{}
	}
	return set
}
