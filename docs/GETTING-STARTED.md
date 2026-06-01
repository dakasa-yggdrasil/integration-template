# Getting started

> Scaffold a new adapter → implement your first capability → test it → publish to
> ghcr.io → let an adopter install it. End to end, with real commands.

This is the path from "I have an idea for an integration" to "an operator ran
`yggdrasil install` and it works". Keep the [Contract digest](CONTRACT.md) open
while you implement — it is the law your capabilities must obey.

---

## 0. Prerequisites

- **Go 1.25+** (the module targets `go 1.25.0`).
- **Docker** + the [Task](https://taskfile.dev) runner (`task`).
- The **`yggdrasil` CLI** (for scaffolding and installing).
- A running RabbitMQ for the local stack — `task up` brings one up for you.

---

## 1. Scaffold

The one-command path. `yggdrasil new` clones this template, strips its git
history, rewrites the module path and project name, and runs `git init`:

```sh
yggdrasil new integration my-thing --owner your-org
```

What that produces:

- a directory `./integration-my-thing`;
- module path rewritten to `github.com/your-org/integration-my-thing`;
- every `integration-template` reference rewritten to `integration-my-thing`;
- a fresh git repo (history-free), files staged.

`go test ./...` in the new directory passes immediately — the scaffold compiles
and round-trips out of the box.

> The bare name is **lowercase kebab-case** (`my-thing`), without the
> `integration-` prefix — the CLI adds it. `--owner` sets the GitHub owner used
> for the module path and the install hint; omit it and the scaffold uses
> `your-org` as a placeholder you fix later.

### Manual clone (alternative)

```sh
gh repo clone dakasa-yggdrasil/integration-template integration-my-thing
cd integration-my-thing
rm -rf .git && git init
# Then rewrite, by hand, what the CLI would have rewritten:
#   - module path in go.mod  → github.com/your-org/integration-my-thing
#   - every import of github.com/dakasa-yggdrasil/integration-template
#   - the string "integration-template" across the tree
```

The CLI does exactly these two rewrites (`integration-template` → your project,
and the template module path → your module path), then `git init`. Doing it by
hand is fine; just don't miss the import paths.

---

## 2. Understand what you got

Skim the [Anatomy](ANATOMY.md) once. The short version:

- **You implement in `internal/adapter/spec.go`** — `Describe()` (the catalog)
  and `Execute()` (the handlers). They must stay aligned.
- The transport (`controllers/message/`), health server (`main.go`), and wire
  types (`internal/protocol/`) are already wired — you rarely touch them.
- The shipped capabilities (`generate_installation`, …) are a **placeholder** you
  replace with real, canonically-named ones.

---

## 3. Implement your first capability

The shipped `Describe()` exposes placeholder operations on a `component` resource
type. Replace them with a real resource and the canonical prefixes. Worked
example: an `ensure_widget` / `observe_widgets` / `destroy_widget` triple.

**a. Decide the name.** Walk the
[naming decision flow](CONTRACT.md#naming-decision-flow). A thing with a stable
external identity that you "make exist" → `ensure_widget`.

**b. Declare it in `Describe()`** (`internal/adapter/spec.go`): add the operation
constants, list them in `SupportedExecuteOperations`, add the `resource_type`
(`widget`) with matching `default_actions`, and add the `ActionCatalog` entries.
Keep all three lists consistent — the linter checks it.

**c. Implement the handler in `Execute()`**: add a `case` for each operation that
decodes the typed request, calls your provider, and replies. Make `ensure_*`
**idempotent and adoption-aware** (GET-then-PUT; adopt a pre-existing resource;
`404` on destroy = success). See
[Contract → invariants](CONTRACT.md#what-an-integration-is-invariants).

**d. Emit a mutation event** on `ensure_*`/`destroy_*` success
(`mything.widget.ensured` / `.destroyed`) — see
[Contract → mutation events](CONTRACT.md#mandatory-mutation-events).

**e. Update tests, examples, and README in the same change** — this is a hard
rule in [AGENTS.md](../AGENTS.md). Update `examples/integration_type.example.json`
to match the new catalog, and `spec_test.go` to cover the new triple.

> **Credentials & secret-store:** if your provider needs credentials, declare a
> `credential_schema` in `Describe()` and read them via a `credentials_ref` URI —
> **never** hardcode a secret-store path. See the
> [Lego principle](CONTRACT.md#the-lego-principle).

---

## 4. Test it

```sh
task test          # go test ./...
```

Run the contract linter directly (CI runs this first):

```sh
go run ./cmd/lint-action-catalog
# → lint-action-catalog: ok (provider=… actions=N resource_types=M)
```

Bring up the full local stack — RabbitMQ + your adapter — and exercise the AMQP
round trip:

```sh
task up            # docker compose: rabbitmq + adapter, health on :8080
curl -s localhost:8080/healthz   # → ok
curl -s localhost:8080/readyz    # → ready  (503 rabbitmq_unavailable if broker is down)
task logs          # follow worker logs
task down          # tear it down
```

`task config` validates the compose files without starting anything.

> **Smoke discipline:** any test that creates a real provider-side resource MUST
> destroy it in the same run (`always: true` on the destroy step). See
> [Contract → smoke tests self-clean](CONTRACT.md#smoke-tests-self-clean).

---

## 5. Publish to ghcr.io

Push a SemVer tag. `release.yml` builds and pushes a multi-arch image:

```sh
git tag v0.1.0
git push origin v0.1.0
# → ghcr.io/your-org/integration-my-thing:v0.1.0  (+ :latest)
# pushes to main also publish :edge
```

The same tag fires `publish-oci.yml`, which pushes
`yggdrasil-quickstart.yaml` as an OCI artifact so adopters can install by
container ref. Full detail in [PUBLISHING](PUBLISHING.md).

Before the first real release, fill in the `TODO:` markers in
`yggdrasil-quickstart.yaml` (display name, provider description, **the image
ref**, the `integration_type` reference) and replace the placeholder smoke test
with a real read-only capability of yours.

---

## 6. An adopter installs it

With the quickstart published, an operator installs your adapter with one
command:

```sh
yggdrasil install your-org/integration-my-thing
# or by OCI ref:
yggdrasil install oci://ghcr.io/your-org/integration-my-thing:v0.1.0
```

The install flow walks the operator through your declared inputs (TUI by default,
`--non-interactive` for CI), deploys the adapter pod, and registers an
`integration_instance` in yggdrasil-core. From there, workflows can dispatch your
capabilities. See [PUBLISHING → how install consumes it](PUBLISHING.md#how-yggdrasil-install-consumes-it).

---

## Next steps

- [CONTRACT](CONTRACT.md) — the rules every capability must satisfy.
- [ANATOMY](ANATOMY.md) — every file, and where to change things.
- [PUBLISHING](PUBLISHING.md) — releases, OCI artifacts, install.
- [`INTEGRATION_CONTRACT.md`](../INTEGRATION_CONTRACT.md) — the full law.

Back to the [README](../README.md) ·
[yggdrasil-core](https://github.com/dakasa-yggdrasil/yggdrasil-core).
