# GEMINI

## 🔐 READ FIRST: `INTEGRATION_CONTRACT.md`

Before any change in this repo or any adapter cloned from it, read [`INTEGRATION_CONTRACT.md`](./INTEGRATION_CONTRACT.md). It defines what a Yggdrasil integration IS / IS NOT, the four canonical capability prefixes (`ensure_/observe_/destroy_/discover_`), the **Lego principle** (no cloud/secret-store/broker/DB hardcoding — Yggdrasil is provider-agnostic), and the forbidden anti-patterns.

If you find yourself naming a capability `create_*`, `list_*`, `delete_*`, `update_*` for a resource — STOP and re-read §5 + §10.
If you hardcode AWS / Vault / RabbitMQ / Postgres — STOP and re-read §2.

Then read `AGENTS.md` for repo-specific rules.

Focus areas:
- Keep this repository transport/runtime focused.
- Keep protocol types local.
- Validate any capability change against README, tests, and examples.
