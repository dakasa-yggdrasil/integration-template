package adapter

import (
	"github.com/dakasa-yggdrasil/integration-template/internal/protocol"
	"github.com/dakasa-yggdrasil/integration-template/pkg/contractcheck"
)

// LintDescribeContract cross-validates the adapter spec returned by
// Describe(). It catches the most common manifest drift pattern that has
// repeatedly bitten DaKasa integrations (e.g. integration-aws v1.22.0 and
// integration-grafana): the SupportedExecuteOperations list, the
// ResourceTypes slice and the ActionCatalog drift out of sync, and the
// describe contract validator in yggdrasil-core rejects the integration
// at runtime with `version_mismatch` or `action_catalog_mismatch`.
//
// This function is a thin internal wrapper around the public
// pkg/contractcheck.LintDescribeContract. Adapter repos that do NOT
// share integration-template's internal/protocol types should import the
// public package directly:
//
//	import "github.com/dakasa-yggdrasil/integration-template/pkg/contractcheck"
//
// The wrapper exists so the in-tree tests and lint-action-catalog cmd
// keep using the existing protocol.AdapterDescribeResponse type they
// already build inside this template.
func LintDescribeContract(spec protocol.AdapterDescribeResponse, supportedOps []string) error {
	return contractcheck.LintDescribeContract(toContractCheckSpec(spec), supportedOps)
}

func toContractCheckSpec(spec protocol.AdapterDescribeResponse) contractcheck.DescribeResponse {
	out := contractcheck.DescribeResponse{
		ResourceTypes: make([]contractcheck.ResourceType, 0, len(spec.ResourceTypes)),
		ActionCatalog: make([]contractcheck.ActionDefinition, 0, len(spec.ActionCatalog)),
		Execution: contractcheck.ExecutionSpec{
			IdempotentActions: append([]string(nil), spec.Execution.IdempotentActions...),
		},
	}
	for _, rt := range spec.ResourceTypes {
		out.ResourceTypes = append(out.ResourceTypes, contractcheck.ResourceType{
			Name:           rt.Name,
			DefaultActions: append([]string(nil), rt.DefaultActions...),
		})
	}
	for _, action := range spec.ActionCatalog {
		out.ActionCatalog = append(out.ActionCatalog, contractcheck.ActionDefinition{
			Name:          action.Name,
			ResourceTypes: append([]string(nil), action.ResourceTypes...),
		})
	}
	return out
}
