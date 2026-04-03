package adapter

import (
	"fmt"
	"strings"

	"github.com/dakasa-yggdrasil/integration-template/internal/protocol"
)

const (
	Provider                           = "template"
	AdapterVersion                     = "1.0.0"
	OperationGenerateInstallation      = "generate_installation"
	OperationReconcileInstallation     = "reconcile_installation"
	OperationDiscoverInstallationState = "discover_installation_state"
	ModeDeclarativeApply               = "declarative_apply"

	DefaultBlueprint = "starter"

	QueueDescribe = "yggdrasil.adapter.template.describe"
	QueueExecute  = "yggdrasil.adapter.template.execute"
)

var SupportedExecuteOperations = []string{
	OperationGenerateInstallation,
	OperationReconcileInstallation,
	OperationDiscoverInstallationState,
}

type starterBlueprint struct {
	Blueprint string
	Name      string
	Namespace string
	Data      map[string]any
}

// Describe returns the normalized contract exposed by this adapter.
func Describe() protocol.AdapterDescribeResponse {
	return protocol.AdapterDescribeResponse{
		Provider: Provider,
		Adapter: protocol.IntegrationAdapterSpec{
			Transport: "rabbitmq",
			Version:   AdapterVersion,
			Queues: protocol.IntegrationAdapterQueue{
				Describe: QueueDescribe,
				Execute:  QueueExecute,
			},
			TimeoutSeconds: 20,
		},
		Capabilities: []string{"describe", "execute"},
		CredentialSchema: protocol.IntegrationSchemaSpec{
			Mode: "none",
		},
		InstanceSchema: protocol.IntegrationSchemaSpec{
			Mode: "inline",
			Properties: map[string]protocol.IntegrationSchemaProperty{
				"default_blueprint": {
					Type:        "string",
					Description: "Default generation blueprint when the product input omits one.",
					Default:     DefaultBlueprint,
				},
				"default_name": {
					Type:        "string",
					Description: "Default generated component name.",
					Default:     "example-component",
				},
				"default_namespace": {
					Type:        "string",
					Description: "Default namespace for generated resources.",
					Default:     "default",
				},
				"default_data": {
					Type:        "object",
					Description: "Default key-value payload injected into the generated ConfigMap.",
				},
			},
		},
		ResourceTypes: []protocol.IntegrationResourceType{
			{
				Name:             "component",
				CanonicalPrefix:  "thirdparty.template.component",
				IdentityTemplate: "component.{namespace}.{name}",
				Discoverable:     false,
				DefaultActions:   cloneStrings(SupportedExecuteOperations),
			},
		},
		ActionCatalog: describeActionCatalog(),
		Discovery: protocol.IntegrationDiscoverySpec{
			Mode:   "push",
			Cursor: "none",
		},
		Normalization: protocol.IntegrationNormalizationSpec{
			ExternalIDPath:         "metadata.name",
			NamePath:               "metadata.name",
			OwnerPath:              "metadata.namespace",
			FallbackResourcePrefix: "thirdparty.template.custom",
		},
		Execution: protocol.IntegrationExecutionSpec{
			SupportsDryRun:    false,
			IdempotentActions: cloneStrings(SupportedExecuteOperations),
		},
		Extensions: protocol.IntegrationExtensionsSpec{
			AllowCustomResourceTypes: true,
			AllowCustomActions:       true,
			PreserveRawPayload:       true,
		},
	}
}

func NormalizeExecuteOperation(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return OperationGenerateInstallation
	}
	return value
}

func NormalizeExecuteCapability(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return OperationGenerateInstallation
	}
	return value
}

func SupportsExecuteCapability(value string) bool {
	value = strings.TrimSpace(value)
	for _, supported := range SupportedExecuteOperations {
		if value == supported {
			return true
		}
	}
	return value == ""
}

// GenerateInstallation materializes one installation-backed component into a starter object set.
func GenerateInstallation(req protocol.AdapterGenerateInstallationRequest) (protocol.AdapterGenerateInstallationResponse, error) {
	blueprint, err := resolveBlueprint(req)
	if err != nil {
		return protocol.AdapterGenerateInstallationResponse{}, err
	}

	objects := buildStarterObjects(req, blueprint)
	return protocol.AdapterGenerateInstallationResponse{
		Operation: OperationGenerateInstallation,
		Objects:   objects,
		Metadata: map[string]any{
			"provider":          Provider,
			"blueprint":         blueprint.Blueprint,
			"name":              blueprint.Name,
			"namespace":         blueprint.Namespace,
			"generated_objects": len(objects),
		},
	}, nil
}

// GenerateProduct is kept as a compatibility wrapper while callers migrate to installation terminology.
func GenerateProduct(req protocol.AdapterGenerateProductRequest) (protocol.AdapterGenerateProductResponse, error) {
	return GenerateInstallation(req)
}

// ReconcileInstallation returns a declarative reconciliation plan for the installation.
func ReconcileInstallation(req protocol.AdapterReconcileInstallationRequest) (protocol.AdapterReconcileInstallationResponse, error) {
	blueprint, err := resolveBlueprint(protocol.AdapterGenerateInstallationRequest{
		Operation:   req.Operation,
		Context:     req.Context,
		Integration: req.Integration,
		Capability:  req.Capability,
		Input:       req.Input,
	})
	if err != nil {
		return protocol.AdapterReconcileInstallationResponse{}, err
	}

	objects := buildStarterObjects(protocol.AdapterGenerateInstallationRequest{
		Operation:   req.Operation,
		Context:     req.Context,
		Integration: req.Integration,
		Capability:  req.Capability,
		Input:       req.Input,
	}, blueprint)

	return protocol.AdapterReconcileInstallationResponse{
		Operation: OperationReconcileInstallation,
		Mode:      ModeDeclarativeApply,
		Objects:   objects,
		Metadata: map[string]any{
			"provider":           Provider,
			"blueprint":          blueprint.Blueprint,
			"name":               blueprint.Name,
			"namespace":          blueprint.Namespace,
			"reconcile_strategy": req.Reconcile.Strategy,
			"target_kind":        req.Target.Kind,
			"generated_objects":  len(objects),
		},
	}, nil
}

// DiscoverInstallationState returns the currently known installation state for the blueprint.
func DiscoverInstallationState(req protocol.AdapterDiscoverInstallationStateRequest) (protocol.AdapterDiscoverInstallationStateResponse, error) {
	blueprint, err := resolveBlueprint(protocol.AdapterGenerateInstallationRequest{
		Operation:   req.Operation,
		Context:     req.Context,
		Integration: req.Integration,
		Capability:  req.Capability,
		Input:       req.Input,
	})
	if err != nil {
		return protocol.AdapterDiscoverInstallationStateResponse{}, err
	}

	return protocol.AdapterDiscoverInstallationStateResponse{
		Operation: OperationDiscoverInstallationState,
		Status:    "planned",
		Observed:  false,
		Resources: plannedInstallationResources(blueprint),
		Metadata: map[string]any{
			"provider":  Provider,
			"blueprint": blueprint.Blueprint,
			"name":      blueprint.Name,
			"namespace": blueprint.Namespace,
			"source":    "blueprint",
			"assumptions": []string{
				"Current state is inferred from the starter blueprint and not from a live observation.",
			},
		},
	}, nil
}

func resolveBlueprint(req protocol.AdapterGenerateInstallationRequest) (starterBlueprint, error) {
	instanceConfig := req.Integration.InstanceSpec.Config
	input := req.Input

	blueprint := firstString(input, instanceConfig, []string{"blueprint", "default_blueprint"}, DefaultBlueprint)
	if blueprint != DefaultBlueprint {
		return starterBlueprint{}, fmt.Errorf("unsupported template blueprint %q", blueprint)
	}

	name := firstString(input, instanceConfig, []string{"name", "default_name"}, "example-component")
	if name == "" {
		return starterBlueprint{}, fmt.Errorf("name is required")
	}

	namespace := firstString(input, instanceConfig, []string{"namespace", "default_namespace"}, "default")
	if namespace == "" {
		return starterBlueprint{}, fmt.Errorf("namespace is required")
	}

	data := mergeObjects(
		asObject(instanceConfig["default_data"]),
		asObject(input["data"]),
	)
	if len(data) == 0 {
		data["example"] = "change-me"
	}

	return starterBlueprint{
		Blueprint: blueprint,
		Name:      name,
		Namespace: namespace,
		Data:      data,
	}, nil
}

func buildStarterObjects(req protocol.AdapterGenerateInstallationRequest, blueprint starterBlueprint) []map[string]any {
	labels := map[string]any{
		"app.kubernetes.io/managed-by": "yggdrasil",
		"yggdrasil.io/provider":        Provider,
		"yggdrasil.io/component":       req.Context.Component,
	}
	if req.Context.Product.Name != "" {
		labels["yggdrasil.io/product"] = req.Context.Product.Name
	}

	return []map[string]any{
		{
			"apiVersion": "v1",
			"kind":       "Namespace",
			"metadata": map[string]any{
				"name": blueprint.Namespace,
				"labels": map[string]any{
					"name": blueprint.Namespace,
				},
			},
		},
		{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]any{
				"name":      blueprint.Name,
				"namespace": blueprint.Namespace,
				"labels":    labels,
			},
			"data": stringifyObject(blueprint.Data),
		},
	}
}

func describeActionCatalog() []protocol.IntegrationActionDefinition {
	return []protocol.IntegrationActionDefinition{
		{
			Name:          OperationGenerateInstallation,
			Description:   "Generate the starter installation object set.",
			ResourceTypes: []string{"component"},
			Idempotent:    true,
		},
		{
			Name:          OperationReconcileInstallation,
			Description:   "Build a declarative reconciliation plan for the starter object set.",
			ResourceTypes: []string{"component"},
			Idempotent:    true,
		},
		{
			Name:          OperationDiscoverInstallationState,
			Description:   "Return the currently known installation state for the starter blueprint.",
			ResourceTypes: []string{"component"},
			Idempotent:    true,
		},
	}
}

func cloneStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	cloned := make([]string, len(values))
	copy(cloned, values)
	return cloned
}

func plannedInstallationResources(blueprint starterBlueprint) []protocol.InstallationResourceState {
	return []protocol.InstallationResourceState{
		{
			Kind:     "Namespace",
			Name:     blueprint.Namespace,
			Status:   "planned",
			Observed: false,
			Metadata: map[string]any{
				"provider": Provider,
			},
		},
		{
			Kind:      "ConfigMap",
			Namespace: blueprint.Namespace,
			Name:      blueprint.Name,
			Status:    "planned",
			Observed:  false,
			Metadata: map[string]any{
				"provider": Provider,
			},
		},
	}
}

func firstString(primary, fallback map[string]any, keys []string, defaultValue string) string {
	for _, source := range []map[string]any{primary, fallback} {
		for _, key := range keys {
			value, ok := source[key]
			if !ok {
				continue
			}
			parsed := strings.TrimSpace(asString(value))
			if parsed != "" {
				return parsed
			}
		}
	}
	return defaultValue
}

func asString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return fmt.Sprint(value)
	}
}

func asObject(value any) map[string]any {
	typed, ok := value.(map[string]any)
	if !ok {
		return map[string]any{}
	}
	return typed
}

func mergeObjects(values ...map[string]any) map[string]any {
	merged := map[string]any{}
	for _, value := range values {
		for key, item := range value {
			merged[key] = item
		}
	}
	return merged
}

func stringifyObject(input map[string]any) map[string]string {
	output := make(map[string]string, len(input))
	for key, value := range input {
		output[key] = asString(value)
	}
	return output
}
