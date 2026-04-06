# integration-template

`integration-template` is the base repository for new Yggdrasil plugins. It already comes with the RabbitMQ RPC worker shape, the `describe/execute` contract expected by `yggdrasil-core`, a minimal installation blueprint, tests, and example manifests.

The repository is intentionally generic. It is not meant to be used as-is in production. The first step after copying it is to rename the module, provider, queue names, blueprint semantics, and examples to match the new integration.

The template keeps its own local protocol types on purpose. New plugins should
follow the schemas documented in
[/Users/dakasa/projects/yggdrasil-core/docs/contracts](/Users/dakasa/projects/yggdrasil-core/docs/contracts),
not import `yggdrasil-core/model` directly.

This repository also ships the reference GitHub Actions pair for dogfooding:

- `.github/workflows/emit-deploy-event.yml`
- `.github/workflows/deploy.yml`

The emit workflow now uses the official GitHub Action
[`dakasa-yggdrasil/action-emit-workflow-run`](https://github.com/dakasa-yggdrasil/action-emit-workflow-run)
instead of embedding the HTTP call inline in repository YAML.

On each commit to `main`, `emit-deploy-event.yml` can POST one workflow run
request into `yggdrasil-core`. The bootstrap workflow
`global/ecosystem-repository-commit` then dispatches this repository's
`deploy.yml` through the global GitHub integration.

The important architectural rule now is: `describe` is not decorative. The core validates the
live `describe` payload against the public contract, compares it with the stored
`integration_type`, and can fast-fail operations when the adapter is in a recent unhealthy state.
That means provider name, adapter version, queue names, capabilities, resource types, and action
catalog must stay honest.

Heimdall support also has an explicit lightweight path now. New integrations
should fill `spec.guardian_support` in their `integration_type` and expose those
signals in runtime details when they can. If a plugin does not expose
`guardian_support`, Heimdall can still see generic health, but it does not get
provider-specific lightweight remediation support for that integration.

This template is intentionally installation-oriented by default. It is the right starting point for
plugins such as `rabbitmq-on-kubernetes` or `grafana-on-kubernetes`. If you are building a
runtime/operation plugin such as `rabbitmq`, `grafana`, or another SaaS/API operator, keep the
same contract discipline but replace the starter installation operations with the domain operations
you actually expose.

## Official plugin convention

Yggdrasil now uses both a naming convention and a catalog convention.

Plugin naming:

- `domain-on-substrate`: installation/substrate plugin
- `domain`: runtime/operation plugin

Plugin catalog grouping:

- `catalog-domain`: shared domain shown in the Yggdrasil catalog
- `catalog-section`: section such as `installations` or `operations`
- `catalog-entry`: the concrete variant inside that section

Examples:

- `rabbitmq-on-kubernetes`: installs RabbitMQ on Kubernetes
- catalog position: `rabbitmq / installations / kubernetes`
- `rabbitmq`: operates RabbitMQ resources after installation
- catalog position: `rabbitmq / operations / api`
- `grafana-on-kubernetes`: installs Grafana on Kubernetes
- catalog position: `grafana / installations / kubernetes`
- `grafana`: operates dashboards, datasources, folders, alerts
- catalog position: `grafana / operations / api`

That convention is not just naming style. It is how we stop one plugin from turning into a hidden
monolith. If the plugin's main job is “put this system on this substrate”, use the explicit
`-on-...` form. If the plugin's main job is “operate this domain”, keep the pure domain name.

## What this template already gives you

- `describe` queue for adapter introspection
- `execute` queue with:
  - `generate_installation`
  - `reconcile_installation`
  - `discover_installation_state`
- a minimal starter blueprint that generates:
  - `Namespace`
  - `ConfigMap`
- unit tests for the adapter contract
- example `integration_type` and `integration_instance`

## When to keep or remove the starter operations

Keep the starter installation operations when the plugin is about installation/substrate:

- `generate_installation`
- `reconcile_installation`
- `discover_installation_state`

Replace them when the plugin is really a runtime/operation plugin. In that case:

- rename the operations to the real domain verbs
- update `SupportedExecuteOperations`
- update `describe` resource types and action catalog
- update `controllers/message/execute.go`
- update the example manifests

## Repository shape

- [/Users/dakasa/projects/integration-template/main.go](/Users/dakasa/projects/integration-template/main.go): worker bootstrap
- [/Users/dakasa/projects/integration-template/controllers/message](/Users/dakasa/projects/integration-template/controllers/message): RabbitMQ RPC handlers
- [/Users/dakasa/projects/integration-template/internal/adapter/spec.go](/Users/dakasa/projects/integration-template/internal/adapter/spec.go): adapter contract and starter blueprint
- [/Users/dakasa/projects/integration-template/internal/adapter/spec_test.go](/Users/dakasa/projects/integration-template/internal/adapter/spec_test.go): contract tests
- [/Users/dakasa/projects/integration-template/examples](/Users/dakasa/projects/integration-template/examples): example manifests for the core

## Rename checklist

When creating a real plugin from this template, update at least:

1. `go.mod` module path
2. `Provider` in the adapter
3. queue names
4. resource types and action descriptions
5. supported operations and their handler routing
6. starter blueprint fields and generated objects
7. `README.md`
8. files under `examples/`
9. catalog labels in the example `integration_type`
10. default component naming in `.github/workflows/*.yml` or the corresponding repository variables

## Queues in the template

- `yggdrasil.adapter.template.describe`
- `yggdrasil.adapter.template.execute`

## Environment

- `BROKER_URL`: RabbitMQ connection string used by the worker itself

## Running

```bash
go run .
```

## Starter blueprint

The default `starter` blueprint accepts:

- `blueprint`
- `name`
- `namespace`
- `data`

It is deliberately small so new plugin authors can replace it quickly.

## Contract discipline

When adapting this template into a real plugin, treat these as one unit:

- `internal/adapter/spec.go`
- `controllers/message/describe.go`
- `controllers/message/execute.go`
- `examples/integration_type.example.json`

If you rename a queue, add or remove an operation, or change the adapter version, update all of
them together. The core now treats contract drift as a real operational health issue, not a soft
warning.

When you turn the template into a real plugin, also set the catalog labels in the example manifest:

- `yggdrasil.io/catalog-domain`
- `yggdrasil.io/catalog-section`
- `yggdrasil.io/catalog-entry`

## Heimdall lightweight support

If you want the plugin to be plug-and-play with Heimdall light mode:

- add `spec.guardian_support.mode` with `light` or `full`
- map canonical signals in `spec.guardian_support.signals`
- emit those keys in runtime detail payloads whenever the adapter can observe them

Canonical signals Heimdall understands today:

- `oom_killed`
- `restart_count`
- `error_rate`
- `queue_backlog`
- `memory_pressure`
- `disk_pressure`
- `rate_limited`
- `auth_denied`
- `sync_lag_seconds`
- `monthly_cost_usd`
- `utilization`
- `idle_hours`
- `overprovisioned`
- `scheduling_failure`
- `insufficient_cpu`

If a future integration omits this section, Heimdall can still fall back to the
generic health path or the LLM path, but it will not have first-class
lightweight remediation hints for that provider.

## Validation

```bash
go mod tidy
go test ./...
```

## GitHub dogfooding configuration

Repository configuration:

- `YGGDRASIL_CORE_BASE_URL` secret: required for commit event emission
- `YGGDRASIL_WORKFLOW_RUN_TOKEN` secret: optional shared token for `/api/v1/workflow-runs`
- `YGGDRASIL_WORKFLOW_NAMESPACE` variable: optional, defaults to `global`
- `YGGDRASIL_WORKFLOW_NAME` variable: optional, defaults to `ecosystem-repository-commit`
- `YGGDRASIL_DEPLOY_WORKFLOW` variable: optional, defaults to `deploy.yml`
- `YGGDRASIL_COMPONENT_KIND` variable: optional, defaults to `integration`
- `YGGDRASIL_COMPONENT_NAME` variable: optional, defaults to the repository name
- `YGGDRASIL_DEPLOY_ENVIRONMENT` variable: optional, defaults to `production`

## Example usage

The starter blueprint currently generates a namespace and a configmap. This is only a placeholder to keep the contract executable while you replace it with the real provider logic.
