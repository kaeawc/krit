# InitTuiSplit

**Cluster:** [core-infra](README.md) · **Status:** shipped ·
**Severity:** n/a (infra) · **Default:** n/a

## What it does

Splits `cmd/krit/init.go` (currently 2,104 lines) into three focused
files: the bubbletea TUI model, the headless onboarding path, and the
shared orchestration logic they both call.

## Current cost

`init.go` interleaves three concerns:

1. **Bubbletea TUI state machine** — `tea.Model` implementation with
   `Init()`, `Update()`, `View()`, screen transitions, key handling,
   and rendering.
2. **Headless onboarding path** — the `--no-tui` / CI-friendly path
   that runs the same setup steps without interactive prompts.
3. **Orchestration logic** — config detection, project scanning,
   rule selection, baseline creation, and krit.yml generation, shared
   by both paths.

The TUI model's `Update()` function contains business logic (project
scanning, config writing) that should live in the orchestration layer.
This makes it hard to test the TUI independently of the setup steps,
and hard to test the setup steps without the TUI.

Additional signals:
- Line 2104: `var _ = sort.Strings` — unused import anchor suggesting
  the `sort` package may no longer be needed after refactoring.

Relevant files:
- `cmd/krit/init.go` — 2,104 lines

## Proposed design

Three files:

```
cmd/krit/init.go          — public entry point: InitCmd() dispatches
                            to TUI or headless based on flags (~50 lines)
cmd/krit/init_tui.go      — bubbletea tea.Model + Init/Update/View
                            (~600-800 lines, UI only)
cmd/krit/init_headless.go — headless path, calls orchestration directly
                            (~100-200 lines)
cmd/krit/init_steps.go    — shared orchestration: DetectProject(),
                            SelectRules(), CreateBaseline(),
                            WriteConfig() (~400-600 lines)
```

The TUI model's `Update()` calls orchestration functions and stores
results in model state. It no longer contains scanning or config logic
directly.

## Migration path

1. Extract orchestration functions from `init.go` into
   `init_steps.go`. Each function is a pure function from inputs to
   outputs (no TUI dependency).
2. Extract the bubbletea model into `init_tui.go`. The model calls
   `init_steps` functions.
3. Extract headless path into `init_headless.go`.
4. Reduce `init.go` to a dispatcher that picks TUI vs headless.
5. Remove the `var _ = sort.Strings` anchor if `sort` is no longer
   needed.
6. Add unit tests for `init_steps.go` functions in isolation.

## Acceptance criteria

- No single file exceeds 800 lines.
- `init_steps.go` functions have no dependency on `tea.Model` or
  `bubbletea`.
- `init_tui.go` has no dependency on `scanner`, `config`, or
  file-system operations — delegates all of that to `init_steps.go`.
- Existing `init_integration_test.go` passes without modification.
- New unit tests cover each orchestration step independently.

## Links

- Related: [`phase-pipeline.md`](phase-pipeline.md) (same structural
  debt pattern in `main.go`)
- Related: `cmd/krit/init_integration_test.go`
