# PhasePipeline

**Cluster:** [core-infra](README.md) · **Status:** planned ·
**Severity:** n/a (infra) · **Default:** n/a

## What it does

Refactors `cmd/krit/main.go` (currently 2,342 lines) into a sequence
of named, independently testable phases. The CLI, LSP server, and MCP
server all compose the same phase functions rather than each
re-implementing rule loading and dispatch.

## Current cost

`cmd/krit/main.go` interleaves: flag parsing, config loading, rule
setup, per-file dispatch, cross-file rule invocation, module-aware
rule invocation, Android project analysis, baseline management,
output formatting, and caching. There is no clear boundary between
phases.

Cross-file and module-aware rules are invoked via bespoke loops
*outside* the dispatcher (lines 1109–1192, 1253) rather than flowing
through `Dispatcher.Run()`. This means:
- Cross-file rules bypass the suppression index applied to per-file
  rules.
- The LSP server builds the dispatcher independently and omits cross-
  file rules entirely.
- The MCP server has a third copy of dispatcher setup logic with
  different config handling.

Any change to rule loading, cross-file analysis, or output must be
made in all three places and is rarely kept in sync.

Relevant files:
- `cmd/krit/main.go` — 2,342 lines
- `internal/lsp/server.go:initialize()` — duplicate dispatcher setup
- `internal/mcp/server.go:buildDispatcher()` — third dispatcher setup

## Proposed design

Six phases, each a pure function from input to output:

```
Parse    → ParseResult
Index    → IndexResult
Dispatch → DispatchResult
CrossFile→ CrossFileResult
Fixup    → FixupResult
Output   → (side effect: writes to writer)
```

```go
// internal/pipeline/pipeline.go

type ParseResult struct {
    Files []scanner.ParsedFile
}

type IndexResult struct {
    ParseResult
    CodeIndex   *scanner.CodeIndex
    ModuleIndex *module.Index
}

type DispatchResult struct {
    IndexResult
    Findings scanner.FindingColumns
}

// ... etc.

func Parse(cfg *config.Config, paths []string) (ParseResult, error)
func Index(pr ParseResult) (IndexResult, error)
func Dispatch(cfg *config.Config, ir IndexResult, rules []rules.Rule) (DispatchResult, error)
func CrossFile(cfg *config.Config, dr DispatchResult, rules []rules.Rule) (DispatchResult, error)
func Fixup(cfg *config.Config, dr DispatchResult) (FixupResult, error)
func Output(cfg *config.Config, fr FixupResult, w io.Writer) error
```

The CLI becomes:

```go
pr, _ := pipeline.Parse(cfg, paths)
ir, _ := pipeline.Index(pr)
dr, _ := pipeline.Dispatch(cfg, ir, activeRules)
dr, _ = pipeline.CrossFile(cfg, dr, crossFileRules)
fr, _ := pipeline.Fixup(cfg, dr)
pipeline.Output(cfg, fr, os.Stdout)
```

The LSP server calls the same phases on the open file's content on
every `textDocument/didChange`, caching the `IndexResult` across
edits. The MCP server calls the same phases from its tool handlers.

## Migration path

1. Extract each logical block from `main.go` into a function in
   `internal/pipeline/`, starting with the easiest (output, fixup).
2. Wire `cmd/krit/main.go` to call the extracted functions — no
   behaviour change at this step.
3. Update `internal/lsp/server.go` and `internal/mcp/server.go` to
   call the same pipeline functions.
4. Delete the duplicate dispatcher setup code from LSP and MCP.
5. Move cross-file rule execution into `pipeline.CrossFile()` so it
   flows through the same suppression path as per-file rules.
6. Shrink `cmd/krit/main.go` to flag parsing + phase composition only.

## Acceptance criteria

- `cmd/krit/main.go` ≤ 300 lines (flag parsing + phase calls).
- LSP and MCP use no rule-dispatch logic of their own.
- Cross-file rules are subject to the same suppression index as
  per-file rules.
- All three entry points produce identical findings for the same
  input (verified by a new integration test).
- `go test ./internal/pipeline/...` covers each phase in isolation.

## Vibe detector evidence (2026-04-16)

The vibe-detector audit independently flagged `cmd/krit/main.go` as the
highest-severity red flag in the codebase:

- **1,513-line `main()` function** (lines 39-1552) handling CLI
  parsing, config loading, cache management, oracle invocation,
  dispatcher setup, file scanning, analysis, formatting, baseline
  creation, diff filtering, and output.
- **5-8 level nested conditionals** at lines 750-798.
- **Verbose "what" comments** throughout lines 350-582 that restate
  code (`// Load YAML configuration`, `// Collect files`) — typical
  of code that grew incrementally without refactoring checkpoints.

This concept already covers the right solution. Priority: High.

## Links

- Depends on: [`unified-rule-interface.md`](unified-rule-interface.md)
  (phases accept `[]rules.Rule`, not the current mixed family slice)
- Unlocks: [`unified-file-model.md`](unified-file-model.md),
  [`suppression-middleware.md`](suppression-middleware.md)
- Related: `cmd/krit/main.go`, `internal/lsp/server.go`,
  `internal/mcp/server.go`
