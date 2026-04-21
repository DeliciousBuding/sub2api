# VectorControl Upstream Sync Runbook

This runbook defines how to keep the VectorControl fork close to `Wei-Shaw/sub2api` while preserving the custom production behavior on `vectorcontrol/main`.

## Scope

Use this runbook whenever upstream `sub2api` releases or when you want to manually sync the fork with `upstream/main`.

## Current Branch Model

- `main` tracks `upstream/main`
- `origin/main` stays upstream-compatible
- `vectorcontrol/main` contains the VectorControl production delta

Do not sync production by merging upstream directly into `vectorcontrol/main` without reviewing which fork-only commits are still needed.

## Current Fork Themes

Only keep custom commits in these categories:

- runtime hygiene
- Codex / Responses compatibility import
- operator observability

If a fork-only change no longer fits these themes, stop and review it before replaying it after an upstream update.

## Sync Procedure

### 1. Fetch and inspect upstream

```bash
git fetch upstream
git fetch origin
git checkout main
git log --oneline --decorate -n 20 upstream/main
```

Confirm that `main` is still your upstream-tracking base.

### 2. Update local `main`

Preferred:

```bash
git checkout main
git merge --ff-only upstream/main
```

If `main` has local-only commits, stop and clean that up first. `main` should remain the replay base.

### 3. Identify the live VectorControl delta

```bash
git log --oneline main..vectorcontrol/main
git diff --stat main...vectorcontrol/main
```

Review each fork-only commit and classify it:

- still required
- upstream already absorbed it
- obsolete / should be dropped

### 4. Rebuild `vectorcontrol/main` on top of fresh `main`

Preferred low-drift flow:

```bash
git checkout vectorcontrol/main
git rebase main
```

If the branch has become too messy, rebuild with explicit replay:

```bash
git checkout -B vectorcontrol/main main
# cherry-pick only the still-needed fork commits, in order
```

### 5. Resolve conflicts with fork rules

When a conflict appears:

- prefer upstream behavior unless the fork doc says the custom behavior is still required
- preserve Redis-backed sticky session semantics
- preserve durable usage / TTFT persistence
- do not reintroduce CPA-style in-memory stats or broad auth-first routing

### 6. Re-run focused regression tests

At minimum:

```bash
go test ./internal/repository -run "Test(GetSharedReqClient|CreateOpenAIReqClient|CreateGeminiReqClient)"
go test ./internal/service -run "Test(OpenAIGatewayService_(GenerateSessionHash|ResolveSessionIdentity|ExtractSessionID|SelectAccountWithScheduler)|ResolveOpenAIWSSessionHeaders|BuildOpenAIWSReplayInputSequence|OpenAIGatewayService_Forward_WSv2|OpenAIGatewayService_ProxyResponsesWebSocketFromClient|OpenAIWS)"
go test ./internal/handler -run "Test(OpenAIGatewayHandler|OpenAI|Responses|ChatCompletions)"
```

If the upstream change touched other hot paths, expand the targeted suite before pushing.

### 7. Update fork docs if behavior changed

If replayed or dropped commits changed the effective fork behavior, update:

- `docs/VECTORCONTROL_FORK_RUNTIME.md`
- `docs/VECTORCONTROL_FORK_MERGE_POLICY.md`
- `docs/VECTORCONTROL_FORK_MIGRATION_CHECKLIST.md`

### 8. Push only after the branch is coherent

```bash
git push origin main
git push origin vectorcontrol/main
```

## Decision Rules During Sync

### Keep the fork commit if

- upstream still lacks the required behavior
- the custom behavior protects Codex / Responses continuity
- the change is small, well-tested, and still within the allowed fork themes

### Drop the fork commit if

- upstream now behaves equivalently
- the custom code only duplicates upstream
- the custom code enlarges merge pain without current production value

## Current High-Value Fork Areas

These are the first places to verify after any upstream sync:

- unified OpenAI session identity resolution
- sticky-session diagnostics
- websocket transcript repair for tool-call / tool-output replay
- bounded auxiliary `req.Client` pool

## Bad Sync Patterns

Avoid these:

- merging upstream directly into `vectorcontrol/main` repeatedly without replay review
- keeping one-off experimental commits on the production fork
- changing `main` into a second custom branch
- batching unrelated fork work into one large conflict-heavy commit
