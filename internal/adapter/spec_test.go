package adapter

import (
	"testing"

	"github.com/dakasa-yggdrasil/integration-template/internal/protocol"
)

func TestDescribe(t *testing.T) {
	response := Describe()

	if response.Provider != Provider {
		t.Fatalf("provider = %q, want %q", response.Provider, Provider)
	}
	if response.Adapter.Queues.Describe != QueueDescribe {
		t.Fatalf("describe queue = %q, want %q", response.Adapter.Queues.Describe, QueueDescribe)
	}
	if response.Adapter.Queues.Execute != QueueExecute {
		t.Fatalf("execute queue = %q, want %q", response.Adapter.Queues.Execute, QueueExecute)
	}
	if len(response.ResourceTypes) != 1 || response.ResourceTypes[0].Name != "component" {
		t.Fatalf("unexpected resource types: %#v", response.ResourceTypes)
	}
	if len(response.ActionCatalog) != 3 {
		t.Fatalf("action catalog = %#v, want 3 actions", response.ActionCatalog)
	}
	if len(response.Execution.IdempotentActions) != len(SupportedExecuteOperations) {
		t.Fatalf("idempotent actions = %#v, want %d operations", response.Execution.IdempotentActions, len(SupportedExecuteOperations))
	}
}

func TestSupportedExecuteOperationsStayAligned(t *testing.T) {
	if len(SupportedExecuteOperations) != 3 {
		t.Fatalf("SupportedExecuteOperations = %#v, want 3 operations", SupportedExecuteOperations)
	}
	for _, operation := range SupportedExecuteOperations {
		if !SupportsExecuteCapability(operation) {
			t.Fatalf("SupportsExecuteCapability(%q) = false", operation)
		}
	}
}

func TestGenerateInstallationUsesDefaults(t *testing.T) {
	response, err := GenerateInstallation(protocol.AdapterGenerateInstallationRequest{
		Operation:  OperationGenerateInstallation,
		Capability: OperationGenerateInstallation,
		Context: protocol.AdapterGenerateInstallationContext{
			Component: "starter",
			Product: protocol.ManifestReference{
				Name: "template-product",
			},
		},
		Integration: protocol.AdapterGenerateInstallationIntegrationContext{
			InstanceSpec: protocol.IntegrationInstanceManifestSpec{
				Config: map[string]any{
					"default_namespace": "platform-system",
					"default_name":      "starter-config",
					"default_data": map[string]any{
						"region": "sa-east1",
					},
				},
			},
		},
		Input: map[string]any{
			"blueprint": DefaultBlueprint,
		},
	})
	if err != nil {
		t.Fatalf("GenerateInstallation() error = %v", err)
	}

	if len(response.Objects) != 2 {
		t.Fatalf("generated objects = %d, want 2", len(response.Objects))
	}

	configMap := response.Objects[1]
	metadata := configMap["metadata"].(map[string]any)
	data := configMap["data"].(map[string]string)

	if metadata["name"] != "starter-config" {
		t.Fatalf("configmap name = %v, want starter-config", metadata["name"])
	}
	if metadata["namespace"] != "platform-system" {
		t.Fatalf("configmap namespace = %v, want platform-system", metadata["namespace"])
	}
	if data["region"] != "sa-east1" {
		t.Fatalf("configmap data = %#v, want region=sa-east1", data)
	}
}

func TestGenerateInstallationSupportsOverrides(t *testing.T) {
	response, err := GenerateInstallation(protocol.AdapterGenerateInstallationRequest{
		Operation:  OperationGenerateInstallation,
		Capability: OperationGenerateInstallation,
		Integration: protocol.AdapterGenerateInstallationIntegrationContext{
			InstanceSpec: protocol.IntegrationInstanceManifestSpec{
				Config: map[string]any{
					"default_namespace": "default",
					"default_data": map[string]any{
						"owner": "platform",
					},
				},
			},
		},
		Input: map[string]any{
			"blueprint": DefaultBlueprint,
			"name":      "custom-config",
			"namespace": "custom-system",
			"data": map[string]any{
				"owner":   "ops",
				"feature": "enabled",
			},
		},
	})
	if err != nil {
		t.Fatalf("GenerateInstallation() error = %v", err)
	}

	configMap := response.Objects[1]
	metadata := configMap["metadata"].(map[string]any)
	data := configMap["data"].(map[string]string)

	if metadata["name"] != "custom-config" {
		t.Fatalf("configmap name = %v, want custom-config", metadata["name"])
	}
	if metadata["namespace"] != "custom-system" {
		t.Fatalf("configmap namespace = %v, want custom-system", metadata["namespace"])
	}
	if data["owner"] != "ops" || data["feature"] != "enabled" {
		t.Fatalf("configmap data = %#v", data)
	}
}

func TestGenerateInstallationRejectsUnknownBlueprint(t *testing.T) {
	_, err := GenerateInstallation(protocol.AdapterGenerateInstallationRequest{
		Operation:  OperationGenerateInstallation,
		Capability: OperationGenerateInstallation,
		Input: map[string]any{
			"blueprint": "custom",
		},
	})
	if err == nil {
		t.Fatal("GenerateInstallation() error = nil, want error")
	}
}

func TestReconcileInstallationReturnsDeclarativePlan(t *testing.T) {
	response, err := ReconcileInstallation(protocol.AdapterReconcileInstallationRequest{
		Operation:  OperationReconcileInstallation,
		Capability: OperationGenerateInstallation,
		Context: protocol.AdapterGenerateInstallationContext{
			Component: "starter",
		},
		Integration: protocol.AdapterGenerateInstallationIntegrationContext{
			InstanceSpec: protocol.IntegrationInstanceManifestSpec{
				Config: map[string]any{
					"default_name":      "starter-config",
					"default_namespace": "default",
				},
			},
		},
		Input: map[string]any{
			"blueprint": DefaultBlueprint,
		},
		Target: protocol.ProductTargetSpec{
			Kind:      "kubernetes",
			Namespace: "default",
		},
		Reconcile: protocol.ProductReconcileSpec{
			Strategy: "apply",
		},
	})
	if err != nil {
		t.Fatalf("ReconcileInstallation() error = %v", err)
	}
	if response.Operation != OperationReconcileInstallation {
		t.Fatalf("operation = %q, want %q", response.Operation, OperationReconcileInstallation)
	}
	if response.Mode != ModeDeclarativeApply {
		t.Fatalf("mode = %q, want %q", response.Mode, ModeDeclarativeApply)
	}
	if len(response.Objects) != 2 {
		t.Fatalf("generated objects = %d, want 2", len(response.Objects))
	}
}

func TestDiscoverInstallationStateReturnsPlannedState(t *testing.T) {
	response, err := DiscoverInstallationState(protocol.AdapterDiscoverInstallationStateRequest{
		Operation:  OperationDiscoverInstallationState,
		Capability: OperationGenerateInstallation,
		Integration: protocol.AdapterGenerateInstallationIntegrationContext{
			InstanceSpec: protocol.IntegrationInstanceManifestSpec{
				Config: map[string]any{
					"default_name":      "starter-config",
					"default_namespace": "default",
				},
			},
		},
		Input: map[string]any{
			"blueprint": DefaultBlueprint,
		},
	})
	if err != nil {
		t.Fatalf("DiscoverInstallationState() error = %v", err)
	}
	if response.Operation != OperationDiscoverInstallationState {
		t.Fatalf("operation = %q, want %q", response.Operation, OperationDiscoverInstallationState)
	}
	if response.Status != "planned" {
		t.Fatalf("status = %q, want planned", response.Status)
	}
	if response.Observed {
		t.Fatal("observed = true, want false")
	}
}
