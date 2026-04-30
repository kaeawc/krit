# Android Lint cluster

This cluster tracks krit's coverage of AOSP Android Lint rules — the
built-in checks that ship with `com.android.tools.lint`. It is the
canonical reference for what has shipped and what
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
| source | v2 source dispatch | 77 | 77 | 0 | 0 | Source rules have concrete callbacks |
| manifest | ManifestRule | 38 | 35 | 0 | 3 | 3 need XML parser improvements |
| resource | ResourceRule | 66 | 35 | 0 | 31 | 31 need XML layout parser |
| gradle | GradleRule | 16 | 13 | 0 | 3 | 3 need version DB or network |
| icon | IconRule | 13 | 13 | 0 | 0 | All shipped incl. autofix |
| cross-file | multi-file | 9 | 0 | 0 | 9 | Needs cross-file resource infra |
| binary | class-file | 1 | 0 | 0 | 1 | Needs bytecode reader |
| proguard | proguard | 2 | 0 | 0 | 2 | Very niche |
| other | misc | 3 | 0 | 0 | 3 | Encoding/properties checks |
| **Total** | | **225** | **173** | **0** | **52** | |

## Fixture coverage

77 source rules are fully shipped, but only 21 of them have `.kt` fixture
files in `tests/fixtures/`. The remaining **43 source rules** rely on
inline Go unit tests only. Fixture files are the preferred form: they
exercise the full scanner pipeline (parse → dispatch → format) and catch
regressions that unit tests miss.

See [`fixture-gaps.md`](fixture-gaps.md) for the prioritised list of the
43 rules that still need fixture files, grouped by risk level.

## Open work (priority order)

1. **Add `.kt` fixture files for 43 source rules** (see
   [`fixture-gaps.md`](fixture-gaps.md)) — P0 security rules first.
2. **Implement 13 planned source rules** (from the gap analysis in item
   22) — all are pattern-based and need no new infra.
3. **31 resource rules** blocked on XML layout parser expansion.
4. **3 manifest rules** blocked on XML parser improvements.
5. **3 gradle rules** blocked on version-catalog DB or network access.
6. **9 cross-file rules** need the cross-file resource reference index.
