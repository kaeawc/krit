# ErrorHandlingStandardization

**Cluster:** [core-infra](README.md) · **Status:** shipped ·
**Severity:** n/a (infra) · **Default:** n/a

## What it does

Establishes a consistent error handling strategy across all packages,
replacing the current mix of panics, silent recovery, log-and-continue,
and swallowed errors.

## Current cost

Error handling diverges across packages in ways that make debugging
harder and mask real issues:

### Panics in non-fatal paths

- `internal/output/json.go:155` — `panic(err)` on JSON marshal
  failure. JSON marshalling can fail for legitimate reasons (e.g.,
  unsupported types added to a finding struct). This should return an
  error, not crash the process.

### Silent panic recovery in dispatcher

- `internal/rules/dispatch.go:26-35` — `defer func() { recover() }`
  catches all rule panics, logs to stderr, and returns `nil`.
  - No structured tracking of which rules panic or how often.
  - No way to distinguish "rule has a bug" from "rule hit an edge
    case" in production.
  - The stderr output is easily lost in piped output.

### Swallowed errors in cache writes

- `cmd/krit/matrix_cache.go:260` — `_ = os.Rename(tmp, path)` after
  writing a temp file. If the atomic rename fails, the cache entry is
  silently lost.
- `cmd/krit/matrix_cache.go:194` — `_ = os.MkdirAll(fallback, 0o755)`
  in fallback cache path creation.
- `cmd/krit/baseline_audit.go:162` — `_ = filepath.Walk()` error
  swallowed.

### Debug-level logs without level gating

- `internal/lsp/server.go:73,285,291,297,449` — unconditional
  `log.Println()` for lifecycle events (EOF, initialized, shutdown,
  exit, config reload). These are useful during development but produce
  noise in production. LSP clients capture stderr, and these messages
  clutter diagnostics.

## Proposed design

### Error handling tiers

1. **Return errors at package boundaries.** Any exported function that
   can fail returns `error`. No panics in exported functions.
2. **Reserve panic for truly unrecoverable states.** Programmer errors
   (nil deref on a value that should never be nil) are fine as panics.
   Runtime errors (I/O, encoding) must be returned.
3. **Log-and-continue only in cleanup paths.** `_ = os.Remove(tmp)`
   in a defer is fine. `_ = os.Rename()` in a write path is not.
4. **Structured error tracking in dispatcher.** The panic recovery
   collects rule name, file path, and panic value into a
   `[]DispatchError` slice returned alongside findings. Callers decide
   whether to log, report, or surface these.
5. **Leveled logging in LSP/MCP.** Introduce a `--verbose` / `-v`
   flag for the LSP and MCP servers. Lifecycle messages gate behind
   verbose. Errors always log.

### Specific changes

| Location | Current | Target |
|----------|---------|--------|
| `output/json.go:155` | `panic(err)` | `return nil, fmt.Errorf("json marshal: %w", err)` |
| `dispatch.go:26-35` | `recover()` → stderr | `recover()` → append to `[]DispatchError` |
| `matrix_cache.go:260` | `_ = os.Rename()` | `if err := os.Rename(); err != nil { log.Printf(); return err }` |
| `matrix_cache.go:194` | `_ = os.MkdirAll()` | `if err := os.MkdirAll(); err != nil { return "", err }` |
| `baseline_audit.go:162` | `_ = filepath.Walk()` | `if err := filepath.Walk(); err != nil { return err }` |
| `lsp/server.go` 5 log calls | unconditional `log.Println` | gate behind `s.verbose` bool |
| `mcp/server.go` log calls | same | same |

## Migration path

1. Fix the JSON formatter panic — change signature to return
   `([]byte, error)`, update callers.
2. Add `DispatchError` type and collect panics in dispatcher. Surface
   in `--perf` output and `--verbose` stderr.
3. Fix the three swallowed errors in `matrix_cache.go` and
   `baseline_audit.go`.
4. Add `--verbose` flag to LSP and MCP server binaries. Gate lifecycle
   logs behind it.
5. Audit remaining `_ = ` assignments in `cmd/krit/` for any that
   mask real errors (vs legitimate cleanup ignores).

## Acceptance criteria

- Zero `panic()` calls in `internal/output/`.
- Dispatcher returns `[]DispatchError` alongside findings; callers
  have access to structured error data.
- No `_ = os.Rename()` or `_ = os.MkdirAll()` in non-cleanup paths.
- LSP and MCP servers produce no output on stderr unless `--verbose`
  is set or an actual error occurs.
- `go vet ./...` clean. All existing tests pass.

## Links

- Related: [`phase-pipeline.md`](phase-pipeline.md) (pipeline phases
  should propagate errors cleanly)
- Related: [`cache-unification.md`](cache-unification.md) (cache
  layer error handling)
