# DidChangeOracleRefresh

**Cluster:** [lsp](README.md) · **Status:** open ·
**Severity:** n/a (infra) · **Default:** n/a · **Est.:** 2 days

## What it does

Keeps the oracle index fresh as the user edits. Wires `textDocument/didChange`
through to an oracle re-analysis of the changed file, debounced to
avoid re-analyzing mid-keystroke. Replaces the changed file's entries
in the reverse index (milestone 1) without a full project re-index.

Without this milestone, navigation results go stale the moment the
user types a character — FQNs point at old locations, rename misses
newly added references.

## Current cost

The diagnostics side of `didChange` already works and is debounced
via `scanner.DebouncedDispatch`. See
[`internal/lsp/server.go:340-470`](../../../internal/lsp/server.go:340).
It re-runs rules on the file, publishes diagnostics, done.

The oracle side currently does nothing on `didChange`. The oracle
cache was designed for one-shot invocations: compute full oracle,
return, exit. Integrating it as a long-lived navigation backend means
handling per-file updates without rebuilding the whole oracle.

## Proposed design

### Per-file re-analyze RPC

`krit-types` already accepts `--files LISTFILE` to re-analyze a subset.
Add a long-lived mode where the daemon stays warm and accepts JSON-RPC
requests to re-analyze a given file:

```json
{"jsonrpc": "2.0", "method": "analyze", "params": {"files": ["/abs/path/to/Foo.kt"]}, "id": 1}
```

Response:

```json
{"jsonrpc": "2.0", "id": 1, "result": {"files": {"/abs/path/to/Foo.kt": {...FileResult...}}}}
```

Reuses the existing CRaC/AppCDS-warmed session. No JVM relaunch per
file.

### Incremental index update

On receiving a refreshed `FileResult` for file F:

1. Remove every entry in `idx.decls` where `decl.File == F`.
2. Remove every entry in `idx.refs` where `ref.File == F`.
3. Remove every entry in `idx.exprs` whose id prefix is F's path hash.
4. Re-insert entries from the new `FileResult`.

All three steps are linear in the deleted/inserted counts, not in the
total index size. Typical file has O(10) declarations and O(100)
expressions — a few microseconds of index mutation per edit.

Thread safety: `idx` mutations happen behind `idx.mu` (RWMutex).
Queries from in-flight LSP requests acquire a read lock. Writes to
the index take the write lock briefly.

### Debounce strategy

LSP `didChange` fires on every keystroke. Existing diagnostic dispatch
debounces at 100ms. Oracle re-analysis costs are higher, so use a
longer window:

- First `didChange` for a file schedules a 400ms timer.
- Subsequent `didChange` resets the timer.
- Timer fires → oracle re-analyze → index update.

400ms matches JetBrains IDEA's default re-analysis window. Navigation
queries during the 400ms window return results from the pre-edit index.
That's acceptable — the user just typed; they're not simultaneously
querying.

### Stale-query protection

If a navigation query arrives while a re-analyze is in-flight, the
query uses whichever index state is currently committed. No
serialization — the navigation response may be slightly stale by up
to 400ms. Explicitly documented in the handler comments.

### Write-through to the on-disk cache

The oracle's content-addressable cache at `.krit/types-cache/entries/`
already keys by content hash. When the daemon computes a fresh
`FileResult` for an edited file, write it to the cache under the new
content hash. Next cold start picks up the cached entry instead of
re-analyzing. This makes the edit-undo-edit cycle instantaneous once
the oracle has seen a given content hash.

## Files to touch

- `krit-types` (JVM side) — add daemon mode with JSON-RPC over stdio.
  Currently `krit-types` exits after one invocation; needs a `--daemon`
  flag that keeps the session alive and accepts re-analyze requests.
  **~2-3 days of JVM work that's not included in the Go-side estimate.**
- `internal/oracle/daemon.go` — new, Go-side client that spawns and
  talks to `krit-types --daemon`
- `internal/oracle/index.go` — `ApplyFileUpdate(file string, newResult *FileResult)`
  incremental mutation
- `internal/lsp/server.go` — `handleDidChange` gains an oracle-refresh
  scheduler after the existing diagnostic scheduler
- `internal/lsp/debounce.go` — extract or extend debouncer to support
  multiple independent buckets (diagnostics vs oracle)

## Testing

- Edit a function body → re-analyze preserves the declaration's FQN
  but updates expressions → goto-def still works, hover type changes.
- Rename a function (edit its name at the declaration site) → old
  FQN disappears from index, new FQN appears → goto-def on a caller
  now resolves through the old identifier (probably shows "unresolved"
  which is correct pending user fixing callers).
- Rapid keystrokes → only one re-analyze fires per 400ms window, not
  one per keystroke.
- Daemon crash mid-analyze → navigation falls back to textual walker
  (milestone 3's fallback path) until daemon restarts.

## Risks

- **Daemon is the hardest unshipped piece in this cluster.** Currently
  `krit-types` is a one-shot JVM. Making it long-lived changes its
  lifecycle: session must be resettable without re-initializing
  `KotlinCoreEnvironment`, which is a singleton per JVM (the infamous
  KT-64167). Worth confirming this is doable before committing to
  the milestone — may force us to accept re-launching the JVM every
  N minutes if state corruption compounds.
- **Memory growth in long-lived daemon**. Every re-analyze mutates
  FIR caches. Unreferenced caches eventually GC, but Analysis API
  is known to leak across sessions in some versions. Monitor RSS;
  plan a recycle-after-N-hours policy.
- **Re-analyze blocking the daemon**. If the daemon is mid-analyze
  for file A and file B's `didChange` arrives, B waits. Acceptable
  for typical usage (users edit one file at a time) but document
  the serialization.

## Blocking

- Milestones 5 and 6 want this for responsiveness.

## Blocked by

- [`fqn-symbol-index.md`](fqn-symbol-index.md) (milestone 1) — index
  to mutate
- [`navigation-handlers-rewrite.md`](navigation-handlers-rewrite.md)
  (milestone 3) — handlers that consume the refreshed index

## Links

- Parent cluster: [`lsp/README.md`](README.md)
- Oracle daemon context:
  [`roadmap/clusters/core-infra/type-resolution-service.md`](../core-infra/type-resolution-service.md)
- Scratch notes on oracle cache design:
  [`scratch/oracle-cache-optimization.md`](../../../scratch/oracle-cache-optimization.md)
  (if retained)
