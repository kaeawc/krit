# Onboarding cluster

Roadmap item 66 · STATUS line: [`roadmap/STATUS.md`](../../STATUS.md) · **Status:** ✅ fully shipped 2026-04-15

Both phases landed in one session. 16 concept docs checked off,
47 tests backing them, one real external repo (coil) validated
the gum prototype end-to-end.

## Phase 1: bash + gum prototype — ✅ shipped 2026-04-15

Artifacts:

- [`config/profiles/`](../../../config/profiles/) — 4 YAMLs (balanced, strict, relaxed, detekt-compat)
- [`config/onboarding/controversial-rules.json`](../../../config/onboarding/controversial-rules.json) — 16-question registry with cascade links
- [`scripts/krit-init.sh`](../../../scripts/krit-init.sh) — full 7-step gum flow
- [`cmd/krit/krit_init_integration_test.go`](../../../cmd/krit/krit_init_integration_test.go) — 2 Go integration tests (9 subtests: 8 playground × profiles + 1 greenfield)

Concepts:

- [x] [`profile-templates.md`](profile-templates.md)
- [x] [`controversial-rules-registry.md`](controversial-rules-registry.md)
- [x] [`cascade-map.md`](cascade-map.md) — encoded in registry as `cascade_from` / `cascades_to`
- [x] [`gum-profile-scan.md`](gum-profile-scan.md)
- [x] [`gum-comparison-table.md`](gum-comparison-table.md)
- [x] [`gum-profile-selection.md`](gum-profile-selection.md)
- [x] [`gum-controversial-rules.md`](gum-controversial-rules.md)
- [x] [`gum-write-config.md`](gum-write-config.md) — uses `yq eval-all` deep merge
- [x] [`gum-autofix-pass.md`](gum-autofix-pass.md)
- [x] [`gum-baseline.md`](gum-baseline.md)
- [x] [`gum-integration-test.md`](gum-integration-test.md) — automation side; external-repo validation still manual

Phase 2 unblocks only after criterion 4 of the integration-test doc is
met: "at least one external project adopts krit using the script and
provides feedback on the flow."

## Phase 2: Go TUI (bubbletea) — ✅ shipped 2026-04-15

All five concept docs are shipped. The bubbletea TUI lives behind
`krit init [target]` and reuses the Phase 1 registry and profile
files; all Phase 2 state is held in `cmd/krit/init.go:initModel`.

Artifacts:

- [`internal/onboarding/`](../../../internal/onboarding/) — shared Go package, no UI deps, 7 unit tests
- [`cmd/krit/init.go`](../../../cmd/krit/init.go) — bubbletea Model/View/Update + headless path
- [`cmd/krit/init_integration_test.go`](../../../cmd/krit/init_integration_test.go) — 5 subcommand integration tests (9+ playground subtests)
- [`cmd/krit/init_tui_test.go`](../../../cmd/krit/init_tui_test.go) — 34 in-process TUI component tests (cascade, key handlers, views, Cmd factories, env helpers)

Concepts:

- [x] [`tui-architecture.md`](tui-architecture.md) — Model/View/Update, pre-scan profiles, re-filter in memory
- [x] [`tui-realtime-finding-count.md`](tui-realtime-finding-count.md) — live count updates on every answer
- [x] [`tui-live-code-preview.md`](tui-live-code-preview.md) — split-pane fixture preview with positive/negative toggle
- [x] [`tui-threshold-sliders.md`](tui-threshold-sliders.md) — 10 numeric sliders for the main complexity + style thresholds
- [x] [`tui-split-pane-explorer.md`](tui-split-pane-explorer.md) — full rule browser (534 unique rules, deduped) reachable via 'b' from the picker

Autofix + baseline phases are inline in the TUI (not just
next-step hints), matching the gum script's behavior. The
headless `--yes` path also runs them so integration tests can
assert `.krit/baseline.xml` exists after every run.

Flow: `scan → picker → { questionnaire | explorer } → thresholds → write → autofix confirm → baseline confirm → done`.

Coverage: `init.go` ships at 81.6% average function coverage
across its 49 functions (see the test-coverage commit for the
full breakdown).
