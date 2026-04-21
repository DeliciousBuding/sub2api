# VectorControl Release Runbook

This runbook defines how to prepare, canary, and roll out the `vectorcontrol/main` fork into production.

It assumes the current production topology:

- `metapi` remains the public entry
- `sub2api` is the backend serving layer
- Docker remains the runtime packaging model

## Scope

Use this runbook when:

- shipping new `vectorcontrol/main` commits to production
- rebuilding the VectorControl Docker image
- canarying Codex / OpenAI Responses continuity changes
- preparing rollback before touching live traffic

## Release Principles

- keep the fork delta small and categorized
- prefer one coherent release change at a time
- do not combine upstream sync, schema changes, and hot-path behavior changes into one blind rollout
- always prepare rollback before changing live traffic

## Release Types

### 1. Safe runtime release

Examples:

- docs only
- bounded helper client changes
- observability-only log additions

Expected rollout:

- build
- smoke test
- normal deploy

### 2. Hot-path compatibility release

Examples:

- session identity extraction
- sticky-session behavior
- websocket continuation repair
- tool transcript handling

Expected rollout:

- build
- focused tests
- canary
- live metric watch
- broad cutover only after continuation health is stable

## Pre-Release Checklist

Before building an image:

- branch is `vectorcontrol/main`
- `git status` is clean
- fork docs are updated if behavior changed
- rollback target commit or image tag is recorded
- focused tests pass for the touched area

Minimum commands for current OpenAI / Codex hot path:

```bash
cd backend
go test ./internal/service -run "Test(OpenAIWS|OpenAIGatewayService_ProxyResponsesWebSocketFromClient|ResolveOpenAIWSSessionHeaders|BuildOpenAIWSReplayInputSequence|OpenAIGatewayService_(GenerateSessionHash|ResolveSessionIdentity|SelectAccountWithScheduler))"
go test ./internal/handler -run "Test(OpenAIGatewayHandler|OpenAI|Responses|ChatCompletions)"
go test ./...
```

## Config Baseline

Unless a release explicitly changes these, keep the current baseline:

- `gateway.openai_ws.enabled=true`
- `gateway.openai_ws.responses_websockets_v2=true`
- `gateway.openai_ws.mode_router_v2_enabled=false`
- `gateway.openai_ws.ingress_mode_default=ctx_pool`
- `gateway.openai_ws.sticky_session_ttl_seconds=3600`
- `gateway.openai_ws.sticky_response_id_ttl_seconds=3600`
- `gateway.openai_ws.store_disabled_conn_mode=strict`

Ingress / proxy baseline:

- every Nginx hop in front of Codex traffic must set `underscores_in_headers on;`
- do not introduce buffering on streaming paths during the same rollout

## Build and Tag

Use a VectorControl-tagged image. Do not ship an unqualified local image with no rollback marker.

Suggested tag shape:

```text
vectorcontrol-YYYYMMDD-<shortsha>
```

Example:

```bash
git rev-parse --short HEAD
docker build -t ghcr.io/deliciousbuding/sub2api:vectorcontrol-YYYYMMDD-SHORTSHA .
```

If the environment requires a local registry or server-local build, still record:

- commit SHA
- image tag
- previous image tag

## Canary Procedure

Use canary whenever the release changes OpenAI / Codex continuation behavior.

### Canary traffic scope

Prefer:

- a small subset of Codex / Responses traffic
- one or two known-active downstream keys
- real multi-turn continuation traffic

Avoid:

- full traffic cutover first
- synthetic single-turn-only validation as the only gate

### Canary checks

Confirm all of the following:

- `/v1/models` smoke passes through `metapi -> sub2api`
- `/v1/responses` single-turn smoke returns 200
- multi-turn Codex / Responses continuation stays successful
- websocket reconnect or new-client same-session continuation still succeeds
- no spike in `invalid_encrypted_content`
- no spike in `previous_response_not_found`
- TTFT does not regress materially

## Metrics and Logs to Watch

During canary and immediately after broad rollout, watch:

- continuation-related 400s
- `invalid_encrypted_content`
- `previous_response_not_found`
- sticky miss / reselection patterns
- account failover frequency
- TTFT p50 / p95
- request latency p50 / p95
- 429 / 529 rate

For this fork generation, the highest-signal areas are:

- OpenAI WS mode logs
- sticky-session diagnostics
- metapi-side upstream error summaries

## Broad Rollout Gate

Do not broaden from canary until:

- focused smoke is green
- targeted test suite is green
- canary continuation traffic is stable
- no new continuation regression signature appears in logs
- rollback image and command are already prepared

## Rollback Rules

Rollback immediately if any of these appear after rollout:

- clear rise in continuation-related 400s
- new `invalid_encrypted_content` cluster
- canary session reconnect starts failing
- TTFT or total latency regresses beyond acceptable operational range
- account reselection behavior becomes unstable or opaque

Rollback should restore:

- previous known-good image tag
- previous runtime config if config changed
- previous metapi backend target only if needed

Do not mix rollback with new code changes. First restore service health, then analyze.

## Release Record

For every production release, record at least:

- release date
- commit SHA
- image tag
- previous image tag
- whether this was safe runtime or hot-path compatibility
- canary scope
- observed result
- rollback target

Store the release note in the control-plane repo or the live operations note for that environment.

## Current VectorControl Hot-Path Notes

Current custom hot-path behaviors that require extra attention during rollout:

- unified session identity extraction across HTTP and websocket paths
- bounded helper `req.Client` pool
- websocket tool transcript replay repair
- session-scoped websocket tool transcript cache for reconnect / new-client repair

These are intentionally narrow imports from CPA / CLIProxyAPI lineage, not a replacement of upstream `sub2api` architecture.
