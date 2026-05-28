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

### §5.b — Response envelope echoes inbound operation name (NON-NEGOTIABLE)

When an adapter receives a request using a legacy operation name that resolves through `LegacyOperationAliases` (or any equivalent compat-shim table) to a canonical handler, the **response envelope MUST echo the inbound (legacy) operation name** — NOT the post-resolution canonical name.

**Rationale**: yggdrasil-core's response-operation-matches-request-operation guard (`controllers/message/integrations.go`, the check `response.Operation != req.Operation`) rejects mismatches as an integrity violation. Echoing the canonical name would cause every legacy-aliased call to fail at the router level — defeating the entire point of the `LegacyOperationAliases` shim. The guard exists so a misrouted response from a different adapter cannot be mistaken for the requested one; it is intentionally strict.

**Wrong** (will fail with `unexpected adapter operation` from core):

```go
// Adapter receives req.Operation = "describe_workflow_run" (legacy alias).
// Handler dispatches to the canonical `observe_workflow_run` implementation
// and returns the canonical name on the envelope — guard rejects.
return Response{Operation: OperationObserveWorkflowRun, Output: result}  // BAD
```

**Right**:

```go
// Adapter captures the inbound name before alias resolution and writes it
// back onto the response envelope before returning to the caller.
inbound := req.Operation
canonical := resolveAlias(inbound)
resp, err := dispatchByCanonicalOperation(canonical, req)
if err != nil {
    return resp, err
}
resp.Operation = inbound   // echo inbound name
resp.Capability = inbound  // same rule for Capability field
return resp, nil
```

**Where to put the echo**: Two acceptable shapes.

1. Per-handler: each handler reads `req.Operation` (captured *before* the alias translation) and stamps it onto the response. Brittle — every new handler has to remember.
2. Centralized at the `Execute()` entry point: `Execute` captures the inbound name, calls a private `dispatchByCanonicalOperation` that knows about canonical names only, then overrides `resp.Operation` and `resp.Capability` with the inbound names before returning. Recommended — one place to maintain.

**Scope**: this rule applies to all four operation categories (`ensure_*`, `observe_*`, `destroy_*`, `discover_*`) plus every action on the allowlist (`verify_signature`, `dispatch_workflow`, `create_payout`, etc.) that passes through aliased names. It applies whether the alias table is in adapter-local Go (this template), in `sdk-go`'s `reconcile.WithLegacyNames`, or in any future framework-level alias registry.

**Discovery context**: this rule was discovered during the `kubernetes` adapter v1.13.0 rename, where a CRUD-style legacy capability (e.g. `delete_secret`) was aliased to its canonical `destroy_secret` handler. The handler echoed `destroy_secret`, the router-level guard saw `response.Operation != req.Operation`, and every legacy-aliased dispatch failed with a 500. Reflected in the cycle 2026-05-27 conformance pass — adapters that still echo canonical-only are subject to the same failure mode the moment a caller still publishes a v1.x name during the deprecation window.

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

## 11. Smoke tests MUST self-clean (NON-NEGOTIABLE)

Every smoke workflow that creates a provider-side resource (Stripe customer, GitHub repo, Slack channel, Grafana folder, etc.) MUST destroy that resource in the SAME workflow run. No exceptions.

**Why**: smoke artifacts that leak accumulate as catalog clutter, billing surprises (Stripe customers), API rate-limit waste, and confused successor maintainers. Every cycle that adds a smoke without cleanup creates a debt that compounds. The 2026-05-27 cycle ended with leftover smoke customers on `stripe-dakasa-validation`, dozens of `smoke-*` / `ad-*` / `adhoc-*` workflow manifests cluttering the catalog, and a `smoke-bridge-v231` Grafana folder hanging around — all of which had to be reaped retroactively.

**Required pattern**:

```yaml
steps:
  - id: ensure
    use: integration: <provider> / capability: ensure_<resource>
    inputs: { ... }

  - id: observe
    use: integration: <provider> / capability: observe_<resource_type>
    inputs: { filter: ... }
    assert: 1 item matching ensure output

  - id: destroy
    use: integration: <provider> / capability: destroy_<resource>
    inputs: { ref: ${{ steps.ensure.metadata.output.id }} }
    always: true   # ← MUST run even if a prior step failed
```

**Always-run cleanup**: even if an intermediate step fails, the destroy step MUST execute (`always: true` or whichever equivalent the workflow engine offers). A smoke that creates a resource and then aborts before destroy is the worst kind of leak — undetected until a billing surprise or catalog audit.

**Workflow manifests are smoke artifacts too**: ad-hoc smoke workflows registered in the Yggdrasil catalog (e.g. `smoke-foo-2026-05-27`) MUST be deleted from the catalog after the smoke run completes, OR scheduled for auto-deletion via TTL on the manifest. Cluttering the catalog with leftover smoke workflows is the same anti-pattern as leftover Stripe customers — just at a different layer. Use `integration-yggdrasil-self delete_manifest` (shipped v2.1.0) to delete the manifest from the smoke's own final step.

**Smoke checklist (for reviewers)**:

- [ ] Resource ensured + observed + destroyed in the same run
- [ ] destroy step uses `always: true` (or workflow-engine equivalent)
- [ ] Smoke workflow manifest itself is deleted (or marked TTL) after first use
- [ ] No `test_run=YYYY-MM-DD` tags lingering in provider after run
- [ ] event_log has both `<provider>.<resource>.ensured` and `<provider>.<resource>.destroyed` rows post-run

**Companion rule on capability authoring**: when you ship a new `ensure_*` capability, ship the paired `destroy_*` capability in the same PR. A new resource type without a destroy is not production-ready — every smoke that exercises the ensure will leak on every run.

---

## 12. Backend is the authorization authority (NON-NEGOTIABLE)

Every state-changing HTTP endpoint exposed by yggdrasil-core or any backend integration MUST enforce authorization at the HANDLER level (or in a guard middleware that runs BEFORE the handler). The check MUST verify:

1. **Authenticated principal** — request carries a valid session, workflow run token, or service token. Anonymous requests are rejected with HTTP 401.
2. **Authorized for the operation** — the principal's effective permissions include the operation being requested. Otherwise HTTP 403.
3. **Scope match** — for tenant-scoped resources, the principal's tenant matches the resource's tenant. Cross-tenant access requires explicit elevation (admin token).

**Anti-patterns (forbidden)**:

- Routes registered without any middleware (anonymous writes are CRITICAL bugs)
- Permission checks ONLY in the UI / surface (UI checks are decorative hints, NEVER security)
- Opt-in auth via `requireXxx` decorators on each route — default-deny middleware wraps EVERY new route
- Permissions inferred from the request body (e.g., `if body.owner_id == session.user_id` checks performed in handler — should be at the data layer's scope filter)

**Verification rule**: a new route is incomplete until a test exists that submits the request with NO auth header AND expects HTTP 401, AND a test that submits with WRONG-permission auth and expects HTTP 403.

**For surfaces**: the §3.6 "no business decision authority" invariant in `SURFACE_CONTRACT.md` explicitly extends to authorization. Surfaces render based on permission HINTS from the session, but the BACKEND must validate every state change. A surface that shows a "Delete" button does NOT imply the user can delete — only the backend's enforcement determines that.

**Why this clause exists**: 2026-05-27 audit found `POST /api/v1/manifests` and ~10 GET endpoints in production WITHOUT auth — leaking credential paths and customer IDs. Surface had decorative `usePermission` checks that did NOTHING server-side. This clause codifies the lesson permanently.

---

## 13. Session lifecycle is centrally authoritative (NON-NEGOTIABLE)

When a collaborator explicitly logs out, or has their session revoked by an admin, or has their account offboarded, the revocation MUST propagate to EVERY system that has an active delegated session for that collaborator. yggdrasil-core is the authority; consumers MUST honor the revocation in real time (or close to it).

**Three propagation mechanisms** (use the one appropriate for the consumer):

1. **RFC 8417 Back-Channel Logout** — preferred for OIDC clients. yggdrasil-core's OIDC issuer MUST send a signed `logout_token` to each registered client's back-channel logout URL upon session termination. Clients MUST verify the token and invalidate the corresponding local session WITHOUT requiring user interaction.

2. **Mutation event reactor** — for any integration adapter that delegates auth (Slack, Google Workspace, GitHub, Grafana, etc.). The adapter MUST implement a reactor `on_collaborator_session_terminated` (registered in `integration.yaml::reactors[]`). yggdrasil-core emits the event upon session revocation; the reactor's job is to terminate the user's session in the upstream system using the provider's appropriate API.

3. **OIDC token introspection** — fallback for legacy consumers that can't be modified to support 1 or 2. yggdrasil-core MUST expose `/api/v1/oidc/introspect` returning `{active: bool, ...}` per RFC 7662 (OAuth 2.0 Token Introspection). Consumers SHOULD call this on every request (with caching ≤30s) — accepting some staleness in exchange for not requiring back-channel.

**Anti-patterns (forbidden)**:

- Stateless JWTs trusted indefinitely with no revocation lookup (security hole — exactly the bug fixed 2026-05-27 in tartaro)
- "Logout" handlers that only clear local cookies without informing yggdrasil
- Per-system logout that doesn't propagate to the IDP
- `AccessTokenLifetime > 5 minutes` without one of the three mechanisms above

**Where the event fires (yggdrasil-core)**:
- User clicks "Logout" → DELETE session row → INSERT session_revocation → emit `collaborator.session.terminated` → dispatch RFC 8417 logout_token to OIDC clients + materialize reactor invocations
- Admin revokes session → same emit
- Password rotated → same emit (forces re-auth everywhere)
- Account offboarded → same emit + permanent revocation

**Reactor contract (for adapters)**:
- Capability name: `on_collaborator_session_terminated`
- Input: `{collaborator_id, primary_email, reason: "logout"|"revoked"|"password_rotated"|"offboarded", emitted_at}`
- Behavior: call upstream API to terminate user's session in the integration (e.g., revoke OAuth token, expire SSO session, etc.). MUST be idempotent.
- Failure mode: log + WARN; do not block other reactors. yggdrasil-core retries via the standard reactor framework.

**Lego principle**: this clause is provider-agnostic. RFC 8417 is an open IETF standard, not vendor-specific. The reactor pattern uses the existing SDK framework. Introspection follows RFC 7662. No specific cloud / IDP vendor required.

**Why this clause exists**: 2026-05-27 audit found that a user could explicitly close their yggdrasil session and STILL log into tartaro via SSO — the tartaro JWT (15-min TTL) was valid until natural expiry. This violates the principle that the IDP is authoritative.

---

## 14. Canonical error response shape (RFC 7807 Problem+JSON)

Every HTTP error response (4xx/5xx) from yggdrasil-core or any backend integration MUST conform to RFC 7807 Problem+JSON:

```json
{
  "type": "https://yggdrasil.dakasa.me/errors/auth/invalid-credentials",
  "title": "Invalid credentials",
  "status": 401,
  "detail": "The provided email or password is incorrect.",
  "code": "auth.invalid_credentials",
  "instance": "/api/v1/auth/login"
}
```

**Required fields**:
- `code` — stable machine-readable identifier (dotted namespace, e.g. `auth.invalid_credentials`, `manifest.validation_failed`, `permission.denied`). The frontend's i18n table keys off this. NEVER drift the code string.
- `status` — HTTP status code (redundant with HTTP envelope but required for clients that lose status during proxy chains).
- `title` — short human description (English, for logs).
- `detail` — longer description, may include context (still English).

**Optional fields**:
- `type` — URI to the error documentation (allows machine discoverability).
- `instance` — the URI of the specific failing operation.
- Additional context fields (e.g. `field` for validation errors, `locked_until` for rate-limit, `correlation_id`).

**Forbidden** (these were the Frankenstein shapes the 2026-05-27 audit found):
- `{error: "..."}` — discontinue. Migrate to Problem+JSON with `code`.
- `{message: "..."}` — same.
- `{reason: "..."}` — same.
- Plain text bodies with status codes — replace with Problem+JSON.
- HTTP 200 with `{error: ...}` in body — emit proper status code.

**i18n strategy**: backend emits English in `title` and `detail`. Surfaces map `code` → localized message via a single per-locale table. No more N independent humanizer regex tables in frontend code.

**Migration**: existing endpoints that emit `{error: "..."}` MUST be migrated; the `code` is added alongside for one minor version, then the legacy field is removed.

**Migration status (yggdrasil-core, 2026-05-28)**:

| Phase | Scope | Status |
|-------|-------|--------|
| 2B-core | Universal `writeMappedError` / `writeJSONError` writers | DONE — every typed-error path goes through `internal/httperr.WriteProblem` |
| 2B-close | Hand-rolled `writeJSON(w, status, map[string]any{...,"code":...})` sites in auth/MFA/password handlers | **DONE 2026-05-28** — 25 sites migrated; new dotted codes shipped (auth.mfa_invalid, auth.password_too_weak, auth.invalid_current_password, auth.setup_token_invalid, auth.reset_token_invalid, auth.kek_not_configured, auth.webauthn_not_implemented, auth.mfa_factor_unavailable, auth.password_unchanged, auth.password_change_required, input.unknown_fields) |
| 2B-pending | `{error: "..."}` sites in invites / saml / scim_admin / integration_webhook / workflow_runs / external_identities / team_sync / tartaro_actions / integration_type_sync | Pending — flagged warn-only by the lint rule |
| 2C | Removal of legacy `error` key | After one-minor deprecation window |

**Regression guard** (yggdrasil-core `scripts/lint-no-legacy-error-envelopes.sh`):

```bash
# Forbidden: writeJSON(w, 4xx/5xx, map[string]{...}) — hand-rolled
# error envelope. Use httperr.WriteProblem(w, status, code, title, detail, opts...)
git diff --name-only origin/main...HEAD -- 'controllers/httpapi/*.go' \
  | xargs grep -nE 'writeJSON\(.*http\.Status(BadRequest|Unauthorized|Forbidden|NotFound|Conflict|UnprocessableEntity|Locked|TooManyRequests|PreconditionRequired|NotImplemented|ServiceUnavailable|InternalServerError).*map\[string\]' \
  && { echo "::error::§14 violation"; exit 1; } || true
```

Files already migrated are hard-fail; pending files emit warnings only. Adding a new closed file extends the `CLOSED_FILES` array in the lint script.

**Why this clause exists**: 2026-05-27 co-design audit found 3 different humanizer tables in surfaces (console x2 + tartaro x1), each with ~50 regex rules trying to translate unstructured backend error strings into pt-BR. Backend emits English `err.Error()`; surfaces guess at translation. Stable `code` strings eliminate the guesswork.

---

## 15. Adapter manifests carry UI metadata (drive surfaces from Describe)

When an adapter declares `credential_schema` or `instance_schema` properties, the schema MUST include UI metadata sufficient for any compliant surface to render a usable form WITHOUT per-provider hardcoded knowledge.

**Required per-property fields**:

```yaml
properties:
  efi_client_key_id:
    type: string
    label: "EFI Client Key ID"
    label_locale:
      pt-BR: "EFI: Chave de cliente"
      en-US: "EFI: Client key"
    placeholder: "Client_Id_xxxxxxxxxxxx"
    group: "EFI Credentials"
    order: 1
    description: "Find this in EFI Pix dashboard → Settings → API Credentials"
    description_locale:
      pt-BR: "Encontre em: EFI Pix dashboard → Configurações → Credenciais API"
    required: true
    sensitive: false      # if true, surface renders as password field
    depends_on:            # optional: only show this when another field has a specific value
      field: "mtls_enabled"
      value: true
```

**Forbidden anti-patterns** (these are the friction the 2026-05-27 audit found in `OpsIntegrationsPage.tsx::fallbackConfigureFields`):

- Surface hardcoding per-provider field knowledge ("if integration_type == 'efi', show fields A, B, C")
- Surface inferring labels from field names (regex-converting `client_key_id` → "Client Key Id")
- Surface providing default placeholders that mention specific companies
- Surface hardcoding field grouping logic per provider

**Surface contract amendment**: surfaces MUST be data-driven from `integration_type` Describe(). The form renderer accepts a schema and produces UI; no per-provider branches.

**Lego principle**: every integration provider has its own field set, but the SHAPE of the metadata is universal. Adding a new provider does NOT require touching surface code — only the adapter's `spec.go` Describe() output.

**Why this clause exists**: 2026-05-27 audit found `OpsIntegrationsPage.tsx` at 2481 LoC because `fallbackConfigureFields()` hardcodes forms for 7 providers. Backend `IntegrationSchemaProperty` lacks `Label`/`Placeholder`/`Group`/`Order`/`DependsOn` — UI compensates with hardcoded knowledge. Extending the schema lets the surface be generic.

---

## 16. Adapter Deployment topology — single-container by default (NON-NEGOTIABLE)

An adapter pod runs **one container** — the adapter binary itself — unless a sidecar has a documented purpose **AND** a distinct port topology. This is enforced at the Deployment manifest level: the `spec.template.spec.containers[]` array of the canonical adapter Deployment MUST contain exactly one container named `integration-<provider>` (matching the Deployment name).

**Why this rule exists**: 2026-05-28 close-out cycle found `integration-slack` stuck in a rollout-revert loop because a manually-injected `adapter` sidecar (experimental HTTP bridge from the #243 Option B cycle, never productionized) was sharing port 8080 with the canonical `integration-<provider>` container — both bound to `HEALTHCHECK_PORT`, the second crashed with `bind: address already in use`, Kubernetes never replaced the older single-container ReplicaSet, Phase 2D code never reached production. The Phase 2D fix-and-roll cycle for 5 adapters was blocked by this one mis-configured Deployment.

**Allowed sidecar patterns** (each requires explicit ADR documenting the contract):
- Adapter + transport bridge (e.g., legacy HTTP→AMQP gateway during a migration). Bridge MUST listen on a distinct port from the main adapter's `HEALTHCHECK_PORT` (default 8080). Bridge MUST have its own readiness/liveness probe targeting its own port. The Deployment MUST emit a single Service with separate `targetPort` values per container.
- Adapter + auth proxy (e.g., istio-proxy, oauth2-proxy). Same port-distinction rule applies; sidecar must be opt-in via annotation, not the default.
- Adapter + init container for credential bootstrapping. Init containers are fine because they run to completion before the main container starts and do not bind ports concurrently.

**Anti-patterns (forbidden)**:
- Two containers, both binding to port 8080 (the canonical `HEALTHCHECK_PORT`). Strategic-merge patch by container name **silently appends** the second container instead of replacing; both crash-loop on the conflict.
- Manual `kubectl apply` / `kubectl patch` modifications that drift the cluster from the Yggdrasil-managed canonical spec. The runtime monitor's forward-drift detector catches this on the *next* describe handshake, but only after the bad rollout has already broken availability.
- Mutating an adapter Deployment via `kubectl edit` instead of through a Yggdrasil workflow that uses `apply_manifest` or `ensure_deployment_spec`. There is no audit trail; the field manager registry shows `kubectl` as owner of fields the canonical reconciler should own.

**Where the rule lives**: any Yggdrasil workflow that dispatches `apply_manifest` for an adapter Deployment MUST emit a single-container `template.spec.containers[]` array. The canonical example is the `ad-roll-adapter-with-emission-wire` workflow — its `pod_spec_patch.containers[].name` defaults to `{{ inputs.deployment_name }}`, which IS the canonical single-container name. Operators dispatching this workflow MUST NOT override `container_name` to a value that differs from the Deployment name; doing so triggers the strategic-merge-append anti-pattern.

**Verification**: post-roll, `kubectl -n dakasa get deploy integration-<name> -o jsonpath='{.spec.template.spec.containers[*].name}'` MUST return a single name matching the Deployment.

---

## 17. References

- Convention spec (full migration tables): `docs/superpowers/specs/2026-05-27-yggdrasil-integration-capability-convention.md` in `dakasa-system`
- Rollout plan: `docs/superpowers/plans/2026-05-27-yggdrasil-integration-convention-rollout.md`
- Reference Crossplane-conformant adapters: `integration-aws`, `integration-rabbitmq`, `integration-kubernetes`
- Reference adapter scaffold: this repo (`integration-template`)

---

*This contract supersedes any earlier per-adapter guidance. New CLAUDE.md / AGENTS.md in adapter repos MUST link to this document and reaffirm conformance. The yggdrasil-core schema validator enforces a subset of this contract at registration time (Phase 1 warn-only, Phase 2 hard-fail).*
