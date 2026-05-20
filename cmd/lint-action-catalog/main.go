// Command lint-action-catalog runs the describe-contract lint against
// the in-tree adapter package. Exits 0 when the live Describe() output
// is consistent with SupportedExecuteOperations, 1 otherwise.
//
// Mirrors the unit test TestLintDescribeContractPassesOnTemplate so the
// same prevention check is reachable from a CI pipeline step that does
// not run the full `go test ./...` (faster smoke).
//
// Designed to be copied into any integration adapter repo. The
// validation engine lives in the public, importable package
//
//	github.com/dakasa-yggdrasil/integration-template/pkg/contractcheck
//
// so downstream adapters can `go get` it and add the same prevention
// without copying code. This binary uses the in-tree adapter package as
// a convenience for the template itself.
package main

import (
	"fmt"
	"os"

	"github.com/dakasa-yggdrasil/integration-template/internal/adapter"
)

func main() {
	spec := adapter.Describe()
	if err := adapter.LintDescribeContract(spec, adapter.SupportedExecuteOperations); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	fmt.Printf("lint-action-catalog: ok (provider=%s actions=%d resource_types=%d)\n",
		spec.Provider, len(spec.ActionCatalog), len(spec.ResourceTypes))
}
