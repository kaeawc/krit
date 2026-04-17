# TUI Architecture Refactor Plan

status: planned

## Problem

`cmd/krit/init.go` is a 2000+ line file with an 82-field monolithic `initModel` struct. All 11 TUI phases share a single model, meaning 10 phases' worth of fields are dead weight at any time. Helper methods use a mix of value and pointer receivers, which is confusing and fragile. The `startThresholds()` method does synchronous file I/O.

## Goals

1. Extract per-phase sub-models implementing `tea.Model`
2. Root model delegates Update/View to the active sub-model
3. Eliminate mixed value/pointer receiver pattern
4. Convert remaining sync I/O (`startThresholds` YAML read) to async Cmd
5. Split the file into logical units

## Phase 1: File splitting (no behavior change)

Split `init.go` into files by concern. Each file stays in `package main`:

| File | Contents |
|---|---|
| `init.go` | CLI entry point (`runInitSubcommand`, `runHeadlessInit`), binary resolution |
| `init_model.go` | `initModel` struct, `newInitModel`, `Init`, `Update`, `View`, message types |
| `init_phases.go` | Per-phase update/view methods (picker, questionnaire, explorer, thresholds, autofix, baseline, done) |
| `init_commands.go` | All `tea.Cmd` factories (scan, write, autofix, baseline, fixture loading) |
| `highlight.go` | Already done — LCS, syntax highlighting, fixture rendering |

## Phase 2: Extract sub-models

Extract each phase into its own struct implementing a sub-model interface:

```go
// phaseModel is implemented by each phase's sub-model.
type phaseModel interface {
    Update(msg tea.Msg) (phaseModel, tea.Cmd)
    View() string
}
```

### Sub-models to extract:

1. **`pickerModel`** — profile selection
   - Fields: `profiles []string`, `pickerIdx int`
   - On enter: returns a `profileSelectedMsg{name}` the root handles

2. **`questionnaireModel`** — the question flow
   - Fields: `registry`, `fixtureCache`, `fixtureViewport`, `visibleQs`, `qIdx`, `qCursor`, `answers`, `cascaded`, `liveTotal`, `scans`
   - On completion: returns answers + threshold overrides to root

3. **`thresholdsModel`** — threshold sliders
   - Fields: `thresholdValues`, `thresholdCursor`, `thresholdOverrides`, specs
   - Async: convert `startThresholds` YAML read to a `tea.Cmd`

4. **`explorerModel`** — rule browser
   - Fields: `ruleItems`, `ruleActive`, `explorerCursor`, `explorerOffset`, `explorerFixtureCache`, `fixtureViewport`
   - On commit: returns overrides to root

5. **`scanningModel`** — progress display during scans
   - Fields: scan timing, progress bar, strict stage tracking
   - On complete: returns scan results to root

6. **`confirmModel`** — reusable yes/no confirmation (autofix + baseline)
   - Fields: `cursor int`, `title string`, `description string`
   - On answer: returns `confirmMsg{value bool}`

### Root model becomes:

```go
type initModel struct {
    opts     onboarding.ScanOptions
    registry *onboarding.Registry
    target   string
    
    // Shared state carried across phases.
    selected string
    scans    map[string]*onboarding.ScanResult
    answers  []onboarding.Answer
    
    // Active phase.
    phase    phaseModel
    
    width, height int
}
```

Update delegates to `m.phase.Update(msg)` and View delegates to `m.phase.View()`. Phase transitions create a new sub-model and assign it to `m.phase`.

## Phase 3: Fix receiver consistency

All sub-model methods use value receivers. State mutations happen by returning new model values from Update, not via pointer receivers.

The `applyAnswer` logic (currently a pointer-receiver method with side effects) becomes a pure function:

```go
func applyAnswer(m questionnaireModel, q *Question, value bool) questionnaireModel {
    // returns new model with updated answers and liveTotal
}
```

## Phase 4: Async threshold loading

Convert `startThresholds()` to:
1. Return a `tea.Cmd` that reads the profile YAML
2. Handle `thresholdsLoadedMsg` in the root Update
3. Create `thresholdsModel` with the loaded values

## Migration strategy

Each phase can be extracted independently. Order:
1. `confirmModel` (simplest, reusable for autofix + baseline)
2. `pickerModel` (small, no shared state)
3. `scanningModel` (isolated, only produces scan results)
4. `thresholdsModel` (medium, includes async YAML load fix)
5. `explorerModel` (medium, has fixture loading)
6. `questionnaireModel` (largest, has viewport + fixture cache)

Each extraction is a single commit that can be reviewed and tested independently. The root model shrinks by ~10 fields per extraction.

## Testing

Each sub-model gets its own `*_test.go` file with unit tests that drive Update/View directly without the root model. Integration tests continue to work through the root model's delegation.
