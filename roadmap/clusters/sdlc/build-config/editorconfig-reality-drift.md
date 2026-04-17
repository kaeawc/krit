# EditorconfigRealityDrift

**Cluster:** [sdlc/build-config](README.md) · **Status:** planned · **Severity:** n/a (subcommand)

## Concept

Compare `.editorconfig` declarations (indent_size, charset, max_line_length)
against what the source actually uses. Report drift as a summary.

## Shape

```
$ krit editorconfig-drift
.editorconfig says indent_size=4; 1247 files use 2-space indentation.
```

Dispatch this as a real verb in [`cmd/krit/main.go`](/Users/jason/kaeawc/krit/cmd/krit/main.go),
using the same early-`os.Args` rewrite pattern that `baseline-audit` already
uses. The first landable shape should stay summary-only rather than emitting
per-line findings:

- `krit editorconfig-drift [paths...]` scans Kotlin sources under the provided paths.
- For each file, load the effective `.editorconfig` by calling `config.LoadEditorConfig(file.Path)` so nested module-local configs behave the same way as normal `--enable-editorconfig`.
- Print one summary row per property/value pair that drifts materially, for example `indent_size=4` vs observed 2-space leading indentation, `max_line_length=100` vs files exceeding 100 columns, and `charset=utf-8` vs files whose bytes are not valid UTF-8.

## Dispatch

This is not a rule-dispatch problem; the subcommand should run after file
collection/parsing and aggregate across parsed files. The concrete reuse seam is:

- `scanner.CollectKotlinFiles(...)` and `scanner.ScanFiles(...)` in [`internal/scanner/scanner.go`](/Users/jason/kaeawc/krit/internal/scanner/scanner.go) to reuse Krit's existing Kotlin/KTS file discovery, ignore rules, and parallel parsing.
- `config.LoadEditorConfig(...)` plus the `EditorConfig` fields in [`internal/config/editorconfig.go`](/Users/jason/kaeawc/krit/internal/config/editorconfig.go) to resolve effective `indent_size`, `indent_style`, `max_line_length`, `insert_final_newline`, and `trim_trailing_whitespace` exactly the same way the normal analysis path already does.
- `scanner.File.Lines` for line-based drift counters and `scanner.File.Content` for byte-level checks. That keeps indentation and line-length measurement aligned with the data `ParseFile(...)` already materializes in [`internal/scanner/scanner.go`](/Users/jason/kaeawc/krit/internal/scanner/scanner.go).

The cheapest implementation path is to group parsed files by the effective
`EditorConfig` values returned from `LoadEditorConfig(...)`, then compute:

- indentation histogram from leading whitespace in `file.Lines`
- max-line-length overflow counts from `len(line)` against `EditorConfig.MaxLineLength`
- charset drift from `file.Content` bytes
- final-newline / trailing-whitespace drift from the same line-oriented data the existing style rules consume

## Infra reuse

- `.editorconfig` loading already exists in [`internal/config/editorconfig.go`](/Users/jason/kaeawc/krit/internal/config/editorconfig.go): `LoadEditorConfig(...)`, `EditorConfig`, and `(*EditorConfig).ApplyToConfig(...)`. The subcommand should reuse that parser/merge behavior instead of inventing a second interpretation of `.editorconfig`.
- Existing source enumeration already exists in [`internal/scanner/scanner.go`](/Users/jason/kaeawc/krit/internal/scanner/scanner.go): `CollectKotlinFiles(...)`, `ParseFile(...)`, and `ScanFiles(...)`.
- Existing style semantics already exist in [`internal/rules/style_format.go`](/Users/jason/kaeawc/krit/internal/rules/style_format.go). `TrailingWhitespaceRule.CheckLines(...)`, `NoTabsRule.CheckLines(...)`, `MaxLineLengthRule.CheckLines(...)`, and `NewLineAtEndOfFileRule.CheckLines(...)` define the current notion of trailing whitespace, tab indentation, line-length overflow, and missing final newlines. `editorconfig-drift` should summarize against those same notions instead of creating subtly different thresholds.

## Links

- Parent: [`../README.md`](../README.md)
