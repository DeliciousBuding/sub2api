# VectorControl Fork Runtime Handbook

This document defines the runtime stance for the `vectorcontrol/main` fork.

## Runtime Position

- `metapi` remains the public edge.
- `sub2api` remains the serving backend for Codex / OpenAI Responses traffic.
- `vectorcontrol/main` is a small production fork, not a product rewrite.

## Allowed Fork-Only Categories

Every fork-only change must belong to one of these categories:

- runtime hygiene
- Codex / Responses compatibility import
- operator observability

Changes outside these categories need an explicit design note before implementation.

## Runtime Invariants

- keep Redis-backed sticky session as the source of truth
- keep durable usage persistence and TTFT persistence
- do not import CPA-style in-memory usage accounting
- do not replace the scheduler with auth-first simple routing
- prefer additive diagnostics over behavior-changing rewrites

## Current VectorControl Runtime Customizations

- OpenAI `plan_type` aware account priority bias
- conservative OpenAI refresh behavior when `expires_at` is absent
- bounded auxiliary `req.Client` pool for OAuth/privacy helper traffic
- unified OpenAI session identity resolution across HTTP + websocket entry paths
- sticky-session and continuation diagnostics in OpenAI handlers
- websocket tool transcript replay repair for orphan tool-call / tool-output pairs
- session-scoped websocket tool transcript cache for reconnect / new-client continuation repair
- tracked import checklist for `CLIProxyAPI` feature absorption

## Release Defaults

Current VectorControl production stance for OpenAI WS / Responses traffic:

- keep `gateway.openai_ws.enabled=true`
- keep `gateway.openai_ws.responses_websockets_v2=true`
- keep `gateway.openai_ws.mode_router_v2_enabled=false` unless a canary explicitly needs mixed ingress modes
- keep `gateway.openai_ws.ingress_mode_default=ctx_pool`
- keep `gateway.openai_ws.sticky_session_ttl_seconds=3600`
- keep `gateway.openai_ws.sticky_response_id_ttl_seconds=3600`
- keep `gateway.openai_ws.store_disabled_conn_mode=strict`
- keep nginx `underscores_in_headers on;` on every reverse-proxy hop that fronts Codex / CLI traffic

These defaults are the baseline that the fork tests against. If a rollout changes them, record the reason in the rollout note before shipping.

## Current Priority Order

1. stability and continuation correctness
2. cache continuity
3. TTFT reduction
4. outbound route observability
5. optional route-quality scheduling

## Candidate Import Queue

### P0

- unified session identity extraction parity
- Responses websocket tool-call repair
- Codex websocket execution-session continuity without bypassing sticky account ownership

### P1

- sticky-session diagnostics and counters
- better per-account / per-proxy route observability
- stream terminal-event repair only where upstream drift is proven

### Explicitly Rejected Imports

- in-memory usage snapshot as the primary stats model
- process-local sticky session as the primary continuity layer
- broad proxy-pool scheduler rewrite before observability exists

## Rollout Rule

When a fork-only behavior changes request routing or continuation behavior:

- land focused tests first or in the same change
- validate with canary traffic before broad rollout
- record the import and merge-risk in `docs/`

Use `docs/VECTORCONTROL_RELEASE_RUNBOOK.md` for the concrete release, canary, and rollback sequence.
