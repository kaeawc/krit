# Prompt: Restart Krit FP Hunt on Signal-Android

Paste this into a fresh Claude Code session (in `/Users/jason/kaeawc/krit`) to resume the false-positive reduction work with a clean context window.

---

You are helping me drive down false positives in krit, a Go-based Kotlin static analyzer at `/Users/jason/kaeawc/krit`, by running it against Signal-Android at `/Users/jason/github/Signal-Android` and fixing rule logic until the FP rate is minimal.

## Prior session results
- Signal-Android: **15,807 → 5,589 findings** (64.65% reduction, 10,218 FPs eliminated over ~174 rounds)
- Full session summary in `scratch/fp-hunt-session-summary.md` — READ IT FIRST to avoid re-doing finished work.

## How to run krit
```bash
# Standard invocation (takes ~40s on Signal post-regression — see below)
cd /Users/jason/github/Signal-Android && /Users/jason/kaeawc/krit/krit -clear-cache 2>&1 >/dev/null
/Users/jason/kaeawc/krit/krit -f json -o /tmp/krit_rN.json .

# Summarize top rules
python3 -c "
import json
from collections import Counter
d = json.load(open('/tmp/krit_rN.json'))
c = Counter()
for f in d.get('findings', []): c[f['rule']] += 1
total = sum(c.values())
print(f'Total: {total}  Elim: {15807 - total}  Reduction: {(15807-total)/15807*100:.2f}%')
for r, n in c.most_common(15):
    print(f'  {n:6d} {r}')
"
```

## Critical context
1. **Line attribution is accurate.** Previous agents repeatedly claimed line drift on UnsafeCast / UseOrEmpty / UnsafeCall. I verified: 0/30 mismatches on multiple samples. They were working with stale Signal checkouts. **ALWAYS verify by reading the actual file at the reported line before classifying**.
2. **Signal-Android has NO detekt/krit config** (only a 5-entry `app/lint-baseline.xml` for lint 3.3.2). All runs are unconfigured defaults.
3. **Tests MUST pass** after each change: `go test ./internal/rules/ -count=1`. Full suite: `go test ./... -count=1`.
4. **Performance regression**: Signal scan currently takes ~40s (was 1.4s). Introduced by auto-commits `7611c16..e8da1d6` (typeinfer optimization + declaration summary caching). Each iteration is slow — account for it.
5. **Auto-commits**: A background process occasionally auto-commits changes. Some of my session's fixes have been committed, others reverted. Check `git log --oneline -15` and `git status` at start.
6. **Do NOT deactivate rules** that have positive/negative fixture tests under `tests/fixtures/` — fixture tests fail if the rule stops firing. Always add scoped exemptions instead.

## The loop (per round)

1. **Measure** — Run krit, summarize top rules.
2. **Spawn one subagent** with the `general-purpose` agent type. Give it a narrow angle:
   - Sample N findings from rule X. Read source. Classify TP/FP.
   - Or: sample from file cluster Y. Identify local pattern.
   - Or: compare detekt output for rule Z.
3. **Agent returns** with either "floor reached" or a targeted fix suggestion (rule file + pattern + estimated FP count).
4. **Apply the fix** — Edit the rule file. Prefer narrow context skips over threshold changes. Never deactivate rules with positive fixtures.
5. **Build + test** — `go build -o krit ./cmd/krit/ && go test ./internal/rules/ -count=1`. If tests fail, either fix the fix or revert.
6. **Re-measure** — `krit -clear-cache` then rerun on Signal. Verify delta.
7. **Loop.**

## Directives from me
- **Keep going without stopping**, one fix at a time, until I tell you to stop.
- After each fix, immediately look for the next FP.
- Do NOT get stuck philosophizing about "diminishing returns" or "the floor" — just try one more angle. The rule surface is large; fresh angles keep finding 1-5 FPs per round.
- Commit nothing unless I explicitly ask.
- If you find a genuine false-NEGATIVE (rule missing real bugs), mention it but don't stop the FP loop.

## Fresh angles to try (in priority order)
If you run out of ideas after 3 rounds, try these:

1. **Run detekt on Signal** — `detekt --input /Users/jason/github/Signal-Android/app/src/main/java --report txt:/tmp/detekt.txt`. Compare rule counts. Rules where krit has 0 but detekt has many are broken naming rules or similar bugs. Rules where krit >> detekt indicate over-firing.
2. **File clusters** — `python3 -c "from collections import defaultdict; import json; d = json.load(open('/tmp/krit_rN.json')); c = defaultdict(int); [c.__setitem__((f['file'].split('/')[-1], f['rule']), c.get((f['file'].split('/')[-1], f['rule']), 0) + 1) for f in d['findings']]; [print(f'{v:3d} {k[1]:30s} {k[0]}') for k,v in sorted(c.items(), key=lambda x:-x[1])[:20]]"`
3. **Annotation exemptions** — check `@Composable`, `@Preview`, `@SignalPreview`, `@Test`, `@ParameterizedTest`, `@Parcelize`, `@Serializable`, `@Entity`, `@JvmStatic`, `@VisibleForTesting`, `@WorkerThread`, `@MainThread`.
4. **Compiler warning alias `@Suppress("unused")`** — Kotlin compiler uses lowercase inspection IDs; krit should map them to rule names.
5. **Proto-file heuristics** — files importing `com.squareup.wire` / `com.google.protobuf` / Signal's `.databaseprotos.` should be treated specially for `!!` patterns.
6. **Android framework nullable properties** — `RecyclerView.adapter`, `DialogFragment.dialog`, `View.parent`, `View.rootView`, `View.tag`, `View.background`, `View.contentDescription` are all `@Nullable` in Java but are often `!!`-ed.
7. **Branch-nullable initializers** — `val x = if (cond) Foo() else null` → a conservative type resolver widens this to non-null incorrectly.
8. **Nested call walkers** — rules that walk up parent chains looking for a specific `call_expression` but return `false` at the first non-match instead of continuing outward. Search `isInside*` helpers for `return false` inside a `call_expression` branch.
9. **AST misparse edge cases** — `fun interface X { ... }` is parsed as `function_declaration`. `class X : Y by z { ... }` misparses `z { ... }` as a trailing-lambda call. Tree-sitter-kotlin has many quirks.

## Known at-the-floor rules (don't re-sample without new angle)
These have all been deeply sampled and confirmed mostly-TP:
- `MaxLineLength` (4266 remaining — all TPs, convention mismatch)
- `UnsafeCallOnNullableType` (262 — most remaining are real `!!`)
- `MagicNumber` (198 — ~20 context skips already applied)
- `UnsafeCast` (124 — extensive predicate/is-check handling already)
- `UseOrEmpty` (85 — all TPs, trivially auto-fixable)
- `LongMethod` (81 — all real after DSL/Table.kt/test/override skips)
- `LongParameterList` (58 — value-holder/callback-param/ViewModel skips applied)
- `CyclomaticComplexMethod` (57 — IgnoreSimpleWhen/SingleWhen + pure boolean applied)
- `TooManyFunctions` (38 — Fragment/ViewModel/Table suffix skips applied)
- `ReturnCount` (36 — guard clause collection covers preamble + initializer guards)

**New angles on these rules are welcome, but be careful about adding FPs. Start with rules you haven't touched.**

## Start command
```
Continue the krit FP hunt loop on Signal-Android from where the previous session left off. Read scratch/fp-hunt-session-summary.md for context, then measure current state and begin the loop. Do not stop looking for the next FP.
```
