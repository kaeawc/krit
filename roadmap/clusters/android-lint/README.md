# Android Lint cluster

This cluster tracks krit's coverage of AOSP Android Lint rules — the
built-in checks that ship with `com.android.tools.lint`. It is the
canonical reference for what has shipped, what is stubbed, and what
remains planned across every sub-pipeline.

## Sub-clusters

- [`source/`](source/) — Kotlin/Java source rules (CheckLines / CheckNode)
- `manifest/` — AndroidManifest.xml rules (ManifestRule)
- `resource/` — XML layout/values resource rules (ResourceRule)
- `gradle/` — Gradle build-file rules (GradleRule)
- `icon/` — Launcher icon rules (IconRule)

## Scoreboard

| Sub-cluster | Pipeline | Total AOSP | ✅ Shipped | 🟡 In-progress | ⏳ Planned | Notes |
|---|---|---:|---:|---:|---:|---|
| source | CheckLines/CheckNode | 77 | 74 | 3 | 0 | 3 no-op stubs remain |
| manifest | ManifestRule | 38 | 35 | 0 | 3 | 3 need XML parser improvements |
| resource | ResourceRule | 66 | 35 | 0 | 31 | 31 need XML layout parser |
| gradle | GradleRule | 16 | 13 | 0 | 3 | 3 need version DB or network |
| icon | IconRule | 13 | 13 | 0 | 0 | All shipped incl. autofix |
| cross-file | multi-file | 9 | 0 | 0 | 9 | Needs cross-file resource infra |
| binary | class-file | 1 | 0 | 0 | 1 | Needs bytecode reader |
| proguard | proguard | 2 | 0 | 0 | 2 | Very niche |
| other | misc | 3 | 0 | 0 | 3 | Encoding/properties checks |
| **Total** | | **225** | **170** | **3** | **52** | |

## Fixture coverage

74 source rules are fully shipped, but only 21 of them have `.kt` fixture
files in `tests/fixtures/`. The remaining **43 source rules** rely on
inline Go unit tests only. Fixture files are the preferred form: they
exercise the full scanner pipeline (parse → dispatch → format) and catch
regressions that unit tests miss.

See [`fixture-gaps.md`](fixture-gaps.md) for the prioritised list of the
43 rules that still need fixture files, grouped by risk level.

## Open work (priority order)

1. **Fix 3 no-op stubs** (`ScrollViewCount`, `DalvikOverride`, `OnClick`)
   — these rules are registered but emit no findings.
2. **Add `.kt` fixture files for 43 source rules** (see
   [`fixture-gaps.md`](fixture-gaps.md)) — P0 security rules first.
3. **Implement 13 planned source rules** (from the gap analysis in item
   22) — all are pattern-based and need no new infra.
4. **31 resource rules** blocked on XML layout parser expansion.
5. **3 manifest rules** blocked on XML parser improvements.
6. **3 gradle rules** blocked on version-catalog DB or network access.
7. **9 cross-file rules** need the cross-file resource reference index.

## Historical references

The following numbered roadmap docs are now superseded by this cluster.
They remain as historical references only; **this cluster is canonical**.

- [`../../14-android-lint-stubs.md`](../../14-android-lint-stubs.md) —
  item 14: initial stub pass for 181 AOSP rules.
- [`../../22-full-parity-plan.md`](../../22-full-parity-plan.md) —
  item 22: gap analysis identifying 65 uncovered AOSP rules.
- [`../../24-android-lint-fixture-audit.md`](../../24-android-lint-fixture-audit.md) —
  item 24: fixture audit (Phases 1–3 done; Phase 4 superseded by
  [`fixture-gaps.md`](fixture-gaps.md)).
