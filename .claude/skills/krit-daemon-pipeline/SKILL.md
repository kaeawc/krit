---
name: krit-daemon-pipeline
description: Use when working on Krit's daemon mode, analysis pipeline, cross-file cache, bundle/findings cache, WorkspaceState residency, conservative delta planner, or invalidation semantics. Covers cold vs warm correctness, per-phase timings, and the cache flags used to verify behavior vs cached output.
---

# Krit Daemon and Pipeline

Use this when changing how the analyzer runs end-to-end — daemon residency, cache wiring, bundle reuse, delta planning, or phase timings. Cross-link with `krit-kaa-benchmarking` when the change touches the oracle's daemon lifecycle.

## Mental Model

Each scan flows through phases the dispatcher schedules after the required indexes are built: parse, suppression indexing, AST dispatch, project-scope phases (cross-file, module-aware, parsed-file, Android, Gradle), aggregate, then output. Daemon mode keeps long-lived state alive across scans in `WorkspaceState`:

- `CodeIndex` — source symbol/reference index
- `TypeResolver` — source-level type resolution
- `AnalysisCache` — per-file analysis output (write-back, lookup-side)
- Oracle (KAA) lifecycle — resident JVM, oracle call filter classification cache
- Findings-bundle cache — full-hit short-circuit when a bundle has not changed

The `ConservativeDeltaPlanner` decides which files genuinely need rework after an edit. "Body-only" edits should reuse the prior bundle rather than re-running cross-file.

## Cold vs Warm Correctness

Two failure modes recur:

- **Warm cache returns stale findings after a rule or config change.** Always use cache-disabling flags when verifying behavior, not when measuring perf:
  ```bash
  ./krit -no-cache -perf -f json -q -o /tmp/krit.json /path/to/project || true
  ```
- **Invalidation does not cover every resident piece of state.** When a config or rule registration changes, invalidation must cover resident resolver, oracle filter, code index, and bundle cache. See `fix(pipeline) InvalidateAll covers resident resolver + oracle filter` (#133).

When changing any resident state, walk every `InvalidateAll` / shutdown path and confirm the new state is included.

## Per-Phase Timings

Daemon mode exposes per-phase wall-time stats. They are the first thing to inspect when latency regresses:

```bash
./krit -perf -f json -q -o /tmp/krit.json /path/to/project || true
jq '.perfPhaseStats' /tmp/krit.json
jq '.perfRuleStats' /tmp/krit.json
```

Useful fields:

- total duration vs per-phase duration (parse, dispatch, cross-file, android, gradle, aggregate, output)
- KAA-related: type-oracle phase, JVM analyze phase, KAA files analyzed
- daemon-only: ResolverHit, OracleFilterHit, bundle hit/miss components

When a bundle misses, `diag(pipeline) report bundle miss components` (#144) emits the reason. Read that before guessing.

## Bundle Reuse And Delta Planning

The findings-bundle cache short-circuits scans whose inputs match a prior bundle. Two invariants:

- **Body-only edits reuse the bundle.** See `fix(pipeline) reuse bundle for body-only edits` (#154). If a body-only edit forces a full scan, the planner has lost precision — diagnose at the planner, not at the rules.
- **Conservative delta planner pairs with structural cross-file.** A delta that touches structural API must invalidate cross-file dependents; a delta that does not, must not. See `feat(daemon) wire ConservativeDeltaPlanner with structural CrossFile` (#135).

## When To Add Daemon State

Promote state into `WorkspaceState` when:

1. it is expensive to rebuild
2. it is keyed on inputs that change less often than file edits
3. its invalidation rules are clear and survive the `InvalidateAll` audit

If any of those fails, prefer rebuilding per scan. Daemon state with unclear invalidation is the biggest source of warm-cache correctness bugs in this codebase.

## Validation

```bash
go build -o krit ./cmd/krit/
go vet ./...
golangci-lint run ./...
go test ./... -count=1
make integration
make regression
```

`make integration` and `make regression` are required for daemon/pipeline changes — `go test ./...` does not exercise the CLI/LSP/MCP integration paths or the playground regression expectations.

## Reporting Standard For Perf Changes

When reporting daemon perf changes, include:

- Krit revision and target revision
- scan command and cache flags
- whether the config was applied
- cold vs warm per-phase timings
- the bundle hit/miss components from the diag output
- whether `InvalidateAll` was audited for any new state

Treat cached warm findings as performance evidence only — never as behavioral truth after rule or config changes.
