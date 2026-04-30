# Roadmap status snapshot

**Last updated:** 2026-04-16

This file is a consolidated one-line-per-item view of every numbered
roadmap entry in `roadmap/*.md`. It exists so future audits can grep
here instead of re-deriving status from 68 individual files.

Legend:
- ✅ **shipped** — implementation lives in the tree and is exercised
- 🟡 **in-progress** — partially done with clear next steps
- ⏳ **planning** — scoped but not started
- ⏸️ **postponed** — blocked on external prerequisites
- ❌ **abandoned** — scope obsoleted by other work
- ❓ **needs triage** — I did not read this file deeply enough

## Items 01–28 (original core roadmap)

| # | Item | Status | Evidence |
|---|---|---|---|
| 01 | Gradle plugin | 🟡 in-progress | Phase 1 complete. Tracking → [`clusters/build-integration/gradle-plugin.md`](clusters/build-integration/gradle-plugin.md) |
| 02 | GitHub Action | ✅ shipped | `.github/actions/krit-action/` has `action.yml` + `problem-matcher.json` |
| 03 | Config schema | ✅ shipped | `schemas/krit-config.schema.json` exists; schema-generation pipeline lands in repo |
| 04 | Accuracy audit | ✅ shipped | all 5 P0 bugs fixed; measured precision 98.6% (per plan file) |
| 05 | Type resolver expansion | ✅ shipped | `internal/typeinfer/` + `internal/oracle/` mature; 40+ rules wired via `SetResolver`; largely superseded by items 16/18/20 |
| 06 | Missing tests | ✅ shipped | `internal/cache/`, `internal/fixer/`, `internal/output/`, `internal/perf/` all have `_test.go` (~37 tests) |
| 07 | Parallel type indexing | ✅ shipped | `internal/typeinfer/parallel.go` — two-phase parallel/merge pattern, `frozen` flag eliminated |
| 08 | Tree-sitter queries | ❌ abandoned | Approach proven non-viable: benchmarks showed compiled queries **472× slower** than `FindChild` (391,631 ns/op vs 830 ns/op) due to Go/C FFI overhead, C traversal, and `map[string]*sitter.Node` marshaling. Item 68's flat-tree migration achieved the same performance goal via `FlatFindChild`/`FlatHasAncestor` (O(1) parent lookup). All query infrastructure (`CompiledQuery`, `MustCompileQuery`, `Exec`, etc.) and 15 tests deleted 2026-04-16 |
| 09 | LSP server | ✅ shipped | `cmd/krit-lsp/` + `internal/lsp/` — diagnostics, code actions, incremental, config, editor support; 37 LSP tests |
| 10 | Detekt coverage gaps | 🟡 in-progress | ~75/94 shipped, ~85 achievable. Tracking → [`clusters/rule-quality/detekt-coverage-gaps.md`](clusters/rule-quality/detekt-coverage-gaps.md) |
| 11 | Module-aware dead code | ✅ shipped | `internal/module/` has full impl: accessor, depparse, discover, graph, permodule; concept doc stays as a design reference |
| 12 | style2 refactor | ✅ shipped | `style2.go` is gone; 10 `style_*.go` files carry the rules |
| 13 | Rule consistency | ✅ shipped | Rule execution is fully on the v2 registry and dispatcher. Registered rules must have executable v2 implementations, duplicate registrations are rejected by invariant tests, and local validation passes with `go build -o krit ./cmd/krit/ && go vet ./...` plus `go test ./... -count=1`. Tracking → [`clusters/core-infra/unified-rule-interface.md`](clusters/core-infra/unified-rule-interface.md) |
| 14 | Android-lint implementation coverage | ✅ shipped | AOSP Android Lint parity rules have concrete implementations across source, manifest, resource, Gradle, and icon pipelines. `ScrollViewCount` and `OnClick` now have source/resource-backed implementations; `DalvikOverride` was removed as obsolete (Dalvik gone since Android 5.0). Ongoing tracking → [`roadmap/clusters/android-lint/`](clusters/android-lint/README.md) |
| 15 | Fix quality | ✅ shipped | 2026-04-15: all phases complete. Phase 1 + 2 + 3a + 3b + Confidence done; `AvoidReferentialEqualityRule` byte-pinned to operator child; `UtilityClassWithPublicConstructorRule` gained a two-shape autofix; Phase 4 spot-check found `EmptyElseBlockRule` safe via empty-body guard and `SerialVersionUIDInSerializableClassRule` is find-only (no insertion bug) |
| 16 | Type resolver adoption | ✅ shipped | `frozen` flag eliminated via builder pattern; 40+ rules now have `SetResolver` |
| 17 | Depth over breadth | ✅ shipped | 2026-04-16: all phases complete. Phases 1-5 (P0 bugs, dedup, core-pkg unit tests, Confidence field), Phase 6 (`--list-rules` completeness transparency), Phase 7 (CI regression gate via `TestRegression_Baselines`) all shipped. 527/527 rules classified with `Confidence()`. 109/109 fixable rules have per-rule fixtures |
| 18 | Type inference heuristics | ⏳ planning | Tracking → [`clusters/performance-infra/type-inference-heuristics.md`](clusters/performance-infra/type-inference-heuristics.md) |
| 19 | Vibe cleanup round 2 | ✅ shipped | Actions 1–3 complete: 5 failing fixtures fixed (WrongThread, ExportedReceiver, GrantAllUris, FieldGetter, ExportedContentProvider), `DispatchBase`/`LineBase` embedded, AST conversions done |
| 20 | Kotlin type oracle | ✅ shipped | `tools/krit-types/` Gradle project + `internal/oracle/` with daemon, content-hash cache, poison markers, per-rule filter, composite resolver. 2026-04-14/15 optimization push measured against Signal-Android + kotlin/kotlin: Signal warm-0 **0.48s (94×)**, warm-1-edit **0.50s (46×)**; kotlin warm-0 **5.67s (455×)**, warm-1-edit **8.07s**, cold 43min → 5.68min. See `~/kaeawc/blog/draft/From 43 Minutes to Half a Second - Krit Oracle Performance Journey.md` and `~/kaeawc/blog/research/krit-perf-journey-timeline.md` |
| 21 | Daemon startup optimization | 🟡 in-progress | Phases 1-3 shipped. Phase 4 (Leyden AOT) blocked on JDK 25+. Tracking → [`clusters/performance-infra/daemon-leyden-aot.md`](clusters/performance-infra/daemon-leyden-aot.md) |
| 22 | Full parity plan | ✅ shipped | 537+ rules; 100% detekt core (227/227) + libraries (3/3) + Android Lint (181/181). Coverage gap tracking → [`roadmap/clusters/android-lint/`](clusters/android-lint/README.md) |
| 23 | Binary autofix for images | ✅ shipped | Phases 1–5 complete: `internal/android/icons.go` (animated GIF/PNG detection), `internal/fixer/binary.go` (all fix types), `ConvertToWebp` wiring, minSdk + reference scanning safety model |
| 24 | Android-lint fixture audit | 🟡 in-progress | Phases 1-3 COMPLETE; Phase 4 (fixtures for 43 source rules) moved to [`roadmap/clusters/android-lint/fixture-gaps.md`](clusters/android-lint/fixture-gaps.md) |
| 25 | Fixture completeness | 🟡 in-progress | Original gaps closed. Tracking → [`clusters/rule-quality/fixture-completeness.md`](clusters/rule-quality/fixture-completeness.md) and [`clusters/android-lint/fixture-gaps.md`](clusters/android-lint/fixture-gaps.md) |
| 26 | Binary signing and release | 🟡 in-progress | GPG, SBOM, SLSA, Homebrew, Scoop shipped. macOS/Windows signing blocked on certificates. Tracking → [`clusters/sdlc/release-signing.md`](clusters/sdlc/release-signing.md) |
| 27 | Performance algorithms | 🟡 in-progress | Ancestor set, DFS memo, packed keys shipped. Remaining → [`clusters/performance-infra/wadler-lindig-printer.md`](clusters/performance-infra/wadler-lindig-printer.md) and [`clusters/performance-infra/full-string-interning.md`](clusters/performance-infra/full-string-interning.md) |
| 28 | MCP server for AI agents | ✅ shipped | `internal/mcp/` has full server.go + prompts.go + protocol.go + resources.go |

## Items 29–42 (test-coverage tranche)

Consolidated into [`clusters/sdlc/testing-infra/test-coverage-backlog.md`](clusters/sdlc/testing-infra/test-coverage-backlog.md).
14 individual files superseded. Progress tracked incrementally —
2,540 test functions exist now (up from ~922 at audit time).

## Items 43–48 (miscellaneous)

| # | Item | Status |
|---|---|---|
| 43 | Replace local repo tests with fixtures | ⏳ planning — tracking → [`clusters/rule-quality/local-repo-test-migration.md`](clusters/rule-quality/local-repo-test-migration.md) |
| 44 | Parity claims vs heuristics | ✅ shipped — rule metadata exposes inferred precision class via `--list-rules -v` and the MCP catalog |
| 45 | Android rule FP hardening | ✅ shipped — `MissingPermission`, `Wakelock`, `ViewTag`, `LayoutInflation` moved to scope-local checks; `OldTargetApi`/`MinSdkTooLow`/`NewerVersionAvailable` made configurable |
| 46 | Kotlin multi-repo audit | ✅ shipped — 33-repo audit: 28,247 → 26,440 findings (−1,807); 9 correctness fixes landed |
| 47 | FP hunt infra session | ✅ shipped — 6-repo audit: 2,750 → 1,601 findings (−1,149); test-directory exclusions, `MagicNumber` debug skips, `naming.go` test-root widening all deployed |
| 48 | Post-optional-abstract audit | ✅ shipped — `UnsafeCallOnNullableType` reduced via KSP/compiler exemptions; test-root detection widened for multiplatform |

## Items 49–67 (rule-category clusters)

These 19 items are the rule-category roadmap entries, each paired
with a `roadmap/clusters/<category>/` directory that holds per-rule
concept files. None of them currently carries a top-level status
header. Their state is best read from the cluster directories directly.

| # | Cluster | Cluster dir | Rough state |
|---|---|---|---|
| 49 | security-rules-syntactic | `roadmap/clusters/security/` | some shipped, some planned |
| 50 | security-rules-call-shape | same | some shipped, some planned |
| 51 | security-rules-taint | same | planned |
| 52 | accessibility-rules | `roadmap/clusters/a11y/` | some shipped, some planned |
| 53 | i18n-l10n-rules | `roadmap/clusters/i18n/` | some shipped, some planned |
| 54 | compose-correctness-rules | `roadmap/clusters/compose/` | ✅ **19/19 shipped 2026-04-14** (see Track F) |
| 55 | di-hygiene-rules | `roadmap/clusters/di-hygiene/` | some shipped, some planned |
| 56 | concurrency-coroutines-rules | `roadmap/clusters/coroutines/` (likely) | some shipped, some planned |
| 57 | database-room-rules | `roadmap/clusters/database/` | some shipped, some planned |
| 58 | release-engineering-rules | `roadmap/clusters/release-engineering/` | some shipped, some planned |
| 59 | testing-quality-rules | `roadmap/clusters/testing-quality/` | some shipped, some planned |
| 60 | privacy-data-handling-rules | `roadmap/clusters/privacy/` | some shipped, some planned |
| 61 | observability-rules | `roadmap/clusters/observability/` | some shipped, some planned |
| 62 | resource-cost-rules | `roadmap/clusters/resource-cost/` | some shipped, some planned |
| 63 | supply-chain-hygiene-rules | `roadmap/clusters/supply-chain/` | some shipped, some planned |
| 64 | licensing-legal-rules | `roadmap/clusters/licensing/` | some shipped, some planned |
| 65 | performance-infra | `roadmap/clusters/performance-infra/` | some shipped, some planned |
| 66 | onboarding | `roadmap/clusters/onboarding/` | ✅ both phases shipped (gum prototype + bubbletea TUI behind `krit init`) |
| 67 | rule-quality | `roadmap/clusters/rule-quality/` | planning |

A per-cluster scoreboard lives at
[`roadmap/clusters/README.md`](clusters/README.md#scoreboard-generated-2026-04-14).
Last regenerated 2026-04-16. Major changes since the 2026-04-14 snapshot:
concurrency jumped from 1→20 shipped, testing-quality from 3→18 shipped,
privacy from 3→13 shipped. Onboarding TUI (`krit init` via bubbletea)
shipped but concept files not yet updated to reflect this.
Architecture and rule-quality clusters remain 0% shipped.

## Item 68 — flat-tree migration — ✅ **shipped 2026-04-14**

Track A (rule file tail) landed 2026-04-14. All production rules use
`FlatFindChild`/`FlatHasAncestor` with O(1) parent lookup via
`[]FlatNode` index-range scans. `internal/rules/` has 4 `*sitter.Node`
refs remaining (all in test resolver code, not reachable from live dispatch).
`internal/typeinfer/` has zero refs.
`internal/scanner/` retains ~63 refs (5 in public API, rest in internal
compat layer — legitimate long-term since tree-sitter parsing still
produces `*sitter.Node` before flattening). The migration obsoleted
item 08's compiled-query approach entirely. See
`roadmap/clusters/flat-tree-migration/README.md` and
`benchmarks/2026-04-14.md`.

## Postponed

Two items live under `roadmap/postponed/`:

- [`flat-field-names.md`](postponed/flat-field-names.md) — blocked
  on vendored Kotlin grammar `FIELD_COUNT = 0`; no current rule
  needs it.
- [`grpc-fir-server.md`](postponed/grpc-fir-server.md) — added
  2026-04-15 after the oracle optimization push. A gRPC FIR server
  collapses to the existing persistent daemon because Kotlin
  Analysis API is single-threaded at the app level
  ([KT-64167](https://youtrack.jetbrains.com/issue/KT-64167)).
  Revisit if the API documents a thread-safe multi-session
  contract, or if item 21 grows into a multi-client daemon.

## New cluster since last snapshot

`roadmap/clusters/build-integration/` was added with five
tool-mode concepts (`abi-hash`, `used-symbol-extraction`,
`cross-module-dead-code`, `symbol-impact-api`, `analysis-daemon`).
These expose krit's symbol-level knowledge to external build
tooling rather than emitting findings. Not yet rolled into the
cluster scoreboard above (which is frozen at the 2026-04-14
20-cluster pass).
