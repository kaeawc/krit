# Incremental FIR checkers (#740), FIR-only: K2 CLI pipeline, no Analysis API

Branch: `claude/krit-issues-iteration-79gicw-740-fir-spike` (spike, not for merging). All spike code is in the `spike` source set of `tools/krit-fir` (`src/spike/kotlin/spike/`); nothing under `src/main` changes. Outside it the branch edits only `tools/krit-fir/build.gradle.kts` (spike source set associated with `main` so it can use `internal` krit-fir code; `kotlin-build-tools-api`; the BTA impl, `jvm-abi-gen` and a relocated plugin jar for the harness; the `spikeClasspath` task). Nothing uses `buildStandaloneAnalysisAPISession`, `KaSession`, `collectDiagnostics` or LL FIR: every compile below is the K2 CLI pipeline (`AbstractCliPipeline` + `JvmConfigurationPipelinePhase` + a frontend phase), with krit's real `KritPluginRegistrar` loaded from the plugin jar exactly as `AnalysisSession` loads it.

Every number is labelled **measured** (this spike, this machine) or **inferred** (reasoned, not run). Appendix A lists both.

## 0. Summary and recommendation

**Recommendation: build F2, the slice compile against a binary snapshot of the module, on the *stock* `FrontendOnlyJvmPipeline`, with dependents from the compiler's own `LookupTracker` gated by a FIR declaration-surface diff. Tier off the 11 rules that read other files' bodies or initializers. Keep the whole-module compile as the fallback.** Add F1 (warm library sessions) later, and only as the "F1+F2" combination with an immutable snapshot. Do not pursue F3 (Build Tools API IC) as the checker driver, or F4 (in-place session update).

1. **F2 works through the shipped pipeline with no compiler internals beyond the IC interfaces.** K2's frontend already compiles a subset of a module against that module's previous class output: it is how Gradle/JPS IC works. `JvmConfigurationUpdater` reads `IncrementalCompilationComponents` and `LookupTracker` from `Services`. `FrontendOnlyJvmPipeline` therefore does a slice compile unchanged when it gets: the dirty files as sources; the snapshot directory on the classpath; the same `-module-name`; `-Xenable-incremental-compilation`; and an `IncrementalCompilationComponentsWithCustomScope` (about 30 lines). The binaries carry the **source module's own module data**, so `internal`, sealed subclasses across source/binary, multifile facades, inline functions and `const` all resolve as in a full compile. The naive variant does not: with the snapshot as a friend library, a sealed subclass fails with `Extending sealed classes or interfaces from a different module is prohibited`.
   - **Speed (measured).** Stock-pipeline slice p50 is 166–180 ms at 600 files, 283–301 ms at 2,000 files, 322 ms at 10,000 files and 0.74 s on krit-fir's own 126-file sources, against 2.9 s, 13.8 s and 4.3 s for today's whole-module compile. With F1's warm environment it is 108 ms at 600 files (body edits 62–67 ms).
   - **Correctness (measured).** On the synthetic projects, the per-file verdict cache equalled a fresh full compile after every one of 60 edits in every F2 variant (0 stale), and the lookup index flagged exactly the truly affected files for every edit kind. On krit-fir's own sources: 0 stale verdicts over 60 edits (F2) and 2 x 60 edits (F1+F2), section 2.4.
2. **Dependents without the Analysis API: the compiler's `LookupTracker` plus a FIR surface diff is exact where the AA spike's edges were 1–4x over.**
   - **Precision.** On synthetic signature, overload, sealed and delete edits it flagged exactly the truth (flagged = truth on average for every kind). Body and comment edits compile only the edited file. Tracker overhead is within noise, ≤ ~7% inferred.
   - **Structural rules.** Lookups cannot see four things, and the dirty-set computation must add them: a sealed parent whose binary lists stale inheritors; a redeclaration or conflicting overload that needs both sides in source; implementers of a changed abstract member; and `expect`/`actual`. Each was found by a probe or a real-code miss and is now covered.
   - **krit-fir's `DepTracker` is not a dependents source.** It records only supertype edges, so it missed 42 of 45 truly affected files on real renames and all of them on synthetic edits.
3. **The main correctness risk is rules, not the compiler.** A split differential over krit-fir's 718 rule test-data scenarios (4,223 files, 77 rules with findings) found **11 rules whose verdict changes when other files come from binaries**: HardcodedSecretKey, ShortAlarm, SharedFlowWithoutReplay, StaticIv, LongLogTag, CollectionsSynchronizedListIteration, SerialVersionUIDInSerializableClass, LocaleGetDefaultForFormatting, WorldWriteableFiles, JdbcStatementExecute and WeakKeySize.
   - They follow resolved symbols into other declarations' bodies or initializers (or scan the compilation's files).
   - The same reads make their verdicts depend on **non-surface** facts of other files, so no ABI- or lookup-based dirty set covers them.
   - These must run on the full tier, behind a lint-gated flag like `requiresWholeCompilation` on branch `-862`. F3 has the same problem.
4. **F2 needs an error-free module for its snapshot.** The backend never runs when the frontend reports an error. AutoMobile without a classpath, where hundreds of files have errors, can only use the full tier or F1. That is a real limit for krit runs without a Gradle-provided classpath.
5. **F1 (warm library sessions) is possible, but only by replacing the frontend phase, it needs four fixes, and it saves little on its own.**
   - **Savings (measured).** Whole-module p50 is −9% at 2,000 files (2.63 s vs 2.88 s) and −10 to −22% on krit-fir (3.6–3.9 s vs 4.3–4.5 s), −5% on AutoMobile (20.4 s vs 21.6 s) and nothing at 10,000 files (14.8 s vs 13.8 s, within noise). Environment-only reuse saved nothing measurable (2.85 s).
   - **The four fixes.**
     - Replay plugin registrars into each new configuration; without it krit's checkers silently stop running.
     - Pin one `LanguageVersionSettings` instance. A library deprecation cache keyed by it retained every compile's FIR: +112 MB per edit at 2,000 files, and the run thrashed at 4 GB.
     - Give each compile fresh `FirSharableJavaComponents`. This is the only reflective step.
     - Drop the diagnostics held by reused configurations.
   - **Memory with the fixes.** Heap is flat over 70 edits (160 MB at 2,000 files; 198–215 MB on krit-fir).
   - **Where it pays.** F1's value is in the F1+F2 combination, where the per-compile fixed cost dominates. That combination needs the snapshot directory to stay immutable: the CLI Java file manager caches class lookups, including misses, and mutating the directory produced massive staleness until exclusion became a scope subtraction.
6. **F3 (Build Tools API IC with krit's plugin) is fast and exact only while the module compiles cleanly.**
   - **Speed (measured).** 281–349 ms per edit at 600–2,000 files. On krit-fir, 0.6 s with the backend running every time. 0 stale verdicts on an edit mix that never introduces errors.
   - **Behaviour with errors.** IC stops propagating at the first iteration with an error, so dependents of an inferred-type change are never recompiled; stale verdicts accumulated on every edit of the error-producing mix (424 files after one hot edit).
   - **Packaging.** The plugin has to be relocated for the embeddable compiler. It also runs in BTA's class loader, so krit's per-request rule context and rule-isolation recorder (thread-locals) never reach it.
   - **Other properties.** The K2-native IC runner is deprecated at ERROR level, and IC caches add 1.7–6 MB on disk.
   - **What it proves.** IC validates the F2 mechanism; it is not a checker driver.
7. **F4 (in-place update of the CLI source session) is blocked.**
   - **What works.** With `FirProviderImpl.ensureConsistent` (`@TestOnly`) and a swapped `FirCachingCompositeSymbolProvider`, re-resolved synthetic files matched a fresh compile (60/60), and without the swap 5/11 edits went stale.
   - **What blocks it.** On krit-fir's real code a file re-resolved in place got false compiler errors that persisted, so 0 of 30 edits matched a fresh compile; updates cost 3.2 s p50 and heap reached 2.2 GB. Heap grew ~15 MB per edit at 600 files, and there is no supported invalidation or removal API.
8. **Oracle facts (#859) survive a slice**, with two gaps. Declarations, diagnostics and lambda flags matched a full compile for 30/30 sampled krit-fir files and expression types for 28/30. The two differences are callee `@OptIn` annotations, which have `SOURCE` retention and so are not in class files. DepTracker's supertype edges are lost for 26/30 files, because supertypes from the snapshot are `Precompiled`, not `Source`, and need mapping back through the snapshot's output→source map.

## 1. Method and environment

- **Machine.** 4 vCPU, 15 GB, JDK 21, Kotlin 2.4.20 compiler (`kotlin-compiler`, what `tools/krit-fir` bundles), BTA 2.4.20.
- **Shared container.** It was shared with other lanes; the 1-minute load average during timing runs was 1.2–6.4 and is recorded per row. Timing runs were serialized under `flock /tmp/claude-0/krit-bench.lock`. Correctness-only runs (rule differential, oracle facts, one BTA debug run) ran niced outside the lock; their timings are not used. Process CPU is reported where it helps.
- **What each compile is.** krit's real checkers through `KritPluginRegistrar` (all 79 rules, empty rule configs). The krit-side setup is identical to `AnalysisSession.CheckPlan`: `FirRuleContext` set on the calling thread, a `FirRuleErrorRecorder` armed, and a `FindingCollector` over the requested files, plus a second collector of every located diagnostic for the strict comparison. Spike `Engine.kt` (`KritCheck`, `runShipped`, `runCheck`).
- **Baseline ("CLI").** The shipped `FrontendOnlyJvmPipeline` over the whole module per edit, `AnalysisSession`-equivalent arguments.
- **Verdicts compared.** *Verdict* level is what Go consumes: krit findings, the compiler-ERROR gate (`errorFiles`), and the rule-error set. *Strict* level is every compiler diagnostic, including warnings, text and position.
- **Correctness method.** After every edit (or every Nth, sampled at 2,000/10,000 files), the daemon-side per-file verdict cache is compared with a **fresh shipped frontend-only compile of the whole module**.
  - "Truth" for dependents = files, other than the edited ones, whose fresh verdict changed with the edit.
  - "Flagged" = what the lookup index chose.
  - "Missed" = truth not re-checked.
- **Corpora.**
  - **Synthetic.** 600 / 2,000 / 10,000 files, regenerated with the AA spike's `Gen.kt`: lib/app, three lib refs per app file, an app→app chain so inferred types propagate, and a hot lib file imported by the first 500 app files. Compiled as one module, as `AnalysisSession` does.
  - **Synthetic edit mix** (unchanged from the AA spike): body 35%, signature 20%, overload 8%, add 10%, delete 10%, shadow 6%, sealed subclass 5%, comment 4%, hot signature 2%.
  - **krit-fir.** `tools/krit-fir/src/main` + `tools/krit-rule-api/src/main` + its generated `RuleApiVersion.kt`: 126 files, 25.9k lines, against `kotlin-compiler-2.4.20`, `kotlin-stdlib`, `annotations-23` with `-Xcontext-parameters`. It compiles with 0 errors. That needs the generated root; the first runs without it had 1–6 error files and are labelled.
  - **AutoMobile.** A fresh shallow clone of `kaeawc/auto-mobile`, `android/{protocol,desktop-domain,control-proxy,desktop-core,junit-runner,auto-mobile-sdk,video-server}/src/main/kotlin`: 646 files today (the AA spike had 637). There is no Gradle classpath, so hundreds of files carry compiler errors.
  - **Real-project edit mix.** String-literal edits (body), appended comments, renaming a function another file mentions (toggled back later), and adding or deleting a trivial file.
- **Warm-up.** Each timing run discards its first 4–15 edits (JIT and class loading). The cold row is the first compile in a fresh JVM.
- **Memory.** Heap after forced GC, metaspace, loaded classes, threads, FDs, RSS and VmHWM, every 10 edits.

## 2. Strategy results, next to the Analysis API spike

### 2.1 Strategies

| id | per edit | entry points |
|---|---|---|
| CLI | shipped path: `FrontendOnlyJvmPipeline` over the whole module, krit plugin loaded (`AnalysisSession.check` equivalent) | shipped |
| F1 | `WarmFrontendPhase`: project environment and library sessions kept in daemon state, new source session over the whole module | replacement frontend phase (section 3) |
| F1 env-only | as F1, but library sessions rebuilt per compile | same |
| F2 patch / keep / keepall | stock `FrontendOnlyJvmPipeline` over the dirty set (edited + lookup dependents + structural rules) against a binary snapshot of the module through the IC interfaces; snapshot freshness (a) patch, (b') keep surface-stale, (b) keep every edited file (section 4.4) | stock pipeline + `IncrementalCompilationComponentsWithCustomScope` + `LookupTracker` |
| F1+F2 keep | F2's dirty set compiled by F1's warm phase against an immutable snapshot (stale outputs subtracted from the IC scope) | both |
| F3 | Build Tools API snapshot-based IC with krit's plugin (`-Xplugin`), recompiled set chosen by Kotlin IC | `kotlin-build-tools-api` 2.4.20 (section 6) |
| F4 | one CLI source session kept alive; edited files and dependents re-parsed into it and re-resolved | `@TestOnly FirProviderImpl.ensureConsistent` + provider swap (section 7) |

The dirty set of F2 / F1+F2 is the same in every run: changed files, plus lookup dependents of every surface symbol that changed, plus the structural rules of 4.2, repeated in waves while compiled files' surfaces change (max 8), plus files kept in source because their binary is surface-stale or they have errors. Above a threshold (400 files; 1,500 at 10,000 files) the edit falls back to the full tier, which also re-snapshots.

### 2.2 Headline numbers (ms, p50 / p95 after warm-up; measured)

FIR-only rows are this spike. AA rows are copied from the previous spike's findings (`fir-incremental-findings.md`, sections 2.2-2.4) for comparison; they ran on the same machine type at different loads (4-17 vs 1.3-4.9 here), so differences under ~20% are noise.

| strategy | synthetic 2,000 | synthetic 10,000 | krit-fir (126 files) | AutoMobile (646 files, errors) | first build, cold JVM (2,000 / 10,000 / krit-fir / AM) | heap after GC over the run |
|---|---|---|---|---|---|---|
| **CLI** (n 30 / 12 / 24 / 10) | 2,876 / 3,124 | 13,754 / 14,118 | 4,317 / 4,947 | 21,554 / 22,809 | 18.1 / 39.5 / 21.0 / 54.6 s | 52 / 195 / 39 / 110 MB |
| **F1** warm env + libs (n 60 / 20 / 40 / 20) | 2,627 / 3,007 | 14,753 / 15,498 | 3,866 / 4,240 (3,551 / 4,034 with the generated root missing) | 20,376 / 22,576 | 18.7 / 40.3 / 21.4 / 55.5 s | 160 / 655 / 215 / 643-667 MB, flat |
| F1 env-only (n 40) | 2,848 / 3,370 | | | | 25.2 s | |
| **F2 patch** (n 100 / 40 / 60) | 283 / 799 | 322 / 942 | 738 / 1,471 | not possible: no snapshot (434 files with errors) | check + snapshot 28.1 / 69.1 / 33.5 s | 47-58 / 155-159 / 41-43 MB |
| F2 keep (n 100) | 301 / 920 | | | | 29.3 s | 47-58 MB |
| **F1+F2 keep** (n 150 / 100 / 60) | 265 / 798 | 276 / 861 | 505 / 1,000 (seed 31), 200 / 772 (seed 32), 527 / 823 (timing run) | 21,073 / 21,997 (no snapshot: every file stays in source) | 29.6 / 67.8 / 33.8-35.4 s | 65 -> 90 / 178 -> 185 / 98 -> 135 MB |
| F3 BTA IC (n 40 / - / 20-30) | 349 / 403 (backend every edit) | | 610 / 1,137 (clean edits); 804 / 1,366 with renames, 13/30 edits stale | | 27.7 s / - / 34.5-45.5 s (IC caches 1.6-6 MB) | 31-33 MB in the host (the compiler runs in its own loader) |
| F4 in-place (n 60 at 600; 30 krit-fir) | 539 / 990 at 600 files, 17 stale verdicts | | 3,231 / 4,235, **0/30 edits identical** | | | +15 MB per edit at 600; 2.2 GB after 30 krit-fir edits |
| AA CLI | 2,356 / 2,692 | 13,423 / 21,678 | 4,368 / 7,125 | 18,677 / 21,988 | 57.6 s (krit-fir, loaded) | |
| AA B (warm JDK/stdlib modules) | 479-518 / 961-965 | 1,656-1,868 / 2,216-3,253 | 675 / 5,126 | 1,474 / 57,568 | | 118-134 / 385-405 MB |
| AA D (lexical slice) | 156 / 579 | 166-202 / 608-654 | 561 / 6,361 | 1,374 / 57,891 | | 201-217 MB at 10,000 |
| AA S/owner (surface-gated, unsafe) | 180 / 853 | 151 / 535 | 428 / 4,980 | 1,405 / 58,001 | | 79-96 / 202-218 MB |
| AA R (resolved reverse edges) | 209 / 922 | 235 / 1,855 | 445 / 1,917 | 1,632 / 23,930 | | 149-165 / 477-493 MB |
| AA E (one live session, verified only at 2,000) | 49 / 66 | | 347 / 1,524 (timing only) | 200 / 1,094 (timing only) | | 71 MB |

Reading:
- **F2 is in the same class as the AA spike's slice strategies (D/S/R) per edit**, 10-50x faster than today's whole-module compile, and unlike S/owner it is safe by construction (the compiler's own lookups, not lexical matching). At 10,000 files it stays at ~0.3 s while the CLI takes 13.8 s.
- **F1 alone barely helps** (−9% at 2,000, −10 to −22% on krit-fir, nothing at 10,000 or on AutoMobile): the frontend's per-compile cost is resolving and checking the sources, not building library sessions. Its value is in F1+F2, where it cuts the fixed per-compile part: 108 vs 166-180 ms at 600 files, 265 vs 283-301 ms at 2,000, 505 vs 738 ms on krit-fir.
- **Nothing in FIR-only reaches AA E's ~50 ms**: E keeps one live session and re-analyzes a single dangling file. The FIR-only analogue is F4, which is blocked (section 7).
- **The first build costs +55-75% once (cold JVM)** for the snapshot (backend on the full compile). A daemon pays it once, at the first request of a session; a one-shot `krit` run would pay it without benefit, so it should stay on the full tier unless the snapshot is persisted (plan step 9).
- Compile-error-heavy modules (AutoMobile without a classpath) get nothing from F2: the backend never runs.

### 2.3 Per edit kind, synthetic (ms p50; compiled-file count p50 in brackets)

| kind | CLI 2,000 | F1 2,000 | F2 patch 2,000 | F2 keep 2,000 | F1+F2 keep 2,000 | F2 patch 10,000 | F1+F2 keep 10,000 | F1+F2 keep 600 | AA D / S-owner / R 2,000 |
|---|---|---|---|---|---|---|---|---|---|
| body | 2,886 | 2,622 | 243 [72] | 261 [73] | 216 [104] | 278 [32] | 239 [89] | 67 [30] | 135 / 156 / 121 |
| comment | | 2,615 | 283 | 279 | 256 | 263 | 249 | 124 | 103 / 127 / 92 |
| add file | 2,501 | 2,511 | 246 | 234 | 225 | 314 | 238 | 73 | 132 / 162 / 202 |
| delete file | 2,876 | 2,522 | 247 | 252 | 195 | 327 | 284 | 79 | 148 / 169 / 220 |
| lib signature | 2,710 | 2,696 | 715 | 778 | 608 | 936 | 796 | 266 | 273 / 323 / 657 |
| new overload | 2,735 | 2,691 | 679 | 763 | 634 | 795 | 734 | 193 | 243 / 284 / 655 |
| sealed subclass | 2,771 | 2,868 | 443 | 529 | 410 | 593 | 501 | 114 | 50 / 73 / 70 |
| shadow `println` | 3,413 | 2,677 | 251 | 229 | 238 | 9 | 245 | 1 | 113 / 125 / 143 |
| hot signature (500 dependents) | 2,811 | 2,427 | 4,954 (fallback: full + re-snapshot) | 4,747 (fallback) | 5,319 (fallback) | 3,207 [537] | 3,442 [538] | 1,046 [599] | 13,487 / 14,111 / 11,026 |

The synthetic generator's signature edits break their dependents (they change a return type to `Long` where callers expect `Int`) until a later edit toggles them back, so files with errors accumulate in the source set (no fresh binary): that is why a body edit compiles 30-100 files here. On krit-fir, which has no such edits, slices compile 2-13 files. Per-edit cost follows the slice size, not the module size: the 10,000-file correctness run (F1+F2 keep, 24 edits, keep set 13-27 files) measured body edits at 75 ms p50 and all edits at 103 ms p50 / 267 ms p95, against 239 / 276 ms in the timing run whose keep set had grown to 133 files. Signature and overload edits take 2-3 waves (an inferred return type changes, so its dependents' surfaces change too).

### 2.4 krit-fir's own sources (126 files, 0 errors; measured)

| kind | CLI | F1 | F2 patch | F1+F2 keep (seed 31) | F1+F2 keep (seed 32) | F3 |
|---|---|---|---|---|---|---|
| string literal (body) | 4,245 | 3,799 | 753 [9] | 490 [9] | 151 [4] | 610 (literal/comment/add mix) |
| comment | 4,382 | 3,932 | 736 | 508 | 200 | |
| add file | 4,956 | 3,587 | 728 | 514 | 198 | |
| delete the added file | 5,025 | 3,542 | 0 (no dependents, nothing to compile) | 0 | 0 | |
| rename a function another file uses | 4,291 | 4,011 | 753 / p95 1,491 (2 waves) | 570 / p95 1,408 | 406 / p95 836 | 804 overall, 13/30 stale |

Correctness, every edit compared with a fresh full compile: F2 patch 0 stale verdicts over 60 edits; F1+F2 keep 0 stale over 2 x 60 edits; F1 0 stale over 46. The first F2 run on krit-fir (before structural rule (3) of 4.2) had 27 stale edits, all one cause: a renamed abstract interface member left an implementing class's binary-backed verdict stale (`Class ... is not abstract and does not implement abstract member`). Rename flagged 2.8 files per edit on average = 2.8 truly affected; krit-fir's `DepTracker` edges would have found 0.3 (42 of 45 missed).

### 2.5 AutoMobile (646 files, no Gradle classpath, 434 files with compiler errors; measured)

CLI 21.6 s p50, cold 54.6 s, 1.35 M lookup records. F1: 20.4 s p50 (p95 22.6 s, n 20), −5%, heap after GC 643-667 MB flat over 20 edits (RSS 2.7 GB). F2 and F1+F2 cannot build a snapshot (the backend never runs with errors), so every edit runs the full tier: measured F1+F2 keep at 21.1 s p50 (n 12; every file stays in the source set because no binary exists). The AA spike's R strategy, which does not need class files, did better here (1.6 s p50, 7.7 s for renames), so F2's error-free-module requirement is the main functional regression against the AA approach. Two ways out, both inferred: give such modules the Gradle classpath (krit's Gradle plugin knows it), or snapshot a module with errors through `-Xheader-mode` / a lenient backend (header mode crashed on krit-fir in 2.4.20, section 4.3).

### 2.6 Memory over long runs (heap after forced GC; measured)

| run | edits | heap after GC | RSS / VmHWM | notes |
|---|---|---|---|---|
| F2 patch 2,000 | 110 | 47 -> 58 MB | | snapshot index and lookup index in the harness |
| F1+F2 keep 2,000 | 160 | 65 -> 90 MB | | keep set grows to 60-190 files (generator's broken dependents) |
| F2 patch 10,000 | 40 | 155 -> 159 MB | | 231k lookup edges |
| F1+F2 keep 10,000 | 110 | 178 -> 185 MB | 966 MB / 2.0 GB | |
| F1 2,000 / 10,000 / krit-fir | 70 / 20 / 40 | 160 / 655 / 198-215 MB, flat | | after the four fixes of 3.2; before them +112 MB per edit at 2,000 |
| F1+F2 keep krit-fir | 60-70 | 98 -> 135 MB | | still rising slowly at the end of the run (+0.5 MB per edit); not investigated |
| F4 600 / krit-fir | 60 / 30 | 369 -> 957 MB / 2.2 GB | 4.6 GB | no eviction |

Metaspace stayed at 84-103 MB, threads at 1 (in-process compiles), FDs at 19-20 in every run.

### 2.7 Correctness against a fresh full frontend compile, all 79 rules (measured)

Every compared edit: the daemon-side verdict of **every file of the module** (fresh for compiled files, cached for the rest) against a fresh shipped `FrontendOnlyJvmPipeline` compile of the whole edited module. "Verdict" = krit findings + compiler-error gate + rule errors (what Go consumes); "strict" = every diagnostic with text and position.

| strategy | corpus, edits compared | stale verdicts (edits / file verdicts) | strict |
|---|---|---|---|
| F1 | synthetic 600, 40; krit-fir, 46 | 0 / 0 | 0 |
| F2 patch | synthetic 600, 60; krit-fir, 60 | 0 / 0 | 0 |
| F2 keep, keepall | synthetic 600, 60 and 40 | 0 / 0 | 0 |
| F1+F2 keep | synthetic 600, 60 + 40; krit-fir, 2 x 60; synthetic 10,000, 24 edits, 6 compared: every 4th edit, on the compiled files plus 400 random files | 0 / 0 | 0 |
| F2 patch before structural rule (3) | krit-fir, 60 | 27 / 27 (implementer of a renamed abstract member) | 27 |
| F1+F2 patch, snapshot directory mutated under the warm environment (abandoned design, 4.5) | synthetic 600, 60; 2,000, 60 | 34 / 588; 59 / 1,627 | 59 / 4,053; 59 / 4,727 |
| F3 | synthetic 600 clean mix, 40; krit-fir literal mix, 20 | 0 / 0 | 0 |
| F3 | synthetic 600 full mix, 40; krit-fir with renames, 30 | 40 / 3,059; 13 / 13 | same |
| F4 | synthetic 300 fixed kinds, 11; 600 random, 60; krit-fir, 30 | 0; 8 / 17; 30 / 30 | |
| all slice strategies, cold first build (check + snapshot) vs the shipped compile | every run | 0 | 0 |

These comparisons cover the checkers as a whole (all 79 rules enabled with default configs). The corpora only produce findings for some rules (23 findings at 600 files, 80 at 2,000, 54 on krit-fir), so per-rule slice safety was tested separately by the split differential of section 8, which exercised 77 rules: 11 rules differ on a slice. The edit mixes above never change the cross-declaration facts those rules read (key bytes, tag strings, wrapper bodies in other files), so the 0 stale verdicts here do not clear them; in an Android project they would show up.

## 3. F1: warm library sessions across K2 frontend compiles

### 3.1 How

The CLI frontend phase (`JvmFrontendPipelinePhase.executePhase`) builds everything per compile: the project environment (`createProjectEnvironment`: IntelliJ core project, jar file system, classpath roots index, Java module graph, Java file manager), the shared library session, the classpath library session, then the source session. It has no injection point for prebuilt sessions, so reuse needs a replacement phase. `WarmFrontendPhase` (spike `Engine.kt`) is a line-for-line port of `executePhase` that takes the environment and library sessions from daemon state when present and builds only the source session per compile. It runs inside the normal CLI pipeline: `JvmConfigurationPipelinePhase then WarmFrontendPhase` under `AbstractCliPipeline.execute`, so argument parsing, plugin loading, diagnostics reporting to the `MessageCollector` (krit's `FindingCollector`) and the error check are the stock ones. Entry points used, all public in 2.4.20:

| step | entry point |
|---|---|
| environment | `JvmFrontendPipelinePhase.createProjectEnvironment(configuration, disposable, EnvironmentConfigFiles.JVM_CONFIG_FILES)` |
| sources | `collectSources(configuration, env).allFiles` (light tree) |
| library list | `JvmFrontendPipelinePhase.createLibraryListForJvm(moduleName, configuration, friendPaths)` |
| library sessions | `FirJvmSessionFactory.Context(configuration, env, librariesScope)`, `createSharedLibrarySession(...)`, `createLibrarySession(...)` |
| source session | `FirSourceModuleData(...)`, `FirJvmSessionFactory.createSourceSession(moduleData, javaSourcesScope, icProviders, registrars, configuration, context, needRegisterJavaElementFinder = false, KmpModuleKind.SingleModule) { register FirBuiltinSyntheticFunctionInterfaceProvider; useCheckers(CliOnlyLanguageVersionSettingsCheckers) }` (opt-in `SessionConfiguration`) |
| resolve + check | `session.buildFirViaLightTree`, `resolveAndCheckFir`, `runPlatformCheckers` |
| backend (snapshot) | `... then JvmFir2IrPipelinePhase then JvmBackendPipelinePhase then JvmWriteOutputsPhase` |

Stability: these are compiler internals with no compatibility promise. Their shapes moved between 2.x releases (`createSourceSession` gained `kmpModuleKind`; session construction moved into `JvmFrontendPipelinePhase`/`SessionConstructionUtils`), so the phase has to be re-ported on each Kotlin bump, the same exposure `FrontendOnlyJvmPipeline` already has, only wider (about 120 lines instead of one phase composition).

### 3.2 Four things a reused environment / library session needs (all found by running it)

1. **Plugin extensions.** `KotlinCoreEnvironment.ProjectEnvironment.registerExtensionsFromPlugins` runs the compiler plugin registrars into the configuration's extension storage only when the environment is created. With a reused environment the next compile's configuration has no plugin extensions: krit's checkers silently do not run (measured: 11 findings on the first warm compile, 0 on the second). Fix: replay `CompilerPluginRegistrar.registerExtensions` into each new configuration's `extensionsStorage`.
2. **`LanguageVersionSettings` identity (memory leak).** Library declarations cache deprecation info per use-site `LanguageVersionSettings` (`DeprecationsProviderImpl.cache`, seen on the `kotlin.Exception` typealias). Each compile creates a new instance; the cache entry keeps it, and through its `JavaTypeEnhancementState` reporter lambda the whole `CompilerConfiguration`, its diagnostics collector, a checker context, and the source session with all of its FIR. Measured: +18 MB/edit at 300 files, +112 MB/edit at 2,000 files, the 2,000-file run hit the 4 GB heap after ~35 edits and thrashed. Retention chain found by a reflective BFS from the reused state (`LeakFinder.kt`):
   `libraryList -> FirBinaryDependenciesModuleData.boundSession -> FirCachingCompositeSymbolProvider.classLikeCache[kotlin/Exception] -> FirTypeAliasImpl.deprecationsProvider.cache[LanguageVersionSettingsImpl] -> analysisFlags[javaTypeEnhancementState] -> JavaTypeEnhancementStateParser.reporter -> CompilerConfiguration -> DIAGNOSTICS_COLLECTOR -> KtLightDiagnosticWithParameters2.context -> CliDiagnosticsCollector.session -> FirCliSession`.
   Fix: pin one `LanguageVersionSettings` instance for the state's lifetime (`configuration.languageVersionSettings = pinned`; any difference in the settings string rebuilds the library sessions).
3. **`FirSharableJavaComponents` across compiles (memory + correctness hazard).** The Java enhancement and JVM-mapped-scope storages in `FirJvmSessionFactory.Context.predefinedJavaComponents` are shared by every session built from one context, which the CLI intends for the sessions of one compilation (common + platform). Reused, they cache symbols created for the first compile's source module (`FirEnhancedSymbolsStorage.enhancedFunctions` keyed by a function whose `moduleData` is compile #0's `FirSourceModuleData`), so later compiles resolve against a declaration bound to a dead session. Fix (the only reflective step): give each compile's source session a fresh `FirSharableJavaComponents` (the field is a private `val`).
4. **Held configurations.** The environment keeps the configuration it was created with, and the pinned settings keep theirs; both reach that compile's diagnostics and FIR. Fix: after each compile, replace those configurations' `diagnosticsCollector` with an empty one.

Environment-only reuse (library sessions rebuilt per compile) needs fix 1 and 4 only; it was flat in memory without the others.

## 4. F2: slice compile against a binary snapshot of the module

### 4.1 The mechanism is the compiler's own IC path, reachable through the stock pipeline

K2 already compiles a subset of a module against the module's previous class output: that is how Gradle/JPS incremental compilation works. `prepareIncrementalCompilationContextAndLibrariesScope` (called by `JvmFrontendPipelinePhase.executePhase`) builds an `IncrementalCompilationContext` when `configuration.incrementalCompilationComponents` is set, and `createSourceSession` then adds a `JvmClassFileBasedSymbolProvider` over the previous output **with the source module's own `FirModuleData`** (declarations get origin `Precompiled`), placed after the source providers and before the libraries. The output scope is subtracted from the libraries scope. The configuration phase reads all of it from `Services`:

- `IncrementalCompilationComponents` -> `incrementalCompilationComponents` (needs `-Xenable-incremental-compilation`)
- `LookupTracker`, `ImportTracker` -> trackers (read even without the IC flag: "used by Build Tools API in non-incremental compilations")
- `ExpectActualTracker`, `InlineConstTracker`, `EnumWhenTracker`, `ICFileMappingTracker` (IC flag)

So **the shipped `FrontendOnlyJvmPipeline` does a slice compile unchanged**, given: `freeArgs` = the dirty files; the snapshot directory on the classpath; the same `-module-name`; `-Xenable-incremental-compilation`; and `Services` carrying an `IncrementalCompilationComponentsWithCustomScope` (an interface the compiler keeps for IntelliJ's IC) whose `createSearchScope` returns the snapshot directory and whose `IncrementalCache.getModuleMappingData()` returns the snapshot's `META-INF/<module>.kotlin_module` (that is all the frontend reads from the cache: package -> file-facade parts, for top-level callables). Spike: `Snapshot.kt` (`SnapshotComponents`, `SnapshotCache`), about 30 lines of glue, no compiler internals beyond those IC interfaces.

Stale outputs of the files in the slice must not be visible, or a top-level function would exist twice (source and binary). The stock path moves them out of the snapshot directory for the duration of the compile (`Snapshot.park/unpark`, cheap: a few renames). A reused environment (F1+F2) must not see the directory change at all (section 4.4), so that path subtracts them from the IC scope instead (`env.getSearchScopeByIoFiles`), with the snapshot directory immutable.

Source -> output map: the backend reports every written class with its sources as `CompilerMessageSeverity.OUTPUT` messages when `-Xreport-output-files` is set (`OutputMessageUtil.parseOutputMessage`); `ICFileMappingTracker` (IC services) gives the same. Measured: 2,503 class files for 2,002 sources (synthetic: one object + one class per lib file).

### 4.2 Constructs (measured, `Shapes.kt`; each slice verdict compared with a full frontend compile of the same edited module)

| construct | same-module IC binaries (proposed) | snapshot as a friend library (`-Xfriend-paths`, naive) |
|---|---|---|
| `internal` declarations from the binary part | identical | identical |
| top-level functions/properties via `@file:JvmName` facade | identical | identical |
| `@JvmMultifileClass`: one part in source, the other binary | identical | identical |
| subclass in source of a sealed parent in the binary | identical | **error** `Extending sealed classes or interfaces from a different module is prohibited` (SEALED_INHERITOR_IN_DIFFERENT_MODULE) |
| exhaustive `when` over a binary sealed hierarchy | identical (missing branch is an error in both) | identical |
| new sealed subclass in another file, `when` file also in the slice | **differs**: the parent's binary metadata still lists the old inheritors, so the `when` is wrongly exhaustive. Identical once the parent's file is in the slice too | differs (plus the module error above) |
| redeclaration (new `class Dup` in B, `Dup` already in binary A) | **differs**: no error in B (the source class shadows the binary one); A's verdict also changes in the full compile | differs |
| conflicting overloads (new `fun ov(Int)` in B, same in binary A) | identical for B; A's verdict also changes in the full compile | **differs**: no error in B |
| inline function bodies and `const val` from the binary (incl. `when` branch on a const, reified inline) | identical | identical |
| `expect` in a common source, `actual` in the slice (`-Xmulti-platform`) | **differs**: `has no corresponding 'expect' declaration` (K2 JVM never writes `expect` declarations to class files) | differs |
| LongLogTag on a `const val` tag declared in the binary part | identical (2 findings) | identical |

Consequences built into the dirty-set computation (`Inc.kt`, `structural`): (1) a class added, removed or changed whose supertypes include a module class pulls in the supertype's declaring file (sealed parents carry their inheritor list), (2) an added or removed top-level name pulls in every other file declaring the same name in the same package, and (3) a member added, removed or changed in a module class pulls in the declaring files of all its module subclasses (an implementer of a renamed abstract member has no lookup of the new name; found on krit-fir, where it caused 27 stale verdicts before the rule). (4) Modules with `expect` declarations stay on the full tier (or keep every `expect`-declaring file in every slice).

### 4.3 Snapshot cost: the backend once (measured, `SnapCost.kt`, one JVM, kinds run in this order so the first kind carries more JIT warm-up; min shown with p50)

| corpus | frontend only (today) | full compile with backend | `-Xheader-mode` | full + `jvm-abi-gen` | warm env+libs, frontend | warm env+libs, full |
|---|---|---|---|---|---|---|
| synthetic 2,000 (3 reps) | p50 3,820 / min 3,102 ms | 6,609 / 5,930 ms, 2,503 classes, 3.6 MB | 3,038 / 2,971 ms (no checkers run in header mode), 2.5 MB | 6,651 / 6,508 ms | 2,423 / 2,248 ms | 5,068 / 4,912 ms |
| synthetic 10,000 (2 reps, load ~4) | 12,112 ms | 24,970 ms, 12,503 classes, 17.9 MB | 10,168 ms | 26,931 ms | 11,307 ms | 26,810 ms |
| krit-fir 126 files (3 reps) | 6,570 / 5,379 ms | 10,339 / 8,665 ms, 441 classes, 3.3 MB | **INTERNAL_ERROR** (crash) | 8,731 / 8,573 ms (297 ABI classes) | 3,670 / 3,651 ms | 7,639 / 7,448 ms |

Cold (fresh JVM) first build, which is what a cold daemon pays once: whole-module frontend check 18.1 s vs check + snapshot 28.1–32.6 s at 2,000 files; 21.0 s vs 33.3 s on krit-fir (measured, `cold:` rows). So the snapshot costs **+70–100% of one full check, once per daemon state** (or per re-snapshot). `jvm-abi-gen` is not cheaper (it is the full backend plus a class-builder interceptor). `-Xheader-mode` is about frontend-only cost but skips checkers (so it cannot replace the check) and crashed on real code: not usable on 2.4.20. Since the frontend check is needed anyway for the cold verdicts, the cheapest snapshot is the full compile with the backend: the checkers run inside it, so its findings are the cold verdicts (equal to the frontend-only reference in every cold comparison: `cold vs reference ... verdictDiff=0 strictDiff=0`). A module with any compiler error gets **no snapshot** (the backend never runs): AutoMobile (section 2.5).

### 4.4 Snapshot freshness after a slice (measured)

- **(a) patch**: the slice compile runs the backend too (stock `K2JVMCompiler` phases with the same IC services, `-Xreport-output-files`), and the outputs of the compiled files replace theirs in the snapshot; the module mapping is merged (`Snapshot.patch`). This only works when the slice has no errors; otherwise the outputs are left as they were and the files whose binary is stale stay in the source set (falls back to b').
- **(b') keep surface-stale**: no backend per edit; a file stays in every later slice only while its **surface** differs from the surface its snapshot binary was built from (a body-only edit does not stale the binary for the frontend: other files only see metadata, and inline bodies, `const` values and contracts are part of the surface). Files with compile errors also stay (they have no fresh binary).
- **(b) keep every file edited since the snapshot** (the literal option in the issue): `keepall`.

| corpus | (a) patch | (b') keep surface-stale | (b) keep every edited file | notes |
|---|---|---|---|---|
| synthetic 600, stock pipeline (60/60/40 edits, oracle every edit) | p50 180 ms, 0 stale | p50 166 ms, 0 stale | p50 152 ms, 0 stale | sig/overload edits leave dependents with errors, so most slices have errors and (a) rarely patches |
| synthetic 2,000, stock pipeline (100 edits each) | p50 283 / p95 799 ms | p50 301 / p95 920 ms | | keep set grows to 60–190 files: Gen's signature edits break dependents until toggled back |
| krit-fir 126, (a) stock vs (b') warm | p50 738 / p95 1,471 ms (backend on each clean slice) | p50 505 / p95 1,000 ms (warm env+libs) | | 0 stale in both (60 edits each, oracle every edit) |

Reading: on real code (few errors) (b') is cheaper (no backend, and the keep set only holds files whose surface changed) and avoids mutating the snapshot, which F1+F2 requires anyway (4.5). Re-snapshot (one full compile with backend) when the keep set passes a bound.

### 4.5 F1+F2: the warm environment with a binary snapshot

The first combined runs gave massive staleness ("Unresolved reference" all over, 34/60 edits stale at 600 files, measured): the reused environment's `KotlinCliJavaFileManagerImpl` caches class lookups (`topLevelClassesCache`, including misses, and `binaryCache`), and the stock path's way of hiding stale outputs (moving class files aside for the duration of the compile) and of building the snapshot (writing it after the environment had indexed the empty directory) both change the directory under those caches. Fix: build the snapshot before the warm environment exists, never touch the directory afterwards, and subtract stale outputs from the IC scope instead (`env.getSearchScopeByIoFiles(...)`), which means option (b'). After that: 0 stale verdicts at 600 files (60 edits) and on krit-fir (2 x 60 edits), heap flat.

## 5. Dependents without the Analysis API

### 5.1 Recording: the compiler's own IC trackers

`LookupTracker` (`CommonConfigurationKeys.LOOKUP_TRACKER`, set from `Services` by `JvmConfigurationUpdater`) is what K2 IC uses: `IncrementalPassThroughLookupTrackerComponent` reports every name the resolver looks up in every package or classifier scope it searches, **whether or not the lookup succeeds** (so a later declaration that would shadow or overload is covered), per source file. The spike registers one `CompileLookups` (a `LookupTracker` + `ImportTracker`) per compile through `Services` (stock path) or the configuration the warm phase reads (warm path) and keeps a daemon-side reverse index (`LookupIndex`: file -> {(scope, name)}, and (scope, name) -> files, scope -> files). Cost (measured, synthetic 2,000 files): 165,735 lookup records per full compile, 48,760 distinct (file, symbol) edges; full-compile wall p50 2,876 ms with the tracker vs 2,692 ms without (n = 30 / 20, load 2.3-4.3; within noise, inferred overhead <= ~7%). `ExpectActualTracker` (expect/actual pairs, MPP only), `InlineConstTracker` and `EnumWhenTracker` (Java constants / Java enums used in `when`; JPS) were not needed: the corpora are Kotlin-only and single-platform, and the surface diff below already covers inline bodies and `const` values of Kotlin declarations.

### 5.2 Body-only vs surface change: a FIR declaration-surface diff

After each compile the spike renders the resolved surface of every compiled file from FIR (`Surface.kt`): every non-private declaration keyed by the lookup symbol other files use to reach it ((package, name) for top-level, (class FqName, name) for members), with resolved signatures and inferred types, modality/visibility/flags, annotations with arguments, type parameters and bounds, supertypes, sealed inheritors, constructors (folded into the class header), the bodies of inline functions and inline accessors, `const` initializers, and resolved contracts. Per edit: changed symbols = keys whose hash differs (added, removed or changed); a changed class header adds its whole scope. Dependents = files whose recorded lookups hit a changed symbol, plus the same symbol through every subclass's scope, plus the two structural rules from 4.2. Waves repeat while compiled files' surfaces keep changing (an inferred type propagates), bounded at 8. Body-only and comment edits change no symbol and compile only the edited file.

The previous spike's alternatives were reconsidered: comparing per-file Kotlin metadata/ABI between compiles needs the backend for every compile (option (a) produces it anyway, but it misses inferred-type changes that are not in the edited file's metadata until dependents recompile, and the frontend already has the resolved FIR); the FIR surface needs no backend and is available in the same compile. In krit-fir it would be one more file checker next to `OracleFileChecker`.

### 5.3 Quality (measured; ground truth = files whose fresh full-compile verdict changed)

"Flagged" = dependents the lookup index chose (edited files excluded); "truth" = files whose fresh full-compile diagnostics changed (strict level; verdict level in brackets); "missed" = truth not re-checked. Missed is 0 in every row, so flagged ⊇ truth on each edit, and equal averages mean flagged = truth on each edit.

| corpus, run | kind (n) | flagged avg | truth avg (verdict) | missed | krit-fir `DepTracker` would flag (missed) |
|---|---|---|---|---|---|
| synthetic 600, F2 patch / keep, F1+F2 keep (identical in all three) | body (19), comment (1), add (5), shadow (4) | 0 | 0 | 0 | 0 (0) |
| | delete (9) | 0.9 | 0.9 (0.6) | 0 | 0 (8 of 8) |
| | signature (13) | 4.5 | 4.5 (2.0) | 0 | 0 (59 of 59) |
| | new overload (4) | 7.5 | 7.5 (1.8) | 0 | 0 (30 of 30) |
| | sealed subclass (4) | 1.0 | 1.0 (0.3) | 0 | 0 (4 of 4) |
| | hot signature (1) | 447 | 447 (408) | 0 | 0 (447 of 447) |
| krit-fir, F2 patch and F1+F2 keep (seed 31) | literal (27), comment (8), add (5), delete (4) | 0 | 0 | 0 | 0.1 (0) |
| | rename (16) | 2.8 | 2.8 (2.5) | 0 | 0.3 (42 of 45) |
| krit-fir, F1+F2 keep (seed 32) | rename (16) | 1.5 | 1.5 (1.3) | 0 | |
| synthetic 10,000, F1+F2 keep (24 edits) | sampled: 6 edits compared, each on the compiled files + 400 random files | not computed | | 0 stale | |

For comparison (AA spike, measured there): resolved reverse edges (R) flagged 14 files for a lib signature edit and 15 for a new overload, 1-4x the affected files, and the safe lexical variant (S/name) flagged so much that it was slower than a full compile. The compiler's lookups are exact because they record what resolution actually searched, including failed lookups in each scope (which is what makes a new overload or a shadowing declaration visible), and the surface diff stops body-only edits before any lookup.

What the lookup index cannot see (all found and handled, section 4.2): sealed inheritor lists in the parent's binary; redeclarations and conflicting overloads (the other side must be in source to get its error); implementers of a changed abstract member (their own scope never looks the new name up); `expect`/`actual`. And what it is not meant to see: facts a *rule* reads from another declaration's body or non-`const` initializer (section 8), which change without any surface change.

## 6. F3: Kotlin's own incremental compiler (Build Tools API)

### 6.1 Setup (`Bta.kt`)

`kotlin-build-tools-api` 2.4.20 on the harness classpath; `kotlin-build-tools-impl` 2.4.20 (with `kotlin-compiler-embeddable`, `kotlin-compiler-runner`, `kotlin-daemon-client`, ...) in its own `URLClassLoader` over `SharedApiClassesClassLoader()`; `KotlinToolchains.loadImplementation(loader)`, `JvmPlatformToolchain`, one `BuildSession`, in-process execution policy. Classpath snapshots via `classpathSnapshottingOperation`, IC via `snapshotBasedIcConfigurationBuilder(workDir, SourcesChanges.Known(modified, removed), snapshots)`; krit's plugin via `-Xplugin=`; diagnostics through `BaseCompilationOperation.COMPILER_MESSAGE_RENDERER` (severity, location, text -> the same verdict shape as `FindingCollector`); the recompiled file set from the IC reporter's `Compile iteration:` debug lines (`BuildToolsApiBuildICReporter.reportCompileIteration`).

Packaging facts (measured): the impl runs the **embeddable** compiler (`com.intellij` relocated to `org.jetbrains.kotlin.com.intellij`), so krit's plugin jar must be relocated the same way (the spike builds `krit-fir-plugin-embeddable.jar` with a Shadow relocation; the shipped `krit-fir.jar` is compiled against the unshaded compiler and would fail to link). The plugin then lives in the impl's class loader: the host's `FirRuleContext` / `FirRuleErrors` thread-locals are different classes there, so the per-request rule selection, rule configs, test-file classification, scan paths, SDK levels and the rule-isolation recorder do **not** reach the checkers (every rule runs with defaults, a throwing rule crashes the compile). They would have to travel as `-P plugin:dev.jasonpearson.krit.fir:...` options. `JvmSnapshotBasedIncrementalCompilationConfiguration.USE_FIR_RUNNER` (the K2-native IC runner) is `@Deprecated(level = ERROR)` in 2.4.20 ("removed in 2.5.0"), so the default runner (`IncrementalJvmCompilerRunner`, K2 frontend + backend per iteration) is what is measured.

### 6.2 Results

| run | edits | p50 / p95 per edit | correctness vs fresh full compile | notes |
|---|---|---|---|---|
| synthetic 600, full AA edit mix | 40 | 318 / 722 ms | **stale on 40/40 edits**, 3,059 stale file verdicts summed over edits; 463 truly affected files never recompiled | the mix's signature edits leave dependents with errors; Kotlin IC stops at the first iteration with an error and does not propagate further |
| synthetic 600, edits that never introduce errors (body, comment, add, shadow) | 40 | 281 / 457 ms | 0 stale | |
| synthetic 2,000 (timing only) | 40 | 349 / 403 ms | not checked | 0 IC rebuilds; caches 4.9 MB; the backend runs on every edit; cold IC build 27.7 s |
| krit-fir, literal / comment / add | 20 | 610 / 1,137 ms | 0 stale | cold IC build 34.5-45.5 s, caches 5.8-6.0 MB |
| krit-fir, mix with renames | 30 | 804 / 1,366 ms | 13/30 edits stale | a rename breaks the calling file (as in an IDE before the user fixes the call); IC stops at the error and that file's later verdicts stay stale |

So IC's dirty-set logic (ABI snapshot diff + lookup caches) is correct for clean builds and about as fast as F2 with the backend (F2 patch: 738 ms on krit-fir). Its contract is "a successful build after a successful build". A checker daemon must give verdicts for code that does not compile yet, which IC does not handle. With the class-loader isolation (no `FirRuleContext`, no rule-isolation recorder) and the relocated plugin, F3 is not a checker driver. Its role: confirmation that the compiler's own IC components (`IncrementalCompilationComponents`, `LookupTracker`, the class-file snapshot with the module's own module data) are the right building blocks, used directly from the frontend pipeline in F2.

## 7. F4: in-place update of the CLI FIR source session (time-boxed, ~1.5 h used)

`InPlace.kt` keeps the source session of one warm compile and, per edit, (1) re-parses the edited files and their dependents into new raw `FirFile`s in the same session (`buildFirViaLightTree` records them), (2) rebuilds the provider index from the new file list with `FirProviderImpl.ensureConsistent(files)` (public but `@TestOnly`; with the hard-coded `rebuildIndex = true` it re-records every file into a fresh `State` and swaps it in, so deleted and replaced files disappear), (3) replaces the session's `FirCachingCompositeSymbolProvider` (whose class/callable/package caches are permanent; its `createCopyWithCleanCaches()` is restricted to the plugin-generated-declarations case) with a new one over the same providers via `session.register(FirSymbolProvider::class, ...)`, and (4) runs `runResolution` + `runCheckers` on the new files only. Measured:

| run | edits | re-resolved files identical to a fresh full compile | update p50 / p95 | heap after GC |
|---|---|---|---|---|
| synthetic 300, fixed kinds, provider swapped | 11 | 11/11 | 6-116 ms per edit | 54 MB |
| synthetic 300, fixed kinds, provider NOT swapped | 11 | 6/11 (stale return types, `Unresolved reference` for a symbol of a re-parsed file, a sealed `when`) | | |
| synthetic 600, random mix, provider swapped | 60 | 60/60 re-resolved files; 8 edits left 17 cached verdicts stale elsewhere (one-wave name-based dependents, no surface fixpoint) | 539 / 990 ms (dependents by name: ~158 files per signature edit) | 369 -> 706 -> 957 MB at edits 20/40/60 |

On real code it fails (measured): on krit-fir (126 files, clean) **0 of 30 edits** left the module's verdicts identical to a fresh compile. The cause is one file: once `FirRuleMerge.kt` is re-resolved in place (it is a lookup-name dependent of most edits), it gets two false `Return type mismatch: expected 'FirExpressionChecker<E>', actual 'IsolatedExpressionChecker<E>'` errors on generic class-to-interface returns, and keeps them for the rest of the run. Inferred cause: some session cache (supertype or substitution scope keyed by the old class symbols) still serves the previous FIR of re-parsed classes; with no invalidation API there is no supported way to find and clear all of them. Name-based dependents also made each update large on real code (80-104 files re-resolved per literal edit), so the update cost 3.2 s p50 (p95 4.2 s), near a whole-module compile, and heap after GC reached 2.2 GB after 30 edits (RSS 4.6 GB).

| run | edits | identical | update p50 / p95 | heap after GC |
|---|---|---|---|---|
| krit-fir 126 files, provider swapped | 30 | **0/30** (false error in `FirRuleMerge.kt`) | 3,231 / 4,235 ms | 2,164 MB at the end |
| krit-fir with the generated root missing (1 error file) | 40 | 0/40 (same false error) | 3,277 / 3,979 ms | 2,579 MB |
| synthetic 600, random mix, provider NOT swapped | 30 | 0/30 (608 stale verdicts) | 574 / 1,052 ms | 534 MB |

So the CLI session *can* be updated in place on 2.4.20 for the synthetic cases tried, but only through a `@TestOnly` method and a session-component swap, and it is not usable as-is: (a) **memory grows ~15 MB per edit at 600 files** (the old FIR stays reachable from per-session caches keyed by declarations and from the old provider instances: there is no eviction API), so a daemon would have to rebuild the session periodically, which is the cost F4 was meant to avoid; (b) nothing in the CLI session invalidates per-declaration caches (sealed inheritors are recomputed only for the files being resolved; scope and signature caches are keyed by the old declarations), so correctness rests on re-resolving every dependent, the same dirty-set computation F2 needs; (c) it relies on compiler internals the CLI never exercises this way (no tests upstream cover re-resolving files in a live CLI session), so every Kotlin bump is a fresh risk. Blockers for production: no supported removal/replacement API on `FirProviderImpl` (only `@TestOnly ensureConsistent`), permanent caches in `FirCachingCompositeSymbolProvider` and other session components with no invalidation hooks, and unbounded retention of replaced FIR. Recommendation: do not pursue; F2 gets the same per-edit cost class with supported entry points.

## 8. Rules that read facts of other files (the main correctness risk of F2 and F3)

### 8.1 By experiment: the split differential (`RuleDiff.kt`, measured)

Corpus: krit-fir's compiler-test data (`compiler-tests/src/test/data/diagnostic/**`, 718 rule scenarios written against the shared Android/AndroidX/Kotlin stubs, which the harness compiles once into a stub library; Java stubs as Java source roots). Each scenario with at least two top-level declarations (581) is split into one file per declaration (`private` -> `internal`, diagnostic markers stripped), compiled in full (frontend only: reference) and with the backend (snapshot); every piece (4,223) is then compiled alone through the stock IC path against the snapshot of the others, and its verdict compared with the reference. 77 of krit's rules produced findings in the split corpus, so nearly the whole rule set was exercised. The compiler-error gate never differed. 26 pieces differed, in 11 rules:

| rule | findings that differ | direction | the fact it reads from another declaration (from the source) |
|---|---|---|---|
| HardcodedSecretKey | 18 | full-only (missed on a slice) | `resolvedInitializer` of a non-local, non-const property declared elsewhere (key bytes in a top-level/object `val`) |
| ShortAlarm | 9 | slice-only (false positives) | `function.fir.body` of a project wrapper function: no body in a binary, and the rule treats "no body" as "may set the interval" |
| SharedFlowWithoutReplay | 6 | full-only | `function.fir.body` of a factory function in another file |
| StaticIv | 5 | full-only | `resolvedInitializer` of an IV byte-array property declared elsewhere |
| LongLogTag | 4 | full-only | `resolvedInitializer` of a non-const tag `val` and `getter.fir.body` (const tags work: they are in the metadata) |
| CollectionsSynchronizedListIteration | 3 | full-only | `resolvedInitializer` of a property holding the synchronized wrapper |
| SerialVersionUIDInSerializableClass | 3 | full-only | `session.firProvider.getFirFilesByPackage` (scans the compilation's files; already flagged `requiresWholeCompilation` on branch `-862`) |
| LocaleGetDefaultForFormatting | 2 | one full-only, one slice-only | `function.fir.body` single-return of a helper / `resolvedInitializer` of a top-level `val` |
| WorldWriteableFiles | 2 | slice-only | getter `fir.body` / `resolvedInitializer` of a mode constant holder that is not `const` |
| JdbcStatementExecute | 1 | slice-only | `resolvedInitializer` of a property used to decide "static SQL" |
| WeakKeySize | 1 | full-only | `resolvedInitializer` of a top-level `val` key size |

Every one of these has two consequences: (1) on a slice the rule's verdict for the file is wrong when the declaration it reads comes from the binary snapshot (both false negatives and false positives occur), and (2) its verdict depends on a **non-surface** fact of another file, so even with every dependency in source, a body-only edit of the other file (no surface change, no lookup-index dependents) changes the verdict: the dirty-set computation of section 5 cannot see it.

### 8.2 By reading the checkers (grep for `resolvedInitializer`, `.fir.body`, `.fir.initializer`, `symbol.source`, `firProvider`, `getContainingFile`)

Candidates that the differential did not flag, either because the read is guarded to local symbols (`isLocal`) or because the corpus has no cross-declaration case: SecureRandom, ShowToast, GetSignatures, Wakelock, GlobalScopeLaunchInViewModel (initializer reads, mostly local or const-guarded); BufferedReadWithoutBuffer (`isLocal` only); TooGenericExceptionThrown (`symbol.source` decides whether an exception class is declared in Kotlin source of this compilation, so a project exception class coming from the snapshot is treated like a library class: an origin check, not a body read; not exercised by a split case); IntoMapMissingKey (`resolvedAnnotationClassIds`: annotations are in metadata, except `SOURCE`-retention ones). Oracle facts showed the same `SOURCE`-retention effect: a callee's `@OptIn` annotation is absent when the callee is binary (section 9).

### 8.3 What to do with them

- Tag them (`readsOtherDeclarationBodies`, or extend `requiresWholeCompilation`) with a lint gate that greps for the patterns above on non-local symbols, as branch `-862` does for `firProvider`.
- On the incremental tier, either run them only in full compiles, or keep their verdicts from the last full compile and invalidate them by **body-level** dependencies: the lookup index already knows which files reference which declarations; for these rules a body edit (not just a surface edit) of a referenced declaration must re-run the full tier, or the slice must include the declaring files of everything the rule followed (unbounded in principle: interprocedural walks).
- F3 (IC) has exactly the same problem plus the class-loader isolation, so it does not escape it.

## 9. Oracle facts (#859) under a slice (measured, `OracleFacts.kt`)

#859 made `analyzeWithCheck` run krit-fir's `OracleCollector` (declarations, expression types, diagnostics, lambda flags, `DepTracker` supertype edges) in the same compile as the checkers. The spike registers the real `OracleCollectorRegistry` in a full compile of krit-fir (126 files, 0 errors) and in single-file slice compiles of 30 sampled files against the full compile's snapshot, and compares each file's payload.

| payload part | identical in slice vs full | difference |
|---|---|---|
| declarations | 30/30 | |
| diagnostics | 30/30 | |
| lambda flags | 30/30 | |
| expression types and call targets | 28/30 | `annotations=[kotlin.OptIn]` on a call target is empty when the callee comes from the snapshot: `@OptIn` has `SOURCE` retention and is not in class files (`SecureRandom.kt`, `StaticIv.kt`, calls to `assignedLocalValues`) |
| `DepTracker` dependency paths | 4/30 | the full compile records 82 files with supertype edges; in a slice, supertypes declared in other files are `Precompiled` (from the snapshot), and `DepTracker` records only `Source` origins, so the edges disappear |

Per strategy:
- **F2 / F1+F2**: as in the table. The fix for dependency paths is small: record `Precompiled` supertypes too and map their class files back to source paths through the snapshot's output->source map (`-Xreport-output-files`). The `@OptIn` gap affects any consumer that reads `SOURCE`-retention annotations of other declarations; it should be documented on the payload (or such calls marked as binary-backed).
- **F1** (whole module per compile): identical payloads (same compile, only the library sessions are reused).
- **F3**: the oracle collector would run in the BTA class loader, unreachable from the daemon's request state; not usable for #859 without a new transport.
- **F4**: payloads of re-resolved files inherit F4's false errors on real code.

## 10. Integration design

### 10.1 Where it plugs in

The incremental runner lives inside the krit-fir daemon, behind `AnalysisSession.check` / `analyzeWithCheck`, and keeps `compileModule` (the shipped `FrontendOnlyJvmPipeline` / `K2JVMCompiler` seam) as the full-compile tier and the fallback. No protocol change is required for the first version: `CheckRequest.files` already carries `FileRef(path, contentHash)`, so the daemon can tell which files changed since its last compile of the same session.

```
Go (unchanged first)                         krit-fir daemon, per AnalysisSession (one module)
--------------------                         --------------------------------------------------
check(files+hashes, rules, configs, facts) -> IncrementalState.plan(request)
                                               | same invocation fingerprint as the state?   no -> full tier, rebuild state
                                               | changed = files whose hash differs; deleted = files gone
                                               | dirty  = changed + LookupIndex(changed surfaces) + structural (sealed parents,
                                               |          same-name declarations) + surface-stale files kept in source
                                               | gates: |dirty| <= threshold, waves <= 8, no expect/actual, snapshot exists,
                                               |        no enabled rule that needs the whole compilation / other files' bodies
                                               v
                                             slice compile: stock FrontendOnlyJvmPipeline, freeArgs = dirty,
                                               Services { IncrementalCompilationComponentsWithCustomScope(snapshot),
                                                          LookupTracker, ImportTracker }, -Xenable-incremental-compilation,
                                               same -module-name, snapshot dir on the classpath
                                             (later: the warm frontend phase, F1+F2)
                                               | per-file verdicts for the compiled files (FindingCollector, recorder armed)
                                               | lookups -> LookupIndex; FIR surfaces (krit file checker) -> surface map
                                               | surface changed -> next wave over the new dependents
                                               v
                                             BatchResult for all requested files = fresh verdicts (compiled) + daemon
                                             verdict cache (others); errorFiles / ruleErrors per file as today
```

State per `AnalysisSession` (module): binary snapshot directory (class files + `META-INF/<module>.kotlin_module` + source->outputs map), `LookupIndex` (file -> lookup symbols, reverse map), per-file declaration surface hashes, per-file last verdict (findings, error gate, rule errors), the set of surface-stale files kept in source, and the invocation fingerprint the state was built under.

### 10.2 Cache key and invalidation

- **Daemon state key** = today's `FirInvocationFingerprint` inputs (classpath with content identity, krit-fir jar, rules, rule configs, test-file classification, scan paths, SDK levels, JVM target, Kotlin settings, module name). Any change discards the state (snapshot included) and runs the full tier, which rebuilds it.
- **Per-file validity inside the daemon**: a file's cached verdict is valid unless it is in the dirty set of some edit since it was computed. The dirty set is exact for everything the compiler resolves (LookupTracker), and conservative for structural effects (sealed parents, redeclarations).
- **Go-side cache** (`FirCacheEntry` v7, `CheckCacheFingerprint` = whole compilation): unchanged in the first PRs. Every edit still misses Go's cache, but the miss is served by the daemon's incremental state in ~0.1-0.3 s instead of a whole-module compile. A later PR can persist the daemon state (snapshot dir + lookup index + surfaces, keyed by the invocation fingerprint) under `.krit-cache` so a fresh daemon starts warm, and can move Go to per-file keys (path + content + invocation fingerprint + dirty-set generation) as the AA spike proposed.

### 10.3 Fallback (full compile tier)

Any of: no state yet (cold daemon) or the invocation fingerprint changed; the snapshot could not be built (the module has compiler errors, so the backend never ran: AutoMobile without a classpath is entirely in this state); dirty set above the threshold (~10-20% of the module, adaptive by measured ms/file); wave limit; an exception that rule isolation does not absorb; module uses expect/actual (MPP fragments) or scripts; an enabled rule is tiered `requiresWholeCompilation` (SerialVersionUIDInSerializableClass) and the edit can change its inputs; periodic self-check sample (compare a slice verdict with the full tier every N requests, disable incremental mode for the session on mismatch). The full tier also refreshes the snapshot when the stale-in-source set grows past a bound (re-snapshot cost = one backend compile).

### 10.4 Interaction with #859 (shared oracle/checker compile)

`analyzeWithCheck` runs the oracle collector and the checkers in one compile. A slice compile registers the same collector, so the oracle payload for the files in the slice comes out of the same compile (measured in section 9). Two adaptations: (1) DepTracker records supertype edges only for `FirDeclarationOrigin.Source`; in a slice, supertypes declared in other files are `Precompiled` (from the snapshot), so the edges must be recorded for that origin too and mapped back to source paths through the snapshot's output->source map; (2) the oracle's own cache (`analyzeWithDeps` closure) can keep its keys, since it is fed the same per-file payloads. The shared-compile condition in `analyzeWithCheck` ("both requests compile the same sources") becomes "both requests are served from the same incremental state".

### 10.5 Rule tiering

Rules whose verdict for a file reads facts of *other* files that binaries do not carry (bodies, non-const initializers, `symbol.source`, the compilation's file list) cannot be evaluated on a slice. Section 8 lists them. Two flags on `FirRule`, both with a source lint gate like the one on branch `-862`:
- `requiresWholeCompilation` (exists on branch `-862`): iterates the compilation's files. Evaluated only on the full tier.
- `readsOtherDeclarationBodies` (new): follows resolved symbols into other declarations' bodies, initializers or source. On the incremental tier such a rule runs only when every declaration it can reach is in source, which in practice means: run it on the full tier, or keep its results from the last full compile and re-run the full tier when any file it read from changes (needs body-level lookups, not just surface).

## 11. PR-sized implementation plan

Each step ships behind `KRIT_FIR_INCREMENTAL` (off by default) until step 7; the full-compile path stays the default and the fallback throughout.

1. **Rule tiering (krit-fir, no behaviour change).** Land `FirRule.requiresWholeCompilation` (branch `-862`) and add `readsOtherDeclarationBodies` for the 10 other rules of section 8, with a source lint gate (`resolvedInitializer`, `.fir.body`, `.fir.initializer`, `symbol.source`, `getContainingFile` on symbols that are not `isLocal`). Test: the split differential (`RuleDiff.kt`) as a krit-fir test over the compiler-test data, asserting that only tagged rules differ.
2. **Lookup recording on the existing compile (krit-fir).** Pass a `LookupTracker` (+ `ImportTracker`) through `Services` in `AnalysisSession.compileModule` and add a declaration-surface file checker next to `OracleFileChecker` (port of `Surface.kt`). Keep both in an `IncrementalState` per `AnalysisSession`. Tests: surface hashes stable across identical compiles, changed on each kind of signature edit, unchanged on body/comment edits; lookups recorded for every requested file.
3. **Binary snapshot + slice compile (krit-fir).** On a clean full compile, run the backend into the state's snapshot directory with `-Xreport-output-files` (source -> outputs). `SnapshotComponents` (`IncrementalCompilationComponentsWithCustomScope` + `IncrementalCache` over the module mapping) and the slice compile through the unchanged `FrontendOnlyJvmPipeline` with `-Xenable-incremental-compilation`, the same `-module-name`, the snapshot on the classpath, stale outputs moved aside (stock path). Tests: the `Shapes.kt` constructs as unit tests (internal, facades, multifile, sealed both ways, inline/const, redeclaration, overloads, expect/actual -> fallback).
4. **Planner + verdict cache (krit-fir).** Dirty set = changed files (by `FileRef.contentHash`) + lookup dependents of changed surface symbols + structural rules (supertype files, same-name declarations, subclass files) + surface-stale keep set; waves; threshold; fallback rules of 10.3; per-file verdict cache in the state; `BatchResult` assembled from fresh + cached verdicts. Tests: the `Inc.kt` harness in CI-sized form (600 synthetic files, 40 edits, oracle every edit, assert 0 stale) plus a real-code run (krit-fir sources, rename edits).
5. **#859 integration.** `analyzeWithCheck` served from the same state; DepTracker records `Precompiled` supertypes and maps them to source files through the snapshot map; tests from `OracleFacts.kt`.
6. **Self-check and telemetry.** `KRIT_FIR_INCREMENTAL_VERIFY=N`: every Nth incremental request also runs the full tier and compares verdicts; mismatch disables the state for that session and logs both. `-perf` counters: dirty-set size, waves, keep-set size, fallbacks, snapshot age.
7. **Enable by default for the daemon path**, after a soak on the playground and a real corpus.
8. **F1+F2 (optional, measured 30-60% less per edit than the stock slice).** The warm frontend phase (port of `WarmFrontendPhase` with the four fixes of 3.2), immutable snapshot with scope subtraction. Only after a Kotlin-bump CI job runs the harness on every compiler upgrade, because it replaces a compiler phase.
9. **Persisted state (optional).** Store snapshot + lookup index + surfaces + verdicts under `.krit-cache/fir-incremental/<invocation fingerprint>/` so a new daemon starts warm; Go-side per-file cache keys after that.

## 12. Risks

| risk | likelihood / impact | mitigation |
|---|---|---|
| A rule reads another declaration's body, initializer, source or the file list: wrong on a slice, and its verdict depends on non-surface facts | measured: 11 rules today / high (false negatives and false positives) | tiering flags + lint gate + differential test (step 1); such rules run on the full tier |
| Dependents miss a case lookups cannot see (sealed inheritors, redeclaration, implementers of abstract members, expect/actual, ...) | each found once by probe or real code; all covered now / high | structural rules; self-check sampling; fallback on MPP |
| The module has compile errors, so there is no snapshot | certain without a resolvable classpath (AutoMobile) / medium (no speed-up, never wrong) | fall back to the full tier (or F1); report it in `-perf` |
| Compiler internals change (IC interfaces, session factories, phases) | IC interfaces have been stable for IntelliJ/JPS; F1's phase port is fragile / medium | F2 first (only IC interfaces + `FrontendOnlyJvmPipeline`); CI harness job on each Kotlin bump |
| Snapshot staleness bugs (outputs of a file visible together with its source; stale sealed-inheritor metadata) | low after the rules above / high | outputs parked or scope-subtracted per compile; keep-set rule; re-snapshot bound |
| Memory: F1 retention bugs (found two) | measured flat after fixes / high if regressed | heap-after-GC watchdog in the daemon; recycle the state above a budget |
| `SOURCE`-retention annotations and other source-only facts missing from binaries (oracle `@OptIn`, rules reading annotations) | measured in oracle payloads / low-medium | tier rules that read callee annotations; oracle payload marks binary-origin targets |
| Threshold mis-tuned: large dirty sets slower than a full compile | the hot-file edit (500 dependents) fell back as designed / low | adaptive threshold from measured ms/file |

## Appendix A. Measured vs inferred

Measured (this spike, logs under the scratch `f740/logs/`):
- Every p50/p95/CPU number, cold first-build time, heap/RSS series, and stale/identical count in sections 2-7, with n and load as listed.
- The four F1 fixes and the leak chain (3.2): leak growth before and after, retention chain from `LeakFinder.kt`.
- Construct behaviour (4.2), both with IC binaries and with the friend-library variant.
- Snapshot cost per kind (4.3), including `-Xheader-mode` crashing on krit-fir and skipping checkers.
- Lookup volume, index size and dependents quality against ground truth (5.1, 5.3); `DepTracker` coverage.
- F3 packaging facts (relocation needed, `USE_FIR_RUNNER` deprecated at ERROR, thread-locals not visible) and its stale-on-error behaviour.
- F4 results, including the krit-fir false errors.
- The 11 rules of 8.1 and their per-rule finding differences (split differential), and that the compiler-error gate never differed.
- Oracle payload equality per part (9).

Inferred (reasoned from source or from the measurements, not run):
- `LookupTracker` overhead "<= ~7%": the with/without difference (2,876 vs 2,692 ms) is inside the run-to-run noise at those loads.
- That the 8.2 candidates (SecureRandom, ShowToast, GetSignatures, Wakelock, GlobalScopeLaunchInViewModel, BufferedReadWithoutBuffer, TooGenericExceptionThrown, IntoMapMissingKey) are safe or unsafe: read from checker source, not exercised by a split case.
- The cause of F4's false errors on krit-fir (stale per-session caches).
- Stability of the entry points across Kotlin releases (3.1, 4.1): from comparing 2.x compiler sources and the IC interfaces' use by IntelliJ/JPS, not from running other versions.
- The adaptive threshold, persisted state, self-check sampling, and every integration and plan item (sections 10-12): design, not built.
- That a Gradle-provided classpath or a lenient snapshot would make F2 usable on AutoMobile (2.5).
- That `ExpectActualTracker` / `InlineConstTracker` / `EnumWhenTracker` are not needed for single-platform Kotlin-only modules: Java sources in a module would need `InlineConstTracker` / `EnumWhenTracker`-style edges (Java constants), not tested.
- Costs for a multi-module project (each `AnalysisSession` module gets its own state): not run; the corpora are single-module as `AnalysisSession` compiles them today.

Not done:
- No measurement with Java sources in the module (krit-fir's compiler tests use Java stubs as Java roots only in the rule differential).
- No run on a module with `expect`/`actual` beyond the construct probe.
- The 10,000-file correctness run sampled 400 files every 4th edit (24 edits, 6 compared: every 4th edit, on the compiled files plus 400 random files), not every file on every edit.
- F4 got about 1.5 h of the 2 h time box; it stopped at the krit-fir failure.

## Appendix B. Reproducing

```
cd tools/krit-fir
./gradlew spikeClasspath   # compiles the spike source set, builds the plugin jars, writes build/spike-classpath.txt, build/spike-bta-classpath.txt, build/spike-env.txt
java -Xmx4g $(cat build/spike-env.txt) -cp "$(cat build/spike-classpath.txt)" spike.BenchKt <mode> [--key=value ...]
```

Modes (`Bench.kt`): `smoke`; `inc --strategy=cli|warm|slice|warmslice --snap=patch|keep|keepall --files=N | --real=<roots,> [--cp=<jars:>] [--kotlinArgs=...] --edits=N --warmup=N --oracle=N [--oracleSample=N] [--deptracker=true] [--threshold=N] [--seed=N]`; `snapcost`; `shapes [--friend=true]`; `bta`; `inplace`; `rulediff`; `oraclefacts --sample=N`; `leak`. Timing runs were serialized with `flock /tmp/claude-0/krit-bench.lock`; JVM `-Xmx4g` (6 GB at 10,000 files).
