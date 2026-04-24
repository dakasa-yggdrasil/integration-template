<div align="center">

# `integration-template`

**Starting point for a new [Yggdrasil](https://github.com/dakasa-yggdrasil/yggdrasil-core) integration adapter**

[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)
[![Part of](https://img.shields.io/badge/part%20of-yggdrasil-brightgreen.svg)](https://github.com/dakasa-yggdrasil/yggdrasil-core)

</div>

---

## What it is

A minimum-viable scaffold for writing a new integration adapter for
Yggdrasil. Clone, rename, implement the operations your family needs,
ship.

Integrations are the outbound edges of a Yggdrasil deployment. They run
as independent containers, speak the AMQP RPC contract with the core,
and execute operations declared by a workflow step. `kubernetes`, `aws`,
`github`, `grafana`, `rabbitmq`, and friends in the
[integration catalog](https://github.com/dakasa-yggdrasil/yggdrasil-core/blob/main/docs/catalog.md)
all started from this template.

## Use

**Recommended — one command:**

```sh
yggdrasil new integration my-thing --owner your-org
```

That clones this template, strips its git history, renames the Go module
to `github.com/your-org/integration-my-thing`, rewrites every
`integration-template` reference, and runs `git init` in the result.
`go test ./...` in the new directory passes out of the box.

Full walkthrough (operations, family manifest, publishing to ghcr.io):
[**extending guide**](https://github.com/dakasa-yggdrasil/yggdrasil-core/blob/main/docs/extending.md).

### Manual clone (alternative)

```sh
gh repo clone dakasa-yggdrasil/integration-template integration-my-thing
cd integration-my-thing
rm -rf .git && git init
# Rename the module in go.mod, update imports, implement your operations.
```

## What the template gives you

- `Dockerfile` + multi-stage Go build
- `Taskfile.yml` matching Yggdrasil workspace conventions
- AMQP RPC scaffolding: `describe`, `execute`, `health`
- Adapter skeleton in [`internal/adapter/spec.go`](internal/adapter/spec.go)
  with the `Describe()` contract and an `Execute()` switch ready for new
  operation cases
- Fake executor for tests (no real backend required)
- CI workflow skeleton
- `yggdrasil-quickstart.yaml` draft so adopters can install your integration
  with one command

## Adapter contract

Every Yggdrasil integration:

- Registers under a **family** (contract) and one or more **providers** (implementations)
- Exposes three mandatory operation categories: `describe`, `execute`, `health`
- Declares a credential schema + instance schema at the family or type manifest
- Ships a `yggdrasil-quickstart.yaml` so adopters install it with
  `yggdrasil install dakasa-org/integration-your-thing`

Read the full contract: [concepts → integrations](https://github.com/dakasa-yggdrasil/yggdrasil-core/blob/main/docs/concepts.md#integrations-family-type-instance-provider).

## License

Apache 2.0 — see [LICENSE](LICENSE).

---

<div align="center">

Part of [Yggdrasil](https://github.com/dakasa-yggdrasil/yggdrasil-core) · [Catalog](https://github.com/dakasa-yggdrasil/yggdrasil-core/blob/main/docs/catalog.md)

</div>
