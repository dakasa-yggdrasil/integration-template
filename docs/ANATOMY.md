# Anatomy of the scaffold

> What every file and directory in `integration-template` is for, and where you
> change things when you build a real adapter.

This is the **archetype** every Yggdrasil integration adapter is cloned from.
The shape here is the shape your repo will have after
`yggdrasil new integration <name> --owner <org>`. Learn it once; it is identical
across `integration-stripe`, `integration-github`, `integration-aws`, and the
rest of the [catalog](https://github.com/dakasa-yggdrasil/yggdrasil-core).

New here? Read the [Getting Started](GETTING-STARTED.md) walkthrough first, and
keep the [Contract digest](CONTRACT.md) open while you implement.

---

## Top-level layout

```
main.go                      # process entrypoint: AMQP dial, consumers, health server, shutdown
controllers/message/         # AMQP RPC plumbing (the transport layer)
internal/adapter/            # YOUR adapter logic: Describe() + Execute() + contract lint
internal/protocol/           # local wire types (RPC envelopes, manifest spec) — kept in-repo on purpose
pkg/contractcheck/           # PUBLIC, importable describe-contract linter
cmd/lint-action-catalog/     # CI binary that runs the linter against this adapter
examples/                    # sample integration_type + integration_instance manifests
yggdrasil-quickstart.yaml    # install bundle consumed by `yggdrasil install`
Taskfile.yml                 # build/test/compose tasks
Dockerfile                   # multi-stage Go build → adapter image
docker-compose*.yml          # local stack (RabbitMQ + adapter)
.github/workflows/           # CI, release (ghcr image), publish-oci (quickstart artifact), ops
INTEGRATION_CONTRACT.md      # THE LAW — what an integration is and is not
AGENTS.md / CLAUDE.md / …    # per-tool pointers, all routing back to the contract
```

---

## `main.go` — the process

The entrypoint, and the one file you rarely touch. It:

1. Reads `BROKER_URL` from the environment. **No default — fatal if unset.**
2. Dials RabbitMQ (`amqp.Dial`).
3. Registers the AMQP consumers via `message.RegisterAllConsumers`.
4. Starts an HTTP health server (`/healthz`, `/readyz`) on `HEALTHCHECK_PORT`
   (default `8080`).
5. Blocks until `SIGINT`/`SIGTERM` **or** the broker connection closes
   (`conn.NotifyClose`), then shuts the health server down within a 10s grace
   window.

Letting the process exit when the broker drops is deliberate — the kubelet then
restarts the pod, which is the cleanest recovery path. Do not paper over a lost
connection with a silent reconnect loop.

**HTTP knobs** read here (all optional, sane defaults): `HEALTHCHECK_PORT`,
`HTTP_READ_HEADER_TIMEOUT_SECONDS`, `HTTP_READ_TIMEOUT_SECONDS`,
`HTTP_WRITE_TIMEOUT_SECONDS`, `HTTP_IDLE_TIMEOUT_SECONDS`. See
[OPERATIONS](#operations-where-things-live-at-runtime) below and
[Getting Started](GETTING-STARTED.md) for the full table.

---

## `controllers/message/` — the transport layer

The AMQP RPC machinery. The template ships its **own minimal RPC layer** here
rather than importing `yggdrasil-sdk-go`, to keep an adopter's import surface
small. You extend, but rarely rewrite, these files.

| File | Responsibility |
|---|---|
| `register.go` | Declares the queues and wires each one to a handler. `Queues()` lists the two queue names the worker serves. |
| `consume.go` | The consume loop: QoS, `ctx` timeout per delivery, `Ack` on success / `Nack` on error. `ConsumerConfig` is one queue + handler + timeout + QoS. |
| `describe.go` | Handler for the **describe** queue. Validates the optional `provider` / `expected_version` and replies with `adapter.Describe()`. |
| `execute.go` | Handler for the **execute** queue. Normalizes the operation, checks it is supported, decodes the typed request, dispatches into `internal/adapter`, replies. |
| `rpc.go` | The reply envelope (`{ok, data, error}`) and the `replySuccess` / `replyFailure` helpers. Replies go to the delivery's `reply_to` queue with the original `correlation_id`. |

### The two queues (this is the real transport surface)

```
yggdrasil.adapter.template.describe   ← core asks "what can you do?"
yggdrasil.adapter.template.execute    ← core asks "do this capability"
```

> **Health is NOT an AMQP queue.** Liveness/readiness are HTTP endpoints served
> by `main.go` (`/healthz`, `/readyz`). The worker speaks two AMQP RPC verbs —
> `describe` and `execute` — plus HTTP health. (The `protocol` package leaves
> room for additional queues like `discover`/`sync`/`health`, but the scaffold
> wires only describe + execute.)

---

## `internal/adapter/` — your adapter logic

**This is where you spend your time.** Everything that makes your integration
*yours* lives here.

| File | Responsibility |
|---|---|
| `spec.go` | The two halves of the contract: `Describe()` (the catalog the core registers) and the `Execute()` handlers it must stay aligned with. Also: provider name, adapter version, queue names, the `SupportedExecuteOperations` list, resource types, and the action catalog. |
| `lint.go` | A thin wrapper over `pkg/contractcheck` that lints **this** adapter's `Describe()` output for drift. |
| `spec_test.go`, `lint_test.go` | Unit tests proving `Describe()`/`Execute()` stay consistent. |

`Describe()` and `Execute()` are two views of the same truth and **must agree**.
If `Execute()` accepts an operation that `Describe()` does not advertise (or vice
versa), `pkg/contractcheck` fails and the core rejects the adapter at
registration with `version_mismatch` / `action_catalog_mismatch`. See
[CONTRACT → describe and execute alignment](CONTRACT.md#describe-and-execute-must-agree).

> **The shipped `Describe()` is a placeholder.** The stub exposes
> `generate_installation` / `reconcile_installation` / `discover_installation_state`
> on a `component` resource type — a compiling example so the round trip works
> out of the box. When you implement a *real* resource, you rename these to the
> four canonical prefixes (`ensure_` / `observe_` / `destroy_` / `discover_`) per
> [INTEGRATION_CONTRACT.md §5](../INTEGRATION_CONTRACT.md#5-the-resource-contract-four-canonical-prefixes).
> See [CONTRACT](CONTRACT.md) for the naming decision flow.

---

## `internal/protocol/` — the wire types

The Go structs for the RPC envelopes (`AdapterDescribeRequest/Response`,
`Adapter…Request/Response`) and the manifest spec (`IntegrationTypeManifestSpec`,
`IntegrationResourceType`, schema/action/discovery specs).

These are kept **local to the repo on purpose**. An adapter MUST NOT import
runtime or domain code from `yggdrasil-core` or the `yggdrasil` monorepo — that
coupling is forbidden by [AGENTS.md](../AGENTS.md). The shapes mirror the core's
public contract; if the contract evolves, you update these types here.

---

## `pkg/contractcheck/` — the public linter

A standalone, importable package (`github.com/dakasa-yggdrasil/integration-template/pkg/contractcheck`)
that cross-validates a `Describe()` response:

- every supported operation appears in the action catalog (and vice versa);
- every action references a declared resource type;
- every `default_actions` / `idempotent_actions` entry is a real operation.

It defines its **own minimal types**, so any adapter repo can `go get` it and run
the same check without depending on this template's `internal/protocol`. It was
extracted to a public package precisely so `integration-grafana` and
`integration-secrets-management` could reuse it. This is the canonical guard
against the manifest-drift class of bug.

---

## `cmd/lint-action-catalog/` — the CI guard

A tiny binary that runs `pkg/contractcheck` against this adapter's live
`Describe()` output and exits non-zero on drift. CI runs it **before** the full
test suite (`go run ./cmd/lint-action-catalog`) so the signal is immediate. Copy
it into your real adapter — it works against any in-tree `adapter` package.

---

## `examples/`

Reference manifests showing the shape the core expects:

- `integration_type.example.json` — the registered type (provider, adapter
  transport/queues/version, schemas, resource types, action catalog,
  `guardian_support` signals).
- `integration_instance.example.json` — a tenant instance referencing that type.

Keep these aligned with `Describe()` when you change capabilities — the
[change checklist](../AGENTS.md) requires it.

---

## `yggdrasil-quickstart.yaml` — the install bundle

The manifest `yggdrasil install` consumes. It declares the provider, the operator
inputs (instance name, namespace, image, …), the install steps (deploy the
adapter pod, register the `integration_instance`), and a read-only smoke test.
Ships full of `TODO:` markers you replace before your first release. See
[PUBLISHING](PUBLISHING.md) for how it is published and consumed.

---

## `Taskfile.yml`, `Dockerfile`, `docker-compose*.yml`

| Artifact | Purpose |
|---|---|
| `Taskfile.yml` | `task test` (`go test ./...`), `task config` (validate compose), `task build:image`, `task up` / `task down` / `task logs` (local stack). |
| `Dockerfile` | Multi-stage Go build producing the adapter image. |
| `docker-compose.yml` + `docker-compose.standalone.yml` | The local stack: RabbitMQ + the adapter, health port published. `task up` merges both files. |

---

## `.github/workflows/`

| Workflow | Trigger | What it does |
|---|---|---|
| `ci.yml` | push to `main`, PRs | `go run ./cmd/lint-action-catalog` → `go test ./...` → `docker build`. |
| `release.yml` | `vX.Y.Z` tag, push to `main` | Publishes the multi-arch adapter image to `ghcr.io/<owner>/<repo>` (`:vX.Y.Z` + `:latest` on tag, `:edge` on main). |
| `publish-oci.yml` | `vX.Y.Z` tag | Pushes `yggdrasil-quickstart.yaml` as an OCI artifact to ghcr.io for `yggdrasil install oci://…`. |
| `emit-deploy-event.yml` | deploy | POSTs a deploy event into yggdrasil-core (soft-skip if unconfigured). |
| `deploy.yml` | — | Placeholder; this repo is a template, not a deployed service. |
| `incident-escalation.yml`, `postmortem.yml` | Heimdall | Ops automation hooks. |

See [PUBLISHING](PUBLISHING.md) for the release/publish flow in depth.

---

## The AI-tool pointer files

`AGENTS.md`, `CLAUDE.md`, `GEMINI.md`, `CONVENTIONS.md`,
`.cursor/rules/yggdrasil.mdc`, `.codex/skills/repo-context/SKILL.md`,
`.windsurfrules`, `.github/copilot-instructions.md` — one per AI coding tool.
They all route back to **`INTEGRATION_CONTRACT.md`**. Update the contract and
every tool inherits the change. They survive scaffolding so your new adapter is
born contract-aware.

---

## OPERATIONS: where things live at runtime

| Concern | Where |
|---|---|
| Broker connection | `BROKER_URL` env, dialed in `main.go` (fatal if unset). |
| Liveness | `GET /healthz` → always `200 ok`. |
| Readiness | `GET /readyz` → `200 ready` when the broker connection is open, `503 rabbitmq_unavailable` when closed. |
| RPC reply contract | `{ok, data?, error?}` from `controllers/message/rpc.go`, sent to `reply_to` with the inbound `correlation_id`. |
| Failure handling | A handler error → `Nack` (no requeue) + a structured `error` reply; the loop continues. No silent swallowing. |

---

## Where to make a change

| You want to… | Edit |
|---|---|
| Add or rename a capability | `internal/adapter/spec.go` (Describe + Execute) → then tests + examples + README |
| Add a new resource type | `spec.go` `ResourceTypes` + `ActionCatalog` (keep them aligned) |
| Add a credential/instance field | `spec.go` schema + `examples/*.json` + quickstart inputs |
| Change transport plumbing | `controllers/message/*` |
| Change health/shutdown | `main.go` |
| Change the install experience | `yggdrasil-quickstart.yaml` |
| Change CI/release | `.github/workflows/*` |

---

*Where this fits in Yggdrasil:* the adapter is the **infra-reconciliation** layer
between yggdrasil-core and an external provider. See
[CONTRACT → where an integration fits](CONTRACT.md#where-an-integration-fits).
Back to the [README](../README.md) ·
[yggdrasil-core](https://github.com/dakasa-yggdrasil/yggdrasil-core).
