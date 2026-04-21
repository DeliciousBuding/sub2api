# VectorControl Fork Merge Policy

This document keeps the fork rebaseable against `Wei-Shaw/sub2api`.

## Branch Model

- `main` tracks `upstream/main`
- `origin/main` mirrors upstream-compatible base
- `vectorcontrol/main` is the only long-lived custom branch

Do not create additional long-lived product branches unless the fork model is formally changed.

## Merge Discipline

For every upstream sync cycle:

1. fetch `upstream/main`
2. update local `main`
3. review whether any existing fork-only patch was superseded upstream
4. replay only the still-needed fork delta onto `vectorcontrol/main`
5. rerun focused regression tests for affected hot paths

## Fork-Only Change Requirements

Every fork-only change must include:

- focused tests
- one short doc note in `docs/`
- a merge-risk statement:
  - low: additive diagnostics or isolated runtime hygiene
  - medium: hot-path logic that is still structurally local
  - high: protocol-shape or scheduler-contract changes

## Review Questions

Before merging any fork-only change, answer:

- can upstream absorb this later?
- does this weaken Redis sticky semantics?
- does this weaken durable persistence or ops visibility?
- does this enlarge the long-term merge surface unnecessarily?
- is the change solving a current proven issue, not a speculative one?

## Regression Gates

At minimum, rerun focused tests for:

- OpenAI Responses HTTP
- OpenAI websocket continuation
- sticky session selection
- previous_response_id recovery
- prompt cache identity behavior

## Preferred Fork Style

- prefer small helper layers over broad rewrites
- prefer importing narrow, proven behaviors from `CLIProxyAPI`
- prefer compatibility patches at boundaries over core architecture replacement
