# Tiered FIR rules under incremental slices (#740): options measured

Branch `claude/krit-issues-iteration-79gicw-740-tiered-research`, based on main `b0b2f7eb` (#871 merged: `sliceCheck`, `IncrementalState`, `Snapshot`). Research only: no production code changes. All new code is test code in `tools/krit-fir/src/test/kotlin/dev/jasonpearson/krit/fir/incremental/Tiered*.kt`, plus a `tieredResearchTest` Gradle task that `test` excludes. Each number is labelled **measured** (this branch, this machine) or **inferred** (reasoned, not run).

## 0. Recommendation

**Do option D: rewrite the 16 tiered rules so that none of them needs another file's body. Then incremental runs with default configs and stays exact by construction, at zero per-edit cost.** The rewrite takes a different form per rule:
- **TooGenericExceptionThrown:** test the class's module instead of its source. This is exact, with no precision loss.
- **11 body-reading rules:** follow initializers and bodies only for declarations in the file under check. Other files contribute only what class files keep: `const` values, signatures and inferred types.
- **ShortAlarm, WorldWriteableFiles and JdbcStatementExecute:** read another file's declaration the way Go does.
- **SerialVersionUIDInSerializableClass:** give it its module-wide name index from the incremental state rather than from `firProvider`.

Keep the full-compile fallback (#872's policy) for any rule that stays tiered: plugin rules, or a rule we decide to keep cross-file.

Why D, and not A, B or C:

1. **D's precision cost, measured, is the cross-file recall that FIR adds beyond Go, and on real code it is zero.**
   - **Real code.** krit-fir's 131 files produce 0 tiered findings. AutoMobile's 630 files produce 22, and all 22 are file-local: 9 LongLogTag with a same-file `const`/literal tag, and 13 TooGenericExceptionThrown on `RuntimeException`, a default name that needs no source check.
   - **Goldens.** All 121 golden scenarios of the 16 rules are single-file, so none changes. The losses are 3 findings of `WeakKeySizeCrossFileTest` and, if SerialVersionUID is narrowed rather than given an index, 1 finding of `SerialVersionUIDInSerializableClassCrossFileTest`.
   - **Go.** Go misses every one of those lost findings too.
2. **A (lookup-closure expansion) is exact only at unbounded depth, and its slices approach a full compile.**
   - **Depth.** Rules chase chains as long as the code allows: transitive in 10 rules, measured to the end of every 2- and 3-file chain tried. Depth 1 and 2 left stale verdicts on 39 and 36 of 40 edits (a stale verdict stays until the file is refreshed). Depth >= the corpus's chain length left 0.
   - **Size.** The compiler's lookups follow every name a file resolves, not the 1-3 declarations a rule reads. On krit-fir a body edit compiled 50 files instead of 4 (p50; 2.1 s vs 0.7 s, against 4.3 s for a full compile). On AutoMobile, even gated to files where a tiered rule can fire, a body edit of a typical file needs **253 of 630 files** at full depth (p50; 19 at depth 1, which is not exact).
3. **B (A + per-file verdict cache + trigger gate) is exact and is cheap where no tiered rule can fire.** On krit-fir, with the gate scoped to `java.sql.*Statement`, it compiled exactly the plain slice on all 40 edits (2.3). It is not cheap on Android code: `Log.*` and `launch` are in 16% of AutoMobile's files, and their closures cover most of the module. Exact read tracking (strategy R below: record the declarations a rule actually followed) compiled only 1.8x the plain slice with 0 stale verdicts on the synthetic corpus. It is the right shape if cross-file recall must be kept. It costs instrumenting ~55 read sites in 14 rules, plus a read index and miss-driven waves in the planner.
4. **C (deferred refresh) is not exact, and it fights krit's contracts.**
   - **What is wrong, and for how long.** Tiered verdicts would be wrong for at least one full compile plus snapshot after the last edit: 8.7-10.3 s warm on krit-fir and ~25 s at 10,000 files (the #740 section 4.3 measurements). During continuous editing the window has no bound.
   - **Go cache.** The cache is keyed by the whole compilation, so it would store the stale verdicts until the next edit.
   - **Contracts.** `--strict-verify` would flag every window, and one-shot `krit` runs cannot wait for a refresh.

## 1. What was run

- **Harness: `TieredResearch`** (tag `tiered-research`; `./gradlew tieredResearchTest --tests ...TieredResearch -Pkrit.fir.tiered.mode=...`). It replays an edit sequence. After every edit, each strategy serves every file's verdict: freshly compiled files from its slice, the rest from its own per-file verdict cache. The harness compares each verdict, file by file, with a **fresh frontend-only full compile of the edited module**, split into the 16 tiered rules and all other rules (plus the compiler-error gate and rule errors).
  - **Production code used.** Every slice is the production `AnalysisSession.sliceCheck` against the production snapshot, and closures use the production `IncrementalState` lookups and declaration index.
  - **The plain slice.** It is a stand-in for the step-4 planner (#872 is not on main): changed files, then lookup dependents of changed surfaces plus subclass and same-name files, in waves of up to 8.
- **Harness: `TieredDepthProbeTest`.** For each rule, a chain of files: A uses the value, B (and C, D) forward it, and the last file holds the fact. The probe slices A with the first k chain files in source and compares A's verdict with a full compile. It records how many lookup hops each chain file is from A, and whether the lookup closure of A gives the right verdict. It then makes a body-only edit of the chain's last file and checks four things:
  - whether the full verdict changes;
  - whether the file's surface changes;
  - whether the step-4 dependents reach A;
  - whether the reverse lookup closure reaches A and the expanded slice is correct.
- **Strategies.**
  - `plain`: tiered rules simply run on the step-4 slice.
  - `Ak`: option A. Refresh T_k = the plain set plus every file whose depth-k lookup closure contains a changed file (body edits included). Compile T_k together with T_k's depth-k closure in one slice.
  - `Gk`: option B. As `Ak`, but only T_k files that pass the trigger gate are expanded and refreshed. A file passes when it holds a lookup of a tiered rule's entry point (e.g. `javax.crypto.spec#SecretKeySpec`, `android.util.Log#d`, `java.sql.Statement#execute`), or when its closure holds a file with a "seed" lookup (`java.util.Collections#synchronized*`, `kotlinx.coroutines.flow#MutableSharedFlow`) for the two rules whose evidence sits in the other file.
  - `R` (synthetic only): exact read tracking. It refreshes the files whose actual read set (the generator knows each user's chain) contains a changed file, with their read sets in source.
- **Corpora.**
  - **Synthetic.** The #740 spike's `Gen.kt`, copied to `TieredGen.kt` and extended. Every 10th lib file holds a `Keys` object whose key, formatter and SQL vals forward along chains of 1-3 files. Every 7th app file feeds them to `SecretKeySpec(Base64...decode(..))`, `withLocale(Locale.getDefault())` and `Statement.execute`. A "secret" edit (25% of edits) flips a chain end between a hardcoded and a runtime value: a body-only edit, since types are explicit. The other edits are the spike's mix. 600 files, 40 edits, seed 11.
  - **krit-fir.** krit-fir's own sources: 131 files, 0 errors, against `kotlin-compiler`, stdlib and annotations, with `-Xcontext-parameters`. Real edits from the spike's `Proj.kt` (`TieredRealProj.kt`): string literal, comment, rename of a function another file uses (toggled back later), and add/delete a file. 40 edits.
  - **AutoMobile.** A static run of `android/{protocol,desktop-domain,control-proxy,desktop-core,junit-runner,auto-mobile-sdk}/src/main/kotlin`: 630 files against the compiler-test Android stubs. One full recording compile, then closure and gate sizes per file. With 350 error files there is no snapshot, so no slices run. Lookups in error files are partial: an unresolved receiver records no member lookup, so these sizes are lower bounds.
- **Machine.** 4 vCPU, JDK 21, Kotlin 2.4.20. It was shared with the #872 lane: timing runs were serialized under `flock /tmp/claude-0/krit-bench.lock`, at 1-minute load 2.2-9. Times are p50 / p95 in one warm JVM. The reference full compile is frontend-only; the production fallback also runs the backend for the snapshot, which costs +70-100% (#740 section 4.3).

## 2. Options

### 2.1 Summary table

Correctness is stale verdicts against a fresh full compile: edits with any stale file verdict, out of 40, and stale tiered file-verdicts summed over edits. Non-tiered rules were never stale in any strategy (0 stale "other" verdicts in every row).

| option | correctness (measured) | extra cost per edit (measured; synthetic 600 / krit-fir 131 / AutoMobile 630 static) | implementation size (inferred) |
|---|---|---|---|
| today's safe policy (#872): full compile while any tiered rule is enabled | exact | the full compile every edit: 0.86 s / 4.3 s p50 frontend-only (+70-100% with the snapshot backend) / ~64 s on AutoMobile (one cold compile, errors, stubs) | none |
| tiered rules on the plain slice (no tiering) | **stale on 39/40 synthetic edits**, 1,277 stale tiered verdicts (max 56 per edit); 0/40 on krit-fir (no tiered findings there) | 0 | none |
| A1 / A2 (lookup closure, depth 1 / 2) | **stale on 39/40 and 36/40** edits (803 / 298) | +57 / +73 files p50 on synthetic | small |
| **A3 / A-inf** (depth >= the corpus's longest chain) | **exact: 0/40** on synthetic and krit-fir | synthetic: 111 files p50 (p95 273) vs plain 10; 237 ms vs 118 ms p50. krit-fir: 50 files vs 4; 2,138 ms vs 688 ms p50 (p95 3.5 s; one literal edit compiled the whole module, 131 files). AutoMobile: closure per file p50 4, p95 191, max 289; a body edit of a file must refresh p50 22 files, p95 66 | ~150-250 lines in the planner: closure over the lookup index, body-level invalidation (any content change), one more slice |
| **B = G-inf** (A-inf + verdict cache + trigger gate) | **exact: 0/40** on synthetic; 0 gate misses (no file with a tiered finding was ever outside the gate) | synthetic: 52 files p50 (p95 159), 202 ms p50 (p95 368). krit-fir: identical to plain on 40/40 edits once the gate is scoped (2.3). **AutoMobile: a body edit needs p50 253 files of 630 (p95 295)**; depth 1: p50 19, p95 132 | A + per-rule entry declaration (16 entries) or exact entry recording in each rule (1 line each) + the per-file tiered verdict cache: ~350 lines |
| R (B with exact read tracking) | **exact: 0/40** on synthetic | 18 files p50 (p95 56) vs plain 10; 160 ms vs 118 ms p50 (p95 228 vs 389). On secret edits: 25 files / 174 ms vs G-inf 123 / 273 and A-inf 248 / 447 | ~55 read sites in 14 rules through a `readOther(symbol)` helper that records (file -> declaring file) and reports reads that hit the snapshot (origin `Precompiled`) as misses; a read index and miss-driven waves in the planner: ~500-700 lines + tests |
| C (deferred refresh) | **not exact** by design (2.4); its wrong set is "plain" for the compiled files plus every carried-over verdict whose read declarations changed | per edit as plain, plus one background full compile with snapshot per quiet period (8.7-10.3 s krit-fir, #740 4.3) competing for CPU with the next slices | Kotlin: a background compile thread, merging its commit with slices that ran meanwhile; protocol: a `provisional` marker per (file, rule); Go: no caching of provisional entries, re-request, strict-verify exemption: ~600-900 lines across Kotlin and Go |
| **D** (rewrite the rules; 3) | **exact by construction**: a narrowed rule reads nothing a binary lacks, so its slice verdict equals its full verdict. The split differential and the depth probe become the regression tests | **0** | 14 rules x 2-9 read sites behind one same-file helper (or Go's reading) (~40 lines) + TooGeneric's module check (~10 lines) + SerialVersionUID's index (~200 lines, or ~20 to narrow) + updated cross-file tests: ~400 lines |

### 2.2 Option A: depth and invalidation (measured, `TieredDepthProbeTest`)

| rule | chain files after A | A's full verdict needs A + k chain files | per prefix (k = 0, 1, ...) | lookup hops of the chain files | slice of A + its lookup closure correct | body-only edit of the last file: full verdict changes / surface changed / step-4 dependents reach A / reverse lookup closure reaches A / expanded slice correct |
|---|---|---|---|---|---|---|
| HardcodedSecretKey | 3 | 3 | ≠ ≠ ≠ = | 1, 2, 3 | yes | yes / no / **no** / yes / yes |
| StaticIv | 2 | 2 | ≠ ≠ = | 1, 2 | yes | yes / no / **no** / yes / yes |
| WeakKeySize | 1 | 1 | ≠ = | 1 | yes | yes / no / **no** / yes / yes |
| LongLogTag | 2 | 2 | ≠ ≠ = | 1, 2 | yes | yes / no / **no** / yes / yes |
| LongLogTag, `const` tag with an escape (`"TagA..."`) in another file | 1 | 1 | **≠** = | 1 | yes | (no edit) |
| LocaleGetDefaultForFormatting | 2 | 2 | ≠ ≠ = | 1, 2 | yes | yes / no / **no** / yes / yes |
| JdbcStatementExecute | 2 | 2 | ≠ ≠ = | 1, 2 | yes | yes / no / **no** / yes / yes |
| CollectionsSynchronizedListIteration | 1 | 1 | ≠ = | 1 | yes | yes / no / **no** / yes / yes |
| SharedFlowWithoutReplay | 2 | 2 | ≠ ≠ = | 1, 2 | yes | yes / no / **no** / yes / yes |
| ShortAlarm | 2 | 2 | ≠ ≠ = | 1, 2 | yes | yes / no / **no** / yes / yes |
| WorldWriteableFiles | 2 | 2 | ≠ ≠ = | 1, 2 | yes | yes / no / **no** / yes / yes |
| SecureRandom | 1 | 1 | ≠ = | 1 | yes | yes / no / **no** / yes / yes |
| ShowToast | 1 | 1 | ≠ = | 1 | yes | yes / no / **no** / yes / yes |
| GetSignatures | 2 | 2 | ≠ ≠ = | 1, 2 | yes | yes / no / **no** / yes / yes |
| GlobalScopeLaunchInViewModel | 2 | 2 | ≠ ≠ = | 1, 2 | yes | yes / no / **no** / yes / yes |
| TooGenericExceptionThrown (custom `exceptionNames`) | 1 | 1 | ≠ = | 1 | yes | (no edit) |
| SerialVersionUIDInSerializableClass | 1 | 1 | ≠ = | **unreachable** | **no** | yes / yes / **no** / **no** / yes |

What this shows:
- **Depth.** Every transitive rule followed the chain to its end: no rule stops at one hop, so A needs the depth of the longest chain in the module. The code bounds are 8 (GlobalScope), 16 (Jdbc), 64 (HardcodedSecretKey, Locale) or none (StaticIv, LongLogTag, GetSignatures, SharedFlow, ShortAlarm, WorldWriteable): see section 3. In practice that means the transitive closure (A-inf).
- **Lookups.** The compiler's lookups put each chain file at exactly its chain position, so the lookup closure is a sound over-approximation of what the 15 body-reading rules read: the "closure correct" column is yes for all 15.
- **Invalidation.** A body-only edit of the last file changes A's full verdict, but no surface. The step-4 dependents never reach A. The reverse lookup closure always does, and the expanded slice is then correct. Invalidation for tiered rules therefore has to be by content, not surface, and transitive.
- **SerialVersionUIDInSerializableClass is outside A.** It reads class names across the module by Go's name resolution, which no lookup records: B is at no hop from A.
- **`const` is not entirely safe for LongLogTag.** A `const` tag from a binary still differs (measured) when its literal has an escape sequence: the message spells the literal's source text, and a binary literal has no source, so the rule prints the decoded value (cause **inferred** from `LongLogTag.spelling`). D must spell the value the same way for both, for example always the decoded value for another file's constant.

### 2.3 A and B on the corpora (measured)

Synthetic, 602-605 files, 40 edits (13 "secret", 11 body, 6 add, 3 delete, 3 signature, 2 sealed, 1 overload, 1 comment). 65 app files held a tiered finding at some point. Reference full compile: p50 855 ms, p95 1,545 ms.

| strategy | edits with any stale verdict | stale tiered (sum, max per edit) | stale other | files compiled p50 / p95 / max | extra over plain p50 / p95 / max | ms p50 / p95 |
|---|---|---|---|---|---|---|
| plain | 39/40 | 1,277 (56) | 0 | 10 / 38 / 38 | 0 | 118 / 389 |
| R (exact read tracking) | **0/40** | 0 | 0 | 18 / 56 / 62 | 8 / 20 / 24 | 160 / 228 |
| A1 | 39/40 | 803 (35) | 0 | 63 / 137 / 159 | 57 / 99 / 121 | 198 / 396 |
| A2 | 36/40 | 298 (16) | 0 | 98 / 205 / 232 | 73 / 177 / 194 | 245 / 459 |
| A3 | **0/40** | 0 | 0 | 111 / 250 / 267 | 87 / 221 / 238 | 237 / 527 |
| A-inf | **0/40** | 0 | 0 | 111 / 273 / 284 | 87 / 246 / 260 | 237 / 573 |
| G1 | 39/40 | 879 (37) | 0 | 31 / 75 / 79 | 15 / 41 / 41 | 161 / 260 |
| G2 | 36/40 | 316 (17) | 0 | 51 / 121 / 129 | 23 / 91 / 99 | 178 / 326 |
| G3 | **0/40** | 0 | 0 | 52 / 145 / 175 | 24 / 127 / 137 | 202 / 378 |
| G-inf | **0/40** | 0 | 0 | 52 / 159 / 175 | 24 / 127 / 137 | 202 / 368 |

Per edit kind, files compiled p50 (ms p50):

| kind | plain | R | G-inf | A-inf |
|---|---|---|---|---|
| secret (body-only, chain end) | 10 (112) | 25 (174) | 123 (273) | 248 (447) |
| body | 4 (91) | 4 (99) | 4 (116) | 21 (130) |
| add | 10 (118) | 10 (116) | 10 (122) | 30 (142) |
| signature | 18 (412) | 21 (174) | 42 (140) | 111 (234) |

krit-fir, 131-134 files, 40 real edits (22 literal, 9 rename, 4 comment, 4 add, 1 delete). It has no tiered findings, so correctness is vacuous there (0 stale in every strategy); the run measures cost. Reference full compile: p50 4,257 ms, p95 5,434 ms.

| strategy | files compiled p50 / p95 / max | ms p50 / p95 |
|---|---|---|
| plain | 4 / 17 / 17 | 688 / 1,514 |
| A1 | 25 / 48 / 72 | 1,749 / 3,040 |
| A-inf | 50 / 61 / 131 | 2,138 / 3,539 |
| G-inf with the first gate (a name-only `execute` pattern for JdbcStatementExecute) | 39 / 50 / 50 | 1,974 / 3,966 |
| G-inf with the gate scoped to `java.sql.*Statement` (separate run, load 4.8-9; plain 811 / 2,030 ms there) | **4 / 17 / 17, identical to plain on every edit**: no krit-fir file passes the gate |

The first gate let in 2 krit-fir files that call an unrelated `execute`, and their closures pulled in ~35 files. That is why a production gate should be exact, recorded by the rule when its entry predicate matches, not matched by name.

AutoMobile, static, 630 files (350 with errors against the stubs), 22 tiered findings:

| depth k | lookup closure per file p50 / p95 / max | files whose depth-k closure contains a given file (reverse) p50 / p95 / max | gated files | gated files + their closures | gated expanded set for a body edit of one file, p50 / p95 / max (files with none) |
|---|---|---|---|---|---|
| 1 | 2 / 12 / 117 | 2 / 11 / 66 | 133 | 438 | 19 / 132 / 160 (192 of 630) |
| 2 | 4 / 52 / 232 | 9 / 36 / 101 | 159 | 537 | 190 / 262 / 265 (93) |
| 3 | 4 / 87 / 282 | 15 / 51 / 116 | 166 | 551 | 247 / 291 / 294 (79) |
| inf | 4 / 191 / 289 | 22 / 66 / 134 | 170 | 555 | **253 / 295 / 295** (75) |

Sink files by rule: LongLogTag 66, GlobalScopeLaunchInViewModel 49 (`kotlinx.coroutines#launch`), GetSignatures 5, CollectionsSynchronizedListIteration 2 (seed), SharedFlowWithoutReplay 9 (seed). A file's own closure is small (p50 4). But the gated files are the ones that log or launch, and they are the app's hubs: the union of their closures is 555 of 630 files, so most body edits reach a gated file whose whole closure must then be in source.

How often B stays small on real edits:
- **krit-fir.** With a scoped gate, every edit: G-inf compiled exactly the plain set on all 40 edits.
- **AutoMobile.** Only for the 75 of 630 files (12%) that no gated file reaches. Everywhere else B compiles ~40% of the module per body edit, inferred at ~40% of a full compile's time.

### 2.4 Option C: what changes, and what is wrong for how long

What it would be: a slice answers immediately. Tiered verdicts are taken from the slice for compiled files and carried over from the last full compile for the rest, and every (file, tiered rule) verdict in the response is marked provisional. A background full compile with the snapshot then replaces them.

What can be wrong, and for how long:
- **Compiled files.** Every tiered rule whose cross-file read hits a binary gives the slice verdict instead of the full one:
  - Missed findings: HardcodedSecretKey, StaticIv, WeakKeySize, LongLogTag (including the `const` escape case), CollectionsSynchronizedListIteration, SharedFlowWithoutReplay, SecureRandom, ShowToast, GetSignatures, GlobalScopeLaunchInViewModel and SerialVersionUIDInSerializableClass.
  - Extra findings: ShortAlarm, WorldWriteableFiles, JdbcStatementExecute and TooGenericExceptionThrown.
  - Both: LocaleGetDefaultForFormatting.
- **Other files.** A carried-over verdict is stale whenever a declaration it read changed, including by a body-only edit. On the synthetic corpus that was 10-56 files per edit for the plain strategy (measured, 2.3).
- **Duration.** From the edit's response until a response is served after the refresh finishes. That is at least one full compile with the snapshot backend after the last edit: 8.7-10.3 s warm on krit-fir, 5.9-6.6 s at 2,000 files and ~25 s at 10,000 files (the #740 section 4.3 measurements). While the user keeps editing, the window never closes unless refreshes are allowed to finish on stale content.

Fit with krit, from reading the code:
- **The krit-fir daemon is single-client and sequential** (`runDaemonStdio` / `runDaemonTcp`). A background compile needs its own thread and a second compile environment (heap). Its commit must merge with slices that ran meanwhile, because its recording describes older content. The snapshot lock (`AnalysisSession.snapshotLock`) would serialize slices behind its commit.
- **Go's FIR cache is keyed by the whole compilation** (`CheckCacheFingerprint`, `invoke_cached.go`). Provisional verdicts written under the new fingerprint stay until the next edit, so they must not be cached, or must be cached with a marker that forces a re-request.
- **Nothing pushes a refresh to the client.** `krit serve` answers analyze requests one at a time, and FIR diagnostics do not reach the LSP (only a preflight notice). A refreshed verdict is seen only by the next request.
- **`--strict-verify`** (`internal/cli/serve/strict_verify.go`) compares the daemon's response with an in-process baseline and would fail during every window unless it waits for the refresh.
- **One-shot `krit` runs** cannot wait, so they would always see provisional verdicts or pay the full compile anyway.

The contract would change from "a warm response equals a cold one" to "tiered findings may lag the source by one refresh". That is a weaker guarantee for exactly the security rules (crypto, JDBC) whose findings people act on.

## 3. The 16 rules: chase depth and option D

"Depth (code)" is read from the checker source: how many declarations of *other* files the rule can follow in one chain, and what bounds it. "Depth (measured)" is `TieredDepthProbeTest`: a chain A -> B -> C -> ... over files, A's verdict compared with a full compile when A plus the first k chain files are in source and the rest come from the snapshot. Every rule needed **the whole chain** it was given (k = chain length), so the measured value is a lower bound and the code bound is what a planner must assume. "Slice direction" is from the split differential and the probe: full-only = the rule loses a true positive on a slice, slice-only = it adds a finding on a slice.

"D" says whether the cross-file read can be dropped or narrowed to facts that class files keep (signatures and inferred types, `const` values, annotations other than SOURCE retention, the class's module). "Same-file" narrowing means: follow a declaration's body or initializer only when it is declared in the file under check (decided from the file's own FIR, not from the other declaration), so the verdict is identical whether the other file is source or binary. Its cost is the rule's cross-file recall, which Go never had: every Go counterpart works on one file's syntax, and the FIR rules list these reads as deliberate recall beyond Go.

| # | rule | kind | cross-file read | depth (code) | depth (measured) | slice direction | D: narrowing | D: precision cost (goldens / real code) |
|---|---|---|---|---|---|---|---|---|
| 1 | HardcodedSecretKey | bodies | `resolvedInitializer`, custom getter body, `by lazy {}` result of non-local non-const vals/vars, transitively (`val K1 = C.K2`) | transitive; recursion depth <= 64 (`MAX_DEPTH`, counts expression nesting too); memo per property, cycle = not hardcoded | 3 of 3 | full-only (FN) | same-file + `const` + Java constant fields (all kept in binaries) | 0 goldens lost (all 10 scenarios single-file). Lost: a key in a non-const `val` of another file (`object Keys { val K = "..." }`), FIR-only recall |
| 2 | StaticIv | bodies | `resolvedInitializer` of string vals used in templates and `kotlin.text` arguments, transitively. It never follows a byte-array val (`literalBytes` stops at a property), so the tier reason text ("an IV byte-array property") is inaccurate | transitive through template initializers; cycle-guarded, no bound | 2 of 2 | full-only | same-file + `const` | 0 goldens lost (12, single-file). Lost: a non-const string val of another file interpolated into an IV literal |
| 3 | WeakKeySize | bodies | the whole declaration of a non-local final generator `val` (`symbol.fir.accept(Writes)`) to find its `getInstance` algorithm. The key size is the call's literal text, so the tier reason text ("a top-level val key size") is inaccurate | 1 (algorithm from a literal or a `const`, both binary-safe) | 1 of 1 | full-only | same-file | **3 golden findings lost** (`WeakKeySizeCrossFileTest`: object member, top-level val, inherited protected member val, all in another file). Go misses all three too |
| 4 | LongLogTag | bodies | `resolvedInitializer` of non-const tag vals, transitively (`val TAG = LogTags.TAG`); a getter that returns `field` | transitive; `path` set breaks cycles, no bound | 2 of 2 | full-only | `const` (already binary-safe: `LongLogTagCrossFileTest` uses `const`) + same-file | 0 goldens lost. Lost: a non-const tag val chain across files. Real code: AutoMobile's 9 LongLogTag findings all use a same-file `const` or a literal (0 lost). Also needed: another file's `const` tag with an escape sequence is spelled differently from source and from a binary (measured, 2.2), so the message must use one spelling |
| 5 | LocaleGetDefaultForFormatting | bodies | final val initializers, single-expression bodies of final extension functions and custom getters | budget of 64 steps (chain steps + initializers + bodies) | 2 of 2 | both (FN and FP) | same-file | 0 goldens lost (6). Lost: a shared formatter holder in another file (`object Formats { val ISO = DateTimeFormatter.ISO_DATE }`): the most plausible real-world loss of the set |
| 6 | JdbcStatementExecute | bodies | final val initializers, to prove the SQL static, transitively | transitive, `MAX_DEPTH` 16 | 2 of 2 | slice-only (FP: a binary val is "computed") | same-file; a cross-file non-const val then counts as computed, which is Go's reading (Go treats any lower-case identifier operand as computed) | 0 goldens lost (7). Gained: FP on a non-const static SQL val of another file (const vals stay static) |
| 7 | CollectionsSynchronizedListIteration | bodies | initializer or `lazy` delegate of a val holding a `Collections.synchronized*` wrapper | 1 | 1 of 1 | full-only | same-file | 0 goldens lost (6). Lost: iterating a wrapper val declared in another file; Go cannot see it either |
| 8 | SharedFlowWithoutReplay | bodies | body of a project factory returning `MutableSharedFlow`, transitively through factories | transitive; `visiting` set, no bound | 2 of 2 | full-only | same-file (the return type is in the signature; "created without arguments" is not) | 0 goldens lost (5). Lost: a lossy factory helper in another file |
| 9 | ShortAlarm | bodies | body of a project wrapper named `setRepeating`/`setInexactRepeating`, transitively | transitive; `seen` set, no bound | 2 of 2 | slice-only (FP: no body = may set) | treat another file's project wrapper like a library function (reported), which is Go's reading (Go matches the name) | 0 goldens lost (6). Gained: FP for a cross-file project lookalike named `setRepeating` that ignores its third argument: practically never |
| 10 | WorldWriteableFiles | bodies | values of project variables named `MODE_WORLD_WRITEABLE`/`WRITABLE` (initializer, getter body, delegate), transitively | transitive; `seen` set, no bound | 2 of 2 | slice-only (FP) | another file's non-const project variable is Unknown, reported (Go reports the name) | 0 goldens lost (11). Gained: FP only for a project that declares its own non-const `MODE_WORLD_WRITEABLE = 0` in another file |
| 11 | SecureRandom | bodies | initializer of a `Random`-typed val holding a `SecureRandom` | 1 | 1 of 1 | full-only | same-file | 0 goldens lost (8). Lost: `val rng: Random = SecureRandom()` in another file, then `rng.setSeed(1)` |
| 12 | ShowToast | bodies | `onlyStore`: whether a non-local property assigned in this file has a non-null initializer in its (other-file) declaration | 1 | 1 of 1 | full-only | same-file (an other-file property is not followed) | 0 goldens lost (6). Lost: a toast stored into a property declared in another file and shown here |
| 13 | GetSignatures | bodies | initializers of non-const flag vals, transitively; a project val named `GET_SIGNATURES` | transitive; `seen` set, no bound (const values: depth 32, binary-safe) | 2 of 2 | full-only (and FP for a non-const lookalike) | same-file + `const` | 0 goldens lost (6). Lost: a non-const flags val in another file |
| 14 | GlobalScopeLaunchInViewModel | bodies | initializer of a final `CoroutineScope`-typed val, transitively | `MAX_DEPTH` 8 | 2 of 2 | full-only | the inferred type is surface: a val whose type is `GlobalScope` is caught without any read; follow initializers same-file only | 0 goldens lost (8). Lost: `val appScope: CoroutineScope = GlobalScope` with the wider declared type, in another file |
| 15 | TooGenericExceptionThrown | bodies (origin) | `symbol.source` of the thrown class: is it a Kotlin class of this compilation | 0: no body, only the declaration's origin | 1 of 1 | slice-only (FP, with a custom `exceptionNames`) | **exact, no loss**: test the module instead of the source (`symbol.moduleData == session.moduleData` and not Java). The snapshot's binaries carry the source module's own `FirModuleData` (findings 4.1) **inferred: not run** | none. AutoMobile's 13 findings use `RuntimeException` (a default name, no source check) |
| 16 | SerialVersionUIDInSerializableClass | compilation files | module-wide fixpoint over every source class's *written* supertype names (Go's name resolution), for exceptions that are Serializable only through Throwable | whole module, by names; **lookup closure does not reach it** (probe: B at hop -1, slice A + closure wrong) | 1 of 1 (needs the whole module) | full-only | (a) exact: the runner keeps a per-file index of (class simple name -> written supertype names), recorded like surfaces, and passes the reached set through `FirRuleContext`; re-run the rule on files with Throwable-only classes when the set changes; (b) narrow to resolved supertypes | (a) none. (b) **1 golden lost** (`SerialVersionUIDInSerializableClassCrossFileTest`: `FatalFailure : kotlin.Error` reported only because another file's `Resource.Error` is Serializable): Go-parity findings that are true but come from a name coincidence |

### 3.1 D, rule by rule, in short

- **Exact, no loss: TooGenericExceptionThrown.** Replace `symbol.source` with "declared in this module and not Java". The snapshot's binaries are read with the source module's own `FirModuleData` (#740 4.1), so `symbol.moduleData == session.moduleData` holds for a project class in both cases (**inferred**, not run: the rule was not changed on this branch).
- **Exact with a runner index: SerialVersionUIDInSerializableClass.** The planner records each file's (class simple name -> written supertype names), as it records surfaces, and passes the module's reached set through `FirRuleContext`. When the set changes, the rule is re-run on the files with Throwable-only classes.
- **Same-file + `const`, small loss, Go never had it: HardcodedSecretKey, StaticIv, WeakKeySize, LongLogTag, LocaleGetDefaultForFormatting, CollectionsSynchronizedListIteration, SharedFlowWithoutReplay, SecureRandom, ShowToast, GetSignatures and GlobalScopeLaunchInViewModel.**
  - **How.** A helper `declaredInFileUnderCheck(symbol)` decides from the current file's own FIR (its declarations, walked once per file), never from the other declaration.
  - **Tests to update.** `WeakKeySizeCrossFileTest` (3 findings) and the `differs = true` cross-file scenarios become controls.
  - **LongLogTag.** It also has to spell another file's `const` tag the same way from source and binary.
- **Go's reading for another file: ShortAlarm, WorldWriteableFiles and JdbcStatementExecute.**
  - **What changes.** A project wrapper or constant in another file is treated like a library one: reported for ShortAlarm, WorldWriteableFiles and JdbcStatementExecute (computed).
  - **Precision.** Each gains a theoretical false positive for cross-file lookalikes, which Go already reports.

After D, `wholeCompilationRuleIds()` is empty for the built-in rules (or holds SerialVersionUID if its index is deferred). The `FirRuleWholeCompilationGateTest` read gate stays, and keeps any new rule honest. The split differential then asserts that **no** rule differs on a slice.

### 3.2 If cross-file recall must be kept for a few rules

Use R, not A or B: record what each rule actually read, and invalidate by those declarations' content.
- **Why.** On the synthetic corpus R compiled 18 files p50 against A-inf's 111 and G-inf's 52, with the same 0 stale.
- **Better still.** Turn each read into a per-declaration *rule fact*, such as "this val is a hardcoded string" or "this factory returns a default MutableSharedFlow". Hash it into the declaration's surface, so that the existing lookup dependents and waves carry it with no new invalidation path (**inferred**; not built).
- **Where.** Either fits behind D as an opt-in for the rules whose cross-file recall is worth its cost. LocaleGetDefaultForFormatting (shared formatter holders) is the most plausible candidate.

## 4. Reproducing

```
cd tools/krit-fir
./gradlew tieredResearchTest --tests dev.jasonpearson.krit.fir.incremental.TieredDepthProbeTest [-Pkrit.fir.tiered.only=Rule]
./gradlew tieredResearchTest --tests dev.jasonpearson.krit.fir.incremental.TieredResearch -Pkrit.fir.tiered.mode=synthetic -Pkrit.fir.tiered.size=600 -Pkrit.fir.tiered.edits=40
./gradlew tieredResearchTest --tests dev.jasonpearson.krit.fir.incremental.TieredResearch -Pkrit.fir.tiered.mode=kritfir -Pkrit.fir.tiered.edits=40 [-Pkrit.fir.tiered.ungated=false]
./gradlew tieredResearchTest --tests dev.jasonpearson.krit.fir.incremental.TieredResearch -Pkrit.fir.tiered.mode=static -Pkrit.fir.tiered.roots=<comma-separated source roots> [-Pkrit.fir.tiered.stubs=false]
```

Options: `krit.fir.tiered.depths` (default `1,2,3,inf`) and `krit.fir.tiered.seed` (default 11).

## Appendix: measured vs inferred

**Measured:**
- every table in sections 2.1-2.3;
- the per-prefix and closure results of 2.2;
- the AutoMobile and krit-fir tiered findings and their file-locality (the AutoMobile tags were checked by hand: same-file `const` or literal);
- golden scenario counts per rule (all single-file);
- the cross-file golden tests that D would change.

**Inferred:**
- implementation sizes;
- C's window lengths beyond the #740 measurements;
- AutoMobile times (sizes only: no snapshot with errors);
- that TooGeneric's module check is exact on a slice;
- the LongLogTag `const` difference's cause (the message spelling);
- the rule-fact-as-surface design.

**Not done:**
- C was not built;
- R was simulated from the generator's knowledge of read sets, not from instrumented rules;
- no real code with a classpath and tiered cross-file findings was available, so B's and R's correctness on real code is shown only on the synthetic corpus and the depth probes.
