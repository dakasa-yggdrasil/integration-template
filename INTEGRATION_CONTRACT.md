# Yggdrasil Integration — Definition & Contract

**Status**: canonical reference for ALL Yggdrasil integration adapters.
**Authority**: this document IS the law. New adapters and new capabilities MUST conform. Violations are reviewed in PR and rejected at registration time once Phase 2 schema validation lands.

---

## 0. ABSOLUTE RULE — Yggdrasil scope vs Backend scope (READ FIRST, NO EXCEPTIONS)

This rule decides whether a given operation belongs in a Yggdrasil integration OR in the operator's backend service. Confusing the two is the #1 architectural mistake AI assistants and human authors make. Always re-read this before designing a new capability or refactoring a backend call.

### The line

**Yggdrasil = Internal Developer Platform (IDP)** — it manages resources that are the operating company's **own internal responsibility**: things the company needs to set up, maintain, and audit so its products and team can operate. Audience: the internal team (collaborators, devops, platform engineers, ops).

**Backend = end-user-facing business** — it runs operations for the company's **end-users** (customers, consumers, clients). Audience: the people who consume the company's product, not the company itself.

### How to tell which side a resource belongs to

Ask: "Whose responsibility is this resource, the COMPANY's or the END-USER's?"

- Company's responsibility → **Yggdrasil integration** territory
- End-user's responsibility → **Backend** territory

### Concrete examples (Stripe — the easiest case to confuse)

| Operation | Whose concern | Who handles it |
|---|---|---|
| Register the Stripe webhook URL on Stripe Dashboard so backend can receive `payment_intent.succeeded` events | The COMPANY (internal infra setup) | **Yggdrasil** via `integration-stripe ensure_webhook_endpoint` |
| Configure the Stripe API key in the company's secret store | The COMPANY (internal setup) | **Yggdrasil** (via `integration-secrets-management`) |
| Create the Stripe Connect platform account for the company itself | The COMPANY | **Yggdrasil** via `integration-stripe ensure_connect_account` |
| Charge end-user U $X for product Y they bought | The END-USER (business transaction) | **Backend** (`enterprise-payments-api` calls Stripe directly) |
| Create a Stripe customer record for end-user U signing up | The END-USER (their account) | **Backend** |
| Refund end-user U for order Z | The END-USER (their transaction) | **Backend** |
| Subscribe end-user U to plan P | The END-USER | **Backend** |

### Concrete examples (other providers)

| Operation | Whose concern | Who handles it |
|---|---|---|
| Provision the AWS S3 bucket that hosts the app's static assets | Company infra | **Yggdrasil** via `integration-aws ensure_s3_bucket` |
| Upload end-user U's profile picture to a bucket | End-user data | **Backend** (calls S3 directly) |
| Create the GitHub repo for a new team project | Company internal | **Yggdrasil** via `integration-github ensure_repository` |
| Issue a SaaS user's GitHub-linked OAuth token | End-user identity | **Backend** |
| Register the EFI Pix webhook URL so backend receives Pix callbacks | Company infra | **Yggdrasil** via `integration-efi ensure_webhook_subscription` |
| Create a Pix charge for end-user U paying R$100 for product Y | End-user transaction | **Backend** (`dakasa-identities` calls EFI directly) |
| Configure NFe.io webhook URL for the company's emission events | Company internal | **Yggdrasil** via `integration-nfeio ensure_webhook_subscription` |
| Emit a NFSe for end-user U's purchase of product Y | End-user business doc | **Backend** (`enterprise-payments-api` calls NFe.io directly) |

### Why this matters

If the rule is violated:
- **Putting end-user operations in Yggdrasil** → integration becomes a thin wrapper around a provider SDK; loses Crossplane-style declarative value; couples business logic to infra layer; adds latency for every business action.
- **Putting company-infra ops in the backend** → infra setup gets scattered across business services; no audit trail; no central reconciliation; each backend reinvents webhook registration / secret management.

The line is sharp and the architectural separation is what makes Yggdrasil valuable. **Never blur it.**

### Quick self-test before designing a capability

```
Question: "If the company changes ownership / sells / shuts down / hands the
           system to a new operator, does this resource follow the company or
           the end-user?"

Follows the company → Yggdrasil integration owns it.
Follows the end-user → Backend owns it.
```

Example: the Stripe webhook URL endpoint is a company-owned config (it points to the company's backend); it follows the company. The end-user's payment record is theirs (and the company processes it on their behalf); it follows the end-user.

---

## 1. Definition (single sentence)

> **A Yggdrasil integration is the declarative infrastructure layer that reconciles the state of external resources (third-party APIs, external systems, cloud providers) with the desired state expressed by consumers — guaranteeing idempotency, adoption of pre-existing resources, and drift detection — without ever containing business logic and without assuming any specific cloud, secret store, or storage backend.**

---

## 2. The Lego Principle (NON-NEGOTIABLE)

Yggdrasil is built so every piece is **generic, agnostic, and composable**.

**An integration MUST NOT assume**:
- Which cloud the operator uses (AWS, GCP, Azure, on-prem)
- Which secret store backs `credentials_ref` (AWS SM, GCP Secret Manager, HashiCorp Vault, sealed-secrets, K8s Secret)
- Which queue broker carries events (RabbitMQ, Kafka, NATS, Redis Streams)
- Which orchestrator schedules the workflow (Yggdrasil-core itself today, anything else tomorrow)
- Which database the operator runs
- Which observability stack (Prometheus, Datadog, OpenTelemetry-only)

**An integration accesses these via abstractions** (URI schemes, capability calls to other integrations, SDK transport interfaces) — never via direct hard-coded calls to a specific provider's SDK at the wrong layer.

**Examples**:
- ✅ `credentials_ref: "secret://provider/path"` where `provider` is `aws-sm`, `gcp-sm`, `vault`, `k8s-secret`, etc. Yggdrasil-core resolves the URI via the appropriate secrets-management integration.
- ❌ `aws_sm_path: "dakasa/prod/stripe"` hardcoded in instance config schema. Locks operator into AWS.
- ✅ `event_emitter` configured to publish via the `rabbitmq` integration's `publish_message` capability.
- ❌ Adapter directly imports `github.com/streadway/amqp` and dials a hardcoded broker URL.

**Exceptions**: rare and EXTREMELY documented. When an integration must couple to a specific provider (e.g. `integration-aws` legitimately only manages AWS resources), the coupling is in the **integration's scope itself**, not in its consumption of infra primitives. Even `integration-aws` reads its own credentials via `credentials_ref` — the operator chooses which secret store holds the AWS credentials.

**If you find yourself writing "must be AWS" or "must be Vault" anywhere outside the integration's primary scope, STOP and revisit. The exception requires:**
1. An ADR explaining why no abstraction is feasible.
2. A comment block on the offending line/struct citing the ADR.
3. Code review sign-off from a platform maintainer.

---

## 3. What an integration **IS** (invariants every adapter satisfies)

| Property | Meaning |
|---|---|
| **Resource-oriented** | Thinks in "resources with stable external identity": S3 bucket, Stripe customer, EFI Pix charge, GitHub repository. Each has a provider-side ID that persists. |
| **Declarative** | Caller declares **what should exist** (desired state), not **how to make it exist** (procedural steps). |
| **Idempotent by contract** | Every mutation is safe to retry. Same input → same output. Each `ensure_*` performs GET-then-PUT internally so adoption + drift correction collapse into one call. |
| **Adoption-aware** | If a resource already exists in the provider (created by another path), the adapter **adopts** it — does not duplicate, does not error. |
| **Drift-detectable** | Caller can call `observe_*` at any time and compare with its desired-state manifest. Adapter MAY optionally expose `DriftReporter` to compute drift internally. |
| **Multi-tenant** | One integration_type (`stripe`) serves N integration_instances (`stripe-acme`, `stripe-corp`), each with isolated credentials. The instance is the tenant. |
| **Stateless adapter** | Adapter does NOT store business state. State lives in the provider (source of truth) and partially in the operator's secret store (credentials + provider-generated secrets like webhook signing keys). |
| **Audit-emitting** | Every mutation logs (structured) and emits an event (via the operator-chosen event bus). Workflows and Heimdall observe these. |
| **Self-healthchecking** | Exposes `/healthz` (liveness) and `/readyz` (ready to serve). Adapter SDK wires these; describe handshake validates surface contract at registration. |
| **Provider-agnostic from above** | Caller workflows don't know the adapter is calling Stripe over HTTPS or EFI over mTLS or AWS over Signature V4. They invoke `ensure_<resource>`; adapter handles transport. |

---

## 4. What an integration is **NOT**

| Anti-pattern | Why it's NOT integration |
|---|---|
| **NOT a re-packaged provider SDK** | Integration adds **convention, idempotency, adoption, drift detection** on top of a provider SDK. Not just a thin wrapper around `stripe.PaymentIntents.New()`. |
| **NOT business logic** | Doesn't decide WHO to charge, HOW MUCH, WHEN to refund. Caller workflows + backend services make those decisions. Integration executes the desired state. |
| **NOT a webhook business processor** | Provisions the webhook subscription (creates URL registration at provider, captures signing secret, persists it via abstract `credentials_ref`). Receiving real webhook payloads and processing them as business events belongs in backend services. |
| **NOT a storage/persistence layer** | No database of its own. No business state. Anything the adapter knows is derivable from `observe_*` calls. |
| **NOT a UI / surface** | Frontend SPAs are a separate layer. Integration is backend infra only. |
| **NOT a workflow** | Workflows **compose** integration capabilities. Integrations don't compose workflows. |
| **NOT a runtime** | Not the application running. The infra the application depends on. |
| **NOT cloud-coupled (Lego!)** | Does not require a specific cloud, secret store, broker, or DB. See §2. |

---

## 5. The Resource Contract (four canonical prefixes)

```
ensure_<resource>(desired_state)        → observed_state
  Mutating, idempotent, adopts-if-exists. Caller says "I want this
  state"; adapter brings it about. Same input → same output.

observe_<resource_type>(filter?)        → ([]observed_state, cursor)
  Read-only, no side effects. Lists resources, optionally filtered.
  Single-resource lookup is filter={id:"..."} — no separate get_*.

destroy_<resource>(ref)                 → result
  Terminal removal. 404 from provider (already gone) = success.
  Idempotent in the same sense as ensure_*.

discover_<resource_type>(scope)         → []resource
  Service-side enumeration: walks provider's external state to find
  resources the adapter doesn't know about. Optional capability.
```

**Exceptions allowed (explicit allowlist)**:
- `on_<event>` reactor capabilities (framework-invoked on external events; not callable via Execute)
- Pure-function helpers: `verify_signature`, `calculate_<thing>`, `dispatch_workflow`, `merge_pull_request`
- Money-movement actions: `create_payout`, `create_refund` — even with idempotency key, the semantics are "do this one-shot side effect", not "ensure this resource state"

---

## 6. Heuristic: Resource vs Action vs Reactor vs Helper

```
Is it a RESOURCE?
  → Has stable external identity (ID, name, ARN) that survives across calls
  → You can observe_<resource_type>() and get back a list with stable IDs
  → Provider API is structured around the identity (GET /resource/{id})
  → USE: ensure_<resource> / observe_<resource_type> / destroy_<resource>

Is it an ACTION?
  → One-shot side effect, no durable external identity
  → Calling twice produces two independent effects
  → Provider API is a POST with no resource ID in response
  → USE: name on the allowlist (no canonical prefix)
  → Examples: send_email, merge_pull_request, dispatch_workflow

Is it a REACTOR?
  → Framework invokes it when an external event arrives
  → Not callable via Execute by user
  → USE: on_<event> + category="reactor"

Is it a HELPER?
  → Pure function, no provider call, no side effect
  → Examples: verify_webhook_signature, calculate_iss
  → USE: name on the allowlist
```

---

## 6.5. Golden Rule — Mandatory mutation event emission (NON-NEGOTIABLE)

**Every `ensure_<resource>` and `destroy_<resource>` capability MUST emit a mutation event after the underlying operation succeeds.** No exceptions for resources. The event is what makes the mutation **auditable** (immutable record beyond `workflow_run`) and **reactive** (other adapters and workflows can subscribe to it).

### Event naming convention

```
<provider>.<resource>.<verb_past>

Examples:
  efi.charge.ensured
  efi.charge.destroyed
  stripe.customer.ensured
  stripe.subscription.destroyed
  nfeio.service_invoice.ensured
  github.repository.ensured
  github.team_membership.destroyed
```

The verb is **past tense** (`ensured`, `destroyed`) because the event records a fact that already happened. Reactor capabilities subscribe via `on_<provider>_<resource>_<verb>` matching the inbound pattern already in use.

### Payload schema

```jsonc
{
  "event_type":   "stripe.customer.ensured",
  "provider":     "stripe",
  "resource":     "customer",
  "verb":         "ensured",                  // or "destroyed"
  "resource_id":  "cus_1234abc",              // provider-side stable identity
  "instance_id":  "stripe-acme",              // integration_instance scope (multi-tenant)
  "idempotency":  "ensure_customer_acme_abc", // dedup key for re-emissions
  "observed":     { /* full observed state */ },
  "emitted_at":   "2026-05-27T10:30:00Z"
}
```

### Where the rule comes from

The pattern already exists in Yggdrasil ecosystem for cross-adapter reactivity: when a user is created in Yggdrasil core, a reactor on `integration-github` fires to send the GitHub invite. Mutation event emission **extends this pattern from core-level events to integration-level resource mutations** — so the same reactor mechanism that already handles user-created can now handle `stripe.customer.ensured`, `efi.charge.destroyed`, etc.

### Implementation (SDK v0.6.0+)

Adapter authors do NOT write boilerplate. The SDK `reconcile.RegisterReconciler` wraps `Ensure()` and `Destroy()` to automatically call `events.Emit(ctx, MutationEvent{...})` on success. Adapter authors only provide the `Reconciler[D, O]` interface; emission is mechanical.

```go
// Adapter author writes (unchanged from v0.5.0):
reconcile.RegisterReconciler(a, "customer", "customers", customerReconciler)

// Behind the scenes (SDK v0.6.0):
// - After Ensure() returns success, SDK emits "stripe.customer.ensured"
// - After Destroy() returns success, SDK emits "stripe.customer.destroyed"
// - Emission goes through yggdrasil-core HTTP API (auth-gated) → event_log table → MaterializeReactions → reactors fire
```

### What happens if you don't emit

If an adapter ships an `ensure_*` that does not emit:
1. The mutation is **silent**: no audit beyond `workflow_run` row.
2. Cross-adapter reactivity **breaks**: no reactor can subscribe.
3. Heimdall pulses can't observe the change without polling — wasted infrastructure.
4. Phase 2 (hard-fail) validator flags the adapter as non-conformant.

This rule is the difference between "I called Stripe's API through a wrapper" and "Yggdrasil reconciled the Stripe resource and the world knows."

### Exemption: read-only and helper capabilities

`observe_*` and `discover_*` do NOT emit (read-only). Pure-function helpers (`verify_signature`, `calculate_iss`) do NOT emit. Reactors (`on_*`) do NOT emit (they consume, they don't mutate). Money-movement actions (`create_payout`, `create_refund` — on allowlist) MUST emit equivalent events using the same convention: `efi.payout.created`, `stripe.refund.created` — `created` instead of `ensured` because they aren't idempotent declarative ops.

---

## 7. Architectural Layering (where integrations sit)

```
┌───────────────────────────────────────────────────────────┐
│ SURFACE (Yggdrasil console, federated SPAs)               │ ← UI
├───────────────────────────────────────────────────────────┤
│ BACKEND SERVICES (operator-owned)                         │ ← Business logic
│   • Receives external webhooks; processes payments        │
│   • Decides who to charge / refund / credit               │
│   • Owns business state (wallets, invoices, accounts)     │
├───────────────────────────────────────────────────────────┤
│ WORKFLOW (Yggdrasil workflows)                            │ ← Orchestration
│   • Composes integration capabilities                     │
│   • Heimdall pulses, scheduled reconciliations            │
├───────────────────────────────────────────────────────────┤
│ INTEGRATION (adapter pods)            ◄── YOU ARE HERE    │ ← Infra reconciliation
│   • ensure_/observe_/destroy_ of external resources       │
│   • Idempotency + adoption + drift detection              │
│   • Provisions webhook subscriptions (does NOT receive    │
│     webhook payloads for business processing)             │
├───────────────────────────────────────────────────────────┤
│ EXTERNAL PROVIDERS (Stripe, EFI, GitHub, AWS, k8s, etc.)  │ ← Source of truth
└───────────────────────────────────────────────────────────┘
```

If your code reaches up into business logic or stores business state, you're not writing an integration — you're writing a backend service.

---

## 8. Lifecycle Invariants (mandatory at registration)

- **SemVer strict**. Breaking changes (rename capability, change input schema, remove resource type) ONLY at major version bump.
- **Compat shim for one minor cycle**. SDK provides `WithLegacyNames("old_op_name", ...)` so callers have time to migrate. Removed at the next major bump.
- **Credentials via `credentials_ref`**, a URI to the operator's chosen secret store. NEVER inline in manifests. Adapter resolves via `secrets-management` integration. (§2 Lego Principle.)
- **Provider-generated secrets** (webhook signing keys, OAuth refresh tokens) captured by the adapter's `ensure_*` and persisted back through the same `credentials_ref` indirection so business consumers (backend services) read them from the same path. Adapter NEVER returns these in capability response payloads except on the very first creation call.
- **No secrets in logs**. Not in zap fields, not in trace span attributes, not in RTA envelope metadata, not in error messages.
- **Describe handshake** after boot. Yggdrasil core validates that the adapter responds and returns a conformant catalog. Failure to handshake = adapter not eligible for capability dispatch.

---

## 9. Forbidden anti-patterns (will be rejected in PR or by validator)

1. ❌ CRUD-style capability names (`create_*`, `list_*`, `get_*`, `delete_*`, `update_*`) for resource ops — violates convention.
2. ❌ Integration receiving external webhook to process business logic — that's a backend service, not an integration.
3. ❌ Integration with its own database — that's a backend service.
4. ❌ Non-idempotent mutation without an idempotency key — caller cannot safely retry.
5. ❌ Capability that does not adopt pre-existing resources — creates duplicates or errors on second run.
6. ❌ Logging credentials, secrets, signing keys, or refresh tokens at any log level.
7. ❌ Hardcoding a specific cloud / secret store / broker / DB in the adapter image. Violates §2 Lego Principle.
8. ❌ Reactor (`on_*`) that makes business decisions — must only emit normalized event envelope for backend consumer.
9. ❌ Inline credentials in `integration_instance` config. Always `credentials_ref`.

---

## 10. Self-test "Am I writing an integration?" (checklist before opening PR)

```
[ ] Am I provisioning / reconciling an external resource?
    YES → integration. NO → wrong layer.

[ ] Does the resource have a stable external identity (ID, name, ARN)?
    YES → use ensure_/observe_/destroy_.
    NO  → use action allowlist.

[ ] Is my mutation safe to retry?
    YES → continue.
    NO  → add GET-then-PUT logic to make it idempotent.

[ ] If the resource already exists, does my adapter adopt it gracefully?
    YES → continue.
    NO  → bug; fix adoption logic.

[ ] Am I storing any business state in the adapter?
    NO → continue.
    YES → move to backend service.

[ ] Am I receiving external webhook payloads to process as business events?
    NO → continue.
    YES → backend service, not integration.

[ ] Does my capability name follow ensure_/observe_/destroy_/discover_/on_?
    YES → continue.
    NO  → rename, or document an explicit allowlist exemption.

[ ] Did I hardcode AWS / GCP / Vault / RabbitMQ / Postgres anywhere?
    NO  → continue.
    YES → STOP. Replace with abstraction (§2 Lego Principle). If
          impossible, write ADR and obtain platform maintainer sign-off.

[ ] Does my capability log any credential, secret, signing key, or token?
    NO  → continue.
    YES → STOP. Remove logging. This is a security breach.

[ ] Is my breaking change at a major version bump with a compat shim?
    YES → continue.
    NO  → bump to next major OR add WithLegacyNames(...) shim.
```

If you answered "YES → wrong layer" to question 1, or any "NO" / "YES → STOP" — do not open the PR until resolved.

---

## 11. References

- Convention spec (full migration tables): `docs/superpowers/specs/2026-05-27-yggdrasil-integration-capability-convention.md` in `dakasa-system`
- Rollout plan: `docs/superpowers/plans/2026-05-27-yggdrasil-integration-convention-rollout.md`
- Reference Crossplane-conformant adapters: `integration-aws`, `integration-rabbitmq`, `integration-kubernetes`
- Reference adapter scaffold: this repo (`integration-template`)

---

*This contract supersedes any earlier per-adapter guidance. New CLAUDE.md / AGENTS.md in adapter repos MUST link to this document and reaffirm conformance. The yggdrasil-core schema validator enforces a subset of this contract at registration time (Phase 1 warn-only, Phase 2 hard-fail).*
