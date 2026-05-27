# Conventions — Yggdrasil integration repository

This file is consumed by **Aider** and other AI coding tools that look for `CONVENTIONS.md`. It is a short index pointing at the canonical contract.

## READ FIRST

**`INTEGRATION_CONTRACT.md`** in the repo root is the canonical law for what a Yggdrasil integration IS / IS NOT. Read it before making any change to this repo or to any adapter cloned from this template.

## Hard rules (one-liners — full text in the contract)

1. Resource ops use the four canonical prefixes: `ensure_<resource>` / `observe_<resource_type>` / `destroy_<resource>` / `discover_<resource_type>`. NEVER `create_*`, `list_*`, `delete_*`, `update_*` for resource operations.
2. **Lego principle**: NEVER hardcode AWS / GCP / Vault / RabbitMQ / Postgres / any specific cloud / secret store / broker / DB. Use abstractions (`credentials_ref` URI scheme, capabilities from other integrations).
3. Integration is **infrastructure reconciliation**, not business logic. No business state, no inbound webhook business processing, no business decisions.
4. Every mutation is **idempotent** and **adopts pre-existing resources** (GET-then-PUT, 409→success on adoption, 404→success on destroy).
5. **NEVER log** credentials, secrets, signing keys, or refresh tokens.
6. SemVer strict; breaking changes only at major bumps; compat shim via `WithLegacyNames` for one minor cycle.

## Self-test before opening a PR

```
[ ] All resource ops use ensure_/observe_/destroy_/discover_/on_ (or are on the allowlist)
[ ] No hardcoded cloud / secret store / broker / DB
[ ] No business state in the adapter
[ ] Every ensure_* adopts pre-existing resources
[ ] No credentials in logs, traces, or response payloads
[ ] Tests cover the canonical triple per resource type
[ ] action_catalog and resource_types[].default_actions are aligned
```

If any "no" — restructure before opening the PR.

## Pointer files for other AI tools

- `CLAUDE.md` — Claude Code
- `AGENTS.md` — OpenAI Codex / generic agent runners
- `GEMINI.md` — Gemini CLI
- `.cursor/rules/yggdrasil.mdc` — Cursor
- `.codex/skills/repo-context/SKILL.md` — OpenAI Codex Skills
- `.windsurfrules` — Windsurf
- `.github/copilot-instructions.md` — GitHub Copilot
- `CONVENTIONS.md` (this file) — Aider + generic

All of them point back to `INTEGRATION_CONTRACT.md`. Update the contract and the AI tools automatically inherit.
