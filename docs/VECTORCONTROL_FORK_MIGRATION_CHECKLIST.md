# VectorControl Fork Migration Checklist

This document tracks the custom `vectorcontrol/main` direction for `sub2api` and the concrete features worth importing from `CLIProxyAPI` / live CPA lineage.

## Goals

- Keep `sub2api` as the long-term production base for VectorControl.
- Preserve upstream compatibility so `upstream/main` remains easy to merge.
- Selectively import proven hot-path ideas from `CLIProxyAPI` instead of switching the whole base.
- Prefer durable fixes for Codex / Responses traffic:
  - better session continuity
  - higher upstream prompt-cache hit rate
  - lower TTFT tail
  - lower long-run memory risk

## Current VectorControl Delta

The current fork delta is intentionally small and reviewable.

- Branch: `vectorcontrol/main`
- Base: `upstream/main`
- Current fork commit: `91583693`

Already added in the fork:

- OpenAI `plan_type` aware priority bias via `EffectivePriority()`
- more conservative OpenAI token refresh behavior when `expires_at` is absent

These changes live in:

- `backend/internal/service/account.go`
- `backend/internal/service/gateway_service.go`
- `backend/internal/service/openai_account_scheduler.go`
- `backend/internal/service/openai_gateway_service.go`
- `backend/internal/service/token_refresher.go`

## What sub2api Already Does Better

These areas are already stronger than public `CLIProxyAPI` and should remain the base architecture.

### Persistence and Ops

- request usage is durably stored in `usage_logs`
- TTFT is persisted as `first_token_ms`
- ops metrics are pre-aggregated into hourly and daily buckets
- sticky session state is backed by Redis rather than only process memory

### Scheduling

- account selection already combines priority, load, queue, error rate, and TTFT samples
- temporary unschedulable and overload cooldown are first-class account states
- pool-mode retries and account switching are integrated with scheduling

### Protocol Surface

- OpenAI Responses, WSv1, and WSv2 are all first-class paths
- `previous_response_id` continuation is handled at the routing layer
- `metadata.user_id` can bridge into sticky session and upstream cache identity

## High-Value Imports From CLIProxyAPI

These are the features worth porting into the fork instead of changing the product base.

### P0: Session Identity Extraction Parity

Status:

- implemented in `vectorcontrol/main`

Why:

- cache hit rate and continuity depend on stable session extraction
- `CLIProxyAPI` has a very explicit extraction order for `metadata.user_id`, `X-Session-ID`, `conversation_id`, and content fallback

Import target:

- normalize extraction precedence across all OpenAI/Codex HTTP + WS paths
- make fallback behavior identical for Responses and websocket continuations

Expected benefit:

- higher prompt-cache hit rate
- fewer cross-account continuation failures

### P0: Codex Websocket Execution Session Continuity

Why:

- `CLIProxyAPI` keeps a global execution-session store for Codex websocket reuse
- this reduces churn when a downstream session stays on the same upstream account

Import target:

- review whether `sub2api` should add a narrower execution-session cache on top of current WSv2 state, without weakening Redis-backed account stickiness

Expected benefit:

- lower websocket reconnect churn
- lower TTFT on follow-up turns

### P0: Responses Websocket Tool-Call Repair

Status:

- partially implemented in `vectorcontrol/main`

Why:

- `CLIProxyAPI` recently hardened the OpenAI Responses websocket path with session-scoped tool-call / tool-output repair
- this directly targets fragile multi-turn tool flows

Import target:

- port the repair logic only for the specific broken event patterns
- keep `sub2api`'s current routing and account ownership model intact
- current fork already repairs replay-input transcript gaps for orphan tool-call / tool-output pairs
- remaining work is a fuller session-scoped cache beyond last-turn replay input

Expected benefit:

- fewer edge-case tool-call breakages in Codex / Responses websocket sessions

### P1: Explicit Session-Affinity Diagnostics

Status:

- implemented in `vectorcontrol/main` for OpenAI handler scheduling logs

Why:

- `CLIProxyAPI` logs session-affinity hit/miss/reselect events very directly
- `sub2api` has stronger state, but the operator-facing diagnostics can still be improved

Import target:

- add compact structured logs or ops counters for:
  - sticky previous-response hit
  - sticky session hit
  - sticky miss with reselection
  - stale sticky binding cleared

Expected benefit:

- faster root cause analysis for cache drops and continuation drift

### P1: Per-Account Proxy Override Review

Why:

- `CLIProxyAPI` makes outbound route choice very explicit via `auth.ProxyURL`
- `sub2api` already supports account-bound proxy selection, but not a richer route-quality scheduler

Import target:

- keep account-bound proxy ownership
- add optional quality metadata and better operator visibility before considering any dynamic route scoring

Expected benefit:

- better control over tail latency and region-specific outbound behavior

### P2: OpenAI-Compatible Stream Completion Repair

Why:

- `CLIProxyAPI` recently fixed late-usage / missing final done handling in OpenAI-compatible streams

Import target:

- compare stream terminal-event handling between both codebases
- port only the minimal safety net where `sub2api` is weaker

Expected benefit:

- fewer incomplete usage writes for odd upstream stream endings

## Not Worth Importing As-Is

These ideas are not worth copying directly.

### In-Memory Usage Statistics Model

Do not import:

- `CLIProxyAPI` usage aggregation is process-memory centric
- model-level request details grow in memory and are exported/imported as snapshots

Reason:

- `sub2api` already has a better durable accounting model

### Pure In-Process Session Cache As Primary Stickiness

Do not import:

- `CLIProxyAPI` process-local session cache should not replace Redis-backed sticky state

Reason:

- restart safety and cross-instance behavior matter more than local simplicity

## Current Fork Risks To Fix

These are the custom-fork risks currently worth addressing before larger feature ports.

### 1. Auxiliary req.Client Pool Was Unbounded

Status:

- fixed in `vectorcontrol/main`

Summary:

- the shared OAuth helper `req.Client` cache used to be an unbounded `sync.Map`
- it now needs to stay bounded with TTL + eviction

Why it matters:

- imported or changing proxy URLs can otherwise grow helper clients forever

### 2. Hot-Path Truth Must Be Re-Checked Before Porting

Status:

- done for the current scheduler feedback path

Summary:

- some older analysis claimed TTFT scheduler feedback was not wired
- current code already reports live outcomes back into scheduling from OpenAI handlers

Why it matters:

- migration work should target real gaps, not stale ones

## Next Recommended Implementation Order

1. Keep the fork delta small and production-focused.
2. Finish low-risk runtime hygiene:
   - bounded helper client pools
   - better sticky-session diagnostics
3. Port Codex/Responses reliability features from `CLIProxyAPI`:
   - session extraction parity
   - websocket tool-call repair
   - execution-session continuity where it does not fight Redis stickiness
4. Only after that, consider optional proxy-quality routing signals.

## Acceptance Criteria For Future Ports

Any imported feature should satisfy all of these:

- no replacement of durable usage persistence with in-memory snapshots
- no weakening of Redis-backed sticky session semantics
- minimal fork delta against upstream
- unit tests added for the imported behavior
- no regression in OpenAI Responses HTTP, WSv1, or WSv2 compatibility
