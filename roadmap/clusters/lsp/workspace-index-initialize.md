# WorkspaceIndexInitialize

**Cluster:** [lsp](README.md) · **Status:** open ·
**Severity:** n/a (infra) · **Default:** n/a · **Est.:** 3–5 days

## What it does

Builds the oracle index across the entire workspace when the LSP
server receives `initialize`, reporting progress via LSP `$/progress`
notifications. Makes the first navigation query fast (index already
warm) instead of forcing the user to wait for an oracle cold-start.

Without this, the first goto-def on a freshly opened workspace hangs
for 1–45 seconds depending on project size and cache state. With this,
the user sees an "Indexing…" progress bar during `initialize` and
every subsequent query is sub-100ms.

## Current cost

The oracle is computed on-demand today. For rule runs, that's once
per `krit` invocation. For LSP, that works for diagnostics (we want
them for the open file only, so a lazy per-file analysis is fine).
For navigation, laziness is a UX problem — the user doesn't know the
first query will pay for the whole project's analysis.

## Proposed design

### Initialize-time index build

On `initialize`:

1. Read the workspace root(s) from `params.workspaceFolders`.
2. Enumerate Kotlin source files via `scanner.CollectKtFiles` (already
   exists, already respects exclude globs).
3. Classify files against the oracle cache (already fast — milestone
   from the 2026-04-14 cache optimization push; see
   `internal/oracle/classify.go` if retained).
4. For cache hits, load `FileResult` from disk; no JVM spawn.
5. For cache misses, spawn `krit-types` (warm daemon from milestone 4
   once it lands; otherwise one-shot) and analyze in batches.
6. Build the reverse index (milestone 1) from the assembled oracle.
7. Publish `$/progress` updates as batches complete.
8. Commit the built index to the server and resolve the `initialize`
   response with navigation capabilities advertised.

### Progress reporting

LSP's `$/progress` protocol requires the client to opt in during
`initialize`. Most modern clients (VS Code, Neovim nvim-lspconfig,
IntelliJ LSP4IJ) do. The server emits:

```json
// Begin
{"kind": "begin", "title": "Krit: indexing workspace", "cancellable": false,
 "percentage": 0, "message": "Scanning 2483 Kotlin files"}

// Progress (every N files or every M ms)
{"kind": "report", "percentage": 37, "message": "Analyzing 912/2483 files"}

// Done
{"kind": "end", "message": "Indexed 2483 files (41829 declarations)"}
```

If the client doesn't support `$/progress`, fall back to
`window/showMessage` at start and end only.

### Non-blocking initialize

The `initialize` response doesn't wait for the index — it returns
immediately with navigation capabilities advertised and the index
builds in the background. Navigation queries that arrive before the
index is ready either (a) block up to a short timeout (500ms) and
then fall back to the textual walker, or (b) return a `null` result
and expect the client to retry. Option (a) is simpler and what modern
LSP clients already do for slow servers; use that.

### Cache-first strategy on cold workspaces

First-ever open of a large project (kotlin/kotlin, no cache) costs
minutes for a full analysis. That's unavoidable given the Analysis
API; the value-add of this milestone is making it feel contained:

- Report progress so the user knows the wait is bounded.
- Warm-start after: cache persists between sessions, so the second
  open is 1–2s total.
- Optional `krit.indexOnInitialize: false` setting for users who
  want lazy indexing instead, at the cost of slow first queries.

### Multi-root workspaces

VS Code supports multiple workspace folders. Each gets analyzed
independently (their file sets don't overlap by definition). Index is
unified — one `*oracle.Index` holds all folders. File paths are
absolute, so collisions aren't an issue.

## Files to touch

- `internal/lsp/server.go` — `handleInitialize` spawns background
  indexer goroutine before returning
- `internal/lsp/indexer.go` — new, orchestrates the full-workspace
  index build with progress events
- `internal/lsp/progress.go` — new, wraps `$/progress` notification
  protocol
- `internal/oracle/index.go` — `BuildIndexIncremental(files []string,
  onProgress func(done, total int))` variant
- `internal/lsp/config.go` — add `indexOnInitialize bool` (default
  true) setting

## Testing

- Small workspace (5 files): index completes during `initialize`, no
  progress notifications needed (too fast).
- Medium workspace (~500 files, warm cache): progress events fire,
  indexed in ~1s.
- Large workspace (~18k files, cold cache): progress events fire,
  indexed in minutes, user sees continuous feedback.
- Client without `$/progress` support: begin/end messages only via
  `showMessage`.
- Query arrives mid-index: correctly waits up to 500ms, then falls
  back to textual walker; subsequent queries (after index completes)
  use oracle path.
- Multi-root workspace: both roots indexed, symbols from each are
  discoverable via goto-def.

## Risks

- **Cold-start on huge repos is still slow.** We can't make the JVM
  analyze 18k files in under a minute. Make sure the progress UX
  makes the wait feel bounded, not broken.
- **Memory pressure during index build**. Assembling the full oracle
  for kotlin/kotlin keeps ~1GB of analysis data in the daemon plus
  equivalent in the Go index. Document RAM requirements; don't
  auto-index if the workspace is above a threshold unless the user
  opts in.
- **Canceled initialize**. If the user closes the window during
  indexing, the goroutine must cancel cleanly and free its FIR
  allocations. Standard context cancellation; test it.

## Blocking

- Milestone 6 (jar-source) benefits from but doesn't require this.

## Blocked by

- [`fqn-symbol-index.md`](fqn-symbol-index.md) (milestone 1) —
  building what this milestone constructs
- [`navigation-handlers-rewrite.md`](navigation-handlers-rewrite.md)
  (milestone 3) — handlers that use what this builds
- [`didchange-oracle-refresh.md`](didchange-oracle-refresh.md)
  (milestone 4) strongly recommended — the long-lived daemon makes
  batched cache-miss analysis efficient. Without it, each cache-miss
  batch re-spawns `krit-types`.

## Links

- Parent cluster: [`lsp/README.md`](README.md)
- Related: oracle cache optimization work (2026-04-14/15 session;
  Signal warm at 0.56s, kotlin target 1–2s), which makes the warm-path
  version of this milestone feasible
