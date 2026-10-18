# Incremental FIR checkers (#740): steady-state benchmark, strategy C re-check, dependents, design

Branch: `claude/krit-issues-iteration-79gicw-740-spike` (spike only, not for merging). Harness: `tools/krit-fir/src/spike/kotlin/spike/`, driven by the `spikeClasspath` / `benchSpike` Gradle tasks. Nothing under `src/main` changed. Outside the `spike` source set the branch edits `tools/krit-fir/build.gradle.kts` (spike source set, tasks, Analysis API dependencies) and `tools/krit-fir/settings.gradle.kts` (repository entry for the unshrunk IntelliJ core, see section 6).

Every number is labelled **measured** (this spike, this machine) or **inferred** (reasoned, not run). Section 8 lists both in one place.

## 0. Summary

1. **Strategy B is stable in steady state but not the win the first spike suggested.** 150 consecutive edits in one warm JVM show no heap, thread or FD growth (2,000 files: 118-134 MB after GC the whole run; 10,000 files: 385-405 MB); class count and metaspace creep slowly and flatten (+250 classes / +6 MB over 150 edits). Latency after warm-up on 2,000 files: p50 ~480 ms, p95 ~965 ms (low load); on 10,000 files p50 ~1.4-1.9 s. Reusing the JDK/stdlib library modules (B) is **not measurably faster than rebuilding everything (A)**: A p50 467 ms vs B p50 479 ms at similar load. B's cost is re-indexing every source file (~0.13-0.15 ms/file), so it scales with module size, not with the edit.
2. **Two designs that use only supported Analysis API surface make cost proportional to the edit** (3-10x over B, 15-90x over the CLI recompile at 10,000 files):
   - **D, dependency-slice session**: build a fresh session over only the edited files, their dependents and the forward closure of the names they mention. Build is flat at 10-26 ms for 2,000 and 10,000 files; typical edit p50 ~150 ms at both sizes (CLI recompile: 2.4 s and 13.4 s).
   - **S/R, D plus a resolved-surface gate**: after checking the edited file, render its resolved declaration surface (inferred types included); only a surface change pulls in dependents, and the check repeats as a fixpoint. Body-only and comment edits then cost one file. **R** takes dependents from resolved reference edges (krit-types' `DepEdges` recorder, run inside the session) plus lexical lookup only for new top-level names. R: p50 209 ms (2,000 files), 235 ms (10,000 files), 445 ms (real 123-file project).
3. **Strategy C (update a session in place) is still not available on Kotlin 2.4.20.** The modification API exists (`KaSourceModificationService`, module out-of-block events, event-driven `KotlinModificationTrackerFactory`) but nothing lets a built standalone session take new file content: events leave the old PSI in place, and every in-place PSI mutation I tried fails (read-only file, then the stub/AST invalidation assertion). **A supported-API substitute exists for single-file, surface-preserving edits: analyze the edited file as a dangling copy in a long-lived session (E).** p50 49 ms (idle, 2,000 files, verified identical to a fresh check) and 200-350 ms on the real projects, but on real code it produced false compiler errors in 5 of 24 stress edits, so it is not safe yet.
4. **Dependents: lexical name lookup alone is either unsafe or imprecise; resolved edges fix both.** Owner-narrowed lexical lookup (precise) missed 2 real dependents on the real project in both runs that used seed 32 (D: 40 edits, S/owner: 60 edits; a member reached through an inferred receiver type), leaving 11-15 later edits with a stale verdict; name-only lookup (safe) flagged 440+ of 600 files per signature edit on the synthetic project and 7-8x the truly affected files on real renames; resolved reverse edges (R) flagged 1-4.4x the affected files with **zero misses** in every run (full-oracle and sampled). Details and the table of runs are in section 4.
5. **The incremental runner has semantic gaps that must be designed in.** `SerialVersionUIDInSerializableClass` scans every FIR file of the compilation (`firProvider.getFirFilesByPackage`), which throws `Should not be called in FIR IDE` under the Analysis API and makes its verdict depend on the whole project: it must stay on the full-compile tier. Rule isolation (`FirRuleErrorRecorder`) must be armed, or one throwing rule aborts the whole file.
6. **Large dependent sets are slower than the CLI.** AA costs 15-27 ms per checked file on synthetic code and ~125 ms per file on the real 637-file project. Modelled break-even against the existing K2JVMCompiler path: ~90 files at 2,000 files, ~500 at 10,000 files, ~135 on AutoMobile, ~40 on the 123-file krit-fir sources. The runner needs a hard fallback to the full compile above a threshold.
7. **Oracle facts come out of the same session**: declarations, expression types and lambda flags from an AA check equal `AnalysisSession.analyze` on 12/12 sampled files on both a synthetic and the real krit-fir project, so the runner can serve #810's shared request rather than compete with it.

## 1. Method and environment

- Machine: 4 vCPU, 15 GB, JDK 21, Kotlin 2.4.20 libraries (what `tools/krit-fir` bundles; the Gradle plugin is 2.4.21). The container was shared with other lanes; the 1-minute load average during runs ranged from about 1 to 25. All timing runs were under `flock /tmp/claude-0/krit-bench.lock`, which serializes benchmark jobs but not builds or tests, so I record the load for every table row and report **process CPU time** where it helps (it is much less sensitive to load). Compare rows only at similar load.
- Runner: standalone Analysis API session (`buildStandaloneAnalysisAPISession`) with krit's real `KritPluginRegistrar`, all 79 FIR rules, `collectDiagnostics(ONLY_COMMON_CHECKERS)` per file. Edits are applied to disk, as a watcher would deliver them. "Total" latency = re-reading changed files + dependents lookup + session build + check (+ surface rendering and edge recording for S/R). Disposing the old session (0-11 ms) is reported by the harness but excluded.
- Synthetic projects: two modules, `lib` (25% of files) and `app` (75%); each app file references three lib files and the previous app file (so inferred types propagate), and the first 500 app files reference one "hot" lib file. Edit mix: body 35%, lib signature (return type Int<->Long) 20%, new overload 8%, add file 10%, delete file 10% (half delete a file that others import), shadow (add a same-package `println`, the #826 pattern) 6%, new sealed subclass 5%, comment 4%, hot-file signature change (500 dependents) 2%. Names repeat across files on purpose (`derived`, `use`, `body`, `log` exist in every app object), as in real code.
- Real projects: (a) `tools/krit-fir/src/main` + `tools/krit-rule-api` (123 files, 25.4k lines, resolved against the real kotlin-compiler classpath); (b) a shallow clone of `kaeawc/auto-mobile`, 637 main-source files from 7 Android/JVM modules (147k lines). (b) has no Gradle classpath: Compose/Android/most third-party references are unresolved (the CLI flags 365 of 637 files as error files), so it stresses shapes and scale, not a clean build. Real-project edits are text mutations on a scratch copy: a string literal change (body-only), an appended comment, a rename of a function another file mentions (signature edit that breaks callers; toggled back later), add/delete of a trivial file.
- Warm-up: 15 edits discarded on 2,000-file runs, 10 on 10,000-file and real-project runs. One JVM per run. The first 50 files checked in a fresh JVM take ~10 s (class loading, JIT); a daemon pays that once.

## 2. Goal 1: steady-state benchmark

### 2.1 Strategies

| id | per edit | status |
|---|---|---|
| CLI | shipped path: in-process K2JVMCompiler frontend-only compile of the whole module (`AnalysisSession.check`), requesting only the affected files | measured baseline |
| A | new AA session over every source file (JDK, stdlib, sources all rebuilt); check edited + dependents | measured |
| B | as A, reusing the JDK/stdlib `KaModule` objects of the first session (the prior spike's B) | measured |
| D | new AA session over a **slice**: edited files, dependents and the transitive forward closure of top-level names they mention (+ files declaring subtypes of any sealed type pulled in). Dependents: lexical, scope-aware, owner-narrowed, with an "inferred type may leak" closure | measured |
| S | D's slice, but dependents are looked up only when the edited file's **resolved surface** changed, then repeated as a fixpoint over the dependents' own surfaces. Lexical variants: S/owner (members matched only when the owner is mentioned: precise, unsafe) and S/name (members matched by name: safe, imprecise) | measured |
| R | S's gate, but dependents come from **resolved reverse edges** (krit-types `DepEdges` recorder run in the session) + lexical lookup only for new top-level names + files whose last verdict held `Unresolved reference 'name'` for new member names | measured |
| E | **one long-lived session over the whole module**; the edited file is analyzed as a dangling copy (`PsiFileImpl.originalFile = session PSI`) | measured, not safe yet |

### 2.2 Synthetic, 2,000 files (1,500 app + 500 lib), after warm-up

Columns: n = measured edits; load = 1-min load average during the measured edits; total/CPU in ms.

| strategy | n | load | total p50 | total p95 | CPU p50 | build p50 | check p50 |
|---|---|---|---|---|---|---|---|
| CLI | 30 | 4-9 | 2,356 | 2,692 | n/a | n/a | n/a |
| A | 60 | 4.5-5.4 | 467 | 977 | 800 | 315 | 155 |
| B, run 1 | 135 | 13-17 | 718 | 1,731 | 750 | 470 | 239 |
| B, run 2 | 100 | 2.5-3.5 | 479 | 965 | 770 | 302 | 150 |
| B, run 3 | 100 | 2.4-6.2 | 518 | 961 | 740 | 339 | 171 |
| D | 135 | 5.4-6.4 | 156 | 579 | 340 | 13 | 141 |
| S/owner | 135 | 7.9-9.6 | 180 | 853 | 320 | 17 | 160 |
| S/name | 135 | 1.7-9.2 | 13,073 | 16,204 | 14,940 | 240 | 12,732 |
| R | 135 | 3.0-4.1 | 209 | 922 | 340 | 16 | 135 |
| E (verified against a fresh check, 0 mismatches) | 30 | 0.8 | 49 | 66 | n/a | 0 | 49 |
| E (timing only, not verified) | 60 | ~6 | 88 | 197 | n/a | 0 | 88 |

Per edit kind, p50 total ms (B = run 2; CLI per kind from its 30 edits):

| kind | CLI | B | D | S/owner | R |
|---|---|---|---|---|---|
| body | 2,419 | 462 | 135 | 156 | 121 |
| comment | | 443 | 103 | 127 | 92 |
| add file | 2,243 | 468 | 132 | 162 | 202 |
| delete file | 2,297 | 430 | 148 | 169 | 220 |
| lib signature (R flags 14, S/owner 6) | 2,322 | 627 | 273 | 323 | 657 |
| new overload (R flags 15, S/owner 6) | 2,256 | 561 | 243 | 284 | 655 |
| sealed subclass | 2,115 | 336 | 50 | 73 | 70 |
| shadow `println` (1-3 files) | 2,353 | 431 | 113 | 125 | 143 |
| hot signature change (500 dependents; one sample per run) | 2,642 | 12,698 | 13,487 | 14,111 | 11,026 |

Reading: B and A are within noise; B's `build` is 300-470 ms regardless of the edit. D/S/R build is 10-26 ms. The check costs ~100-120 ms for the first file of a fresh session (library symbols, checker set-up) and 15-27 ms for each further file (the 500-file edit: 10-14 s wall, 25-31 s CPU). S/name shows what a safe but name-only member lookup does when member names repeat: add/delete/overload/signature edits each flagged **1,492** of 2,000 files.

### 2.3 Synthetic, 10,000 files (7,500 app + 2,500 lib)

| strategy | n | load | total p50 | total p95 | CPU p50 | build p50 | check p50 |
|---|---|---|---|---|---|---|---|
| CLI | 16 | 2-4 | 13,423 | 21,678 | n/a | n/a | n/a |
| B | 60 | ~10 | 1,868 | 3,253 | 2,340 | 1,540 | 242 |
| B (second run) | 30 | 2.0-2.7 | 1,656 | 2,216 | 2,470 | 1,399 | 259 |
| D | 60 | 5.7-6.0 | 166 | 608 | 430 | 14 | 151 |
| D (second run) | 30 | 3.8-4.4 | 202 | 654 | 510 | 15 | 179 |
| S/owner | 60 | 4.6-6.5 | 151 | 535 | 380 | 12 | 137 |
| R | 60 | 6.4-7.0 | 235 | 1855 | 380 | 26 | 208 |

(An earlier D pass at load 2, before package scoping existed in the dependents lookup, had p50 104 ms but a p95 of 9.1 s: the `println` shadow edit flagged 664 files by name alone. B at load 2 on that pass: p50 1,431 ms, build 1,260 ms.) B's build scales with module size (~0.15 ms/file: 300 ms at 2,000, 1.3-1.5 s at 10,000); D/S/R builds do not. The 500-dependent edit costs 13-17 s at 10,000 files, about the same as the CLI recompile of the whole module.

### 2.4 Real projects

krit-fir main + krit-rule-api (123 files, real classpath), edits: literal/comment/rename/add/delete:

| strategy | n | load | total p50 | total p95 | build p50 | check p50 | notes |
|---|---|---|---|---|---|---|---|
| CLI | 24 | ~10 | 4,368 | 7,125 | | | cold full check 57.6 s (loaded) |
| B | 70 | 4.5-5.3 | 675 | 5,126 | 274 | 395 | literal 634, comment 694, add 268, rename 1,223 |
| D | 70 | ~5 | 561 | 6,361 | 117 | 419 | |
| S/owner | 70 | 2.6-4.1 | 428 | 4,980 | 104 | 313 | literal 385, rename 1,111 |
| S/name | 70 | 3.3-5.4 | 460 | 1,974 | 109 | 324 | literal 453, rename 1,103 |
| R | 70 | 2.9-4.6 | 445 | 1,917 | 114 | 311 | literal 398, comment 445, add 122, rename 1,220 (p95 7.6 s) |
| E (40 literal edits, timing only) | 40 | ~2 | 347 | 1,524 | 0 | 347 | |

AutoMobile main sources (637 files, 147k lines, no real classpath); `SerialVersionUIDInSerializableClass` excluded for the AA rows because it throws under AA (section 6); 365 of 637 files have compiler errors in every strategy:

| strategy | n | load | total p50 | total p95 | build p50 | per-kind p50 |
|---|---|---|---|---|---|---|
| CLI (all rules) | 12 | ~10 | 18,677 | 21,988 | | literal 18.8 s, rename 18.7 s, every kind 17-24 s |
| B | 30 | 2-5 | 1,474 | 57,568 | 1,188 | literal 1,474, comment 1,304, rename 55,826 (434 files flagged) |
| D | 30 | 1.4-5.4 | 1,374 | 57,891 | 1,184 | slice is most of the project; rename 56,549 |
| S/owner | 40 | 1.3-3.4 | 1,405 | 58,001 | 1,190 | rename 28,597 (198 files flagged) |
| R | 40 | 1.8-2.1 | 1,632 | 23,930 | 1,324 | literal 1,632, comment 1,284, rename 7,735 (6 files flagged, p95 26.8 s) |
| E (30 literal edits, timing only, not verified) | 30 | ~2.5 | 200 | 1,094 | 0 | |

Real-project lessons: the forward closure of one file is nearly the whole project when names are shared (D degenerates into B, build ~1.2 s); a rename in a popular class flags hundreds of files whose checks cost ~125 ms each, which is 3x worse than the CLI (18.7 s); resolved edges cut that rename from 434 to 6 files and 56 s to 7.7 s. A long-lived session (E) is what removes the 1.2 s per-edit rebuild on large modules.

### 2.5 Memory and leaks (measured)

Forced GC every 10-15 edits; sessions rebuilt per edit, old one disposed after the new one is built (peak = two sessions). 150 edits at 2,000 files, 70 at 10,000.

| run | heap after GC (MB) | metaspace (MB) | loaded classes | threads | open FDs | peak heap (MB) | peak RSS (VmHWM) |
|---|---|---|---|---|---|---|---|
| B 2,000 (115 edits) | 118-134, flat | 75-82 | 12.9k-13.0k | 6 | 34 | 1,322 | 1.7 GB |
| S/owner 2,000 (150) | 79-96, flat | 77-85 | 13.3k-13.4k | 6 | 34 | 849 | 1.36 GB |
| R 2,000 (150) | 149-165, flat | 82-87 | | 6 | 34 | 1,229 | 1.76 GB |
| B 10,000 (70) | 385-405, flat | 73-81 | 12.7k-13.0k | 6 | 34 | 3,105 (30-edit run) | 3.77 GB (30-edit run) |
| D 10,000 (70) | 201-217, flat | 73-81 | 12.7k-13.0k | 6 | 34 | 743 (30-edit run) | 1.25 GB (30-edit run) |
| S/owner 10,000 (70) | 202-218, flat | 76-84 | 13.1k-13.3k | 6 | 34 | 1,095 | 1.88 GB |
| R 10,000 (70) | 477-493, flat | 81-86 | | 6 | 34 | 3,063 | 4.0 GB (28k edges, cold edge pass) |
| real krit-fir, R | 219-329, falling | 94-96 | | 6 | 34 | 1,688 | 1.9 GB |
| E 2,000 (one live session, 60 edits) | 71 | 75 | 12.6k | 6 | 34 | | |

No leak signal: heap, threads and FDs stay flat while sessions are created and disposed. Loaded classes and metaspace creep slowly and flatten (+250 classes / +6 MB over 150 edits, +3 classes per 15 edits at the tail), consistent with first-use class loading; that cannot rule out a very slow leak over days, so a daemon should watch metaspace and recycle on a budget. Side effects of dispose: an `AWT-EventQueue-0` `NullPointerException` from `BinaryFileTypeDecompilers.notifyDecompilerSetChange` is printed after each dispose (a task queued by the disposed application; harmless, filter it), and the IntelliJ platform leaves non-daemon threads, so an uncaught exception in `main` keeps the JVM alive (I hit a hung run; exit explicitly as krit-types does).

### 2.6 Correctness: diagnostics equal a fresh full check

Method: after every edit the harness compares the incremental state (cache of the last check of each file) with a fresh session over the whole tree. **Verdict** level = krit findings + rule-isolation records + "has a compiler ERROR" (what Go consumes through findings, `ruleErrors`, `errorFiles`). **Strict** level = all diagnostics (factory, severity, offset, message). A full compare of all files after every edit is only affordable on 600 synthetic and 123 real files; at 2,000 / 10,000 / 637 files I compare the checked set plus 100-400 random unchecked files after every edit and a full compare every 20-25 edits at 2,000 files.

| run | edits compared | compared against fresh full check | verdict mismatches (edits / files) | strict mismatches (edits / file-edits) | strict mismatch causes |
|---|---|---|---|---|---|
| D, lexical (owner-narrowed + inferred closure), real krit-fir seed 32 | 40 | full after every edit | 15 / 15 | 15 / 15 | (not recorded in this snapshot; every printed example was UNRESOLVED_IMPORT) |
| S/owner, synthetic 600 | 60 | full after every edit | 0 / 0 | 56 / 329 | (not recorded in this snapshot; every printed example was UNRESOLVED_IMPORT) |
| S/owner, real krit-fir seed 31 | 60 | full after every edit | 0 / 0 | 0 / 0 | none |
| S/owner, synthetic 2,000 | 100 | checked set + 250 random each edit; full every 25 | 0 / 0 | 34 / 48 | (not recorded in this snapshot; every printed example was UNRESOLVED_IMPORT) |
| S/owner, AutoMobile 637 | 30 | checked set + 120 random each edit | 0 / 0 | 0 / 0 | none |
| S/owner, synthetic 10,000 | 30 | checked set + 400 random each edit | 0 / 0 | 6 / 6 | (not recorded in this snapshot; every printed example was UNRESOLVED_IMPORT) |
| B, synthetic 2,000 | 50 | checked set + 250 random each edit; full every 25 | 0 / 0 | 0 / 0 | none |
| S/name, synthetic 600 | 40 | full after every edit | 0 / 0 | 0 / 0 | {} |
| S/name, real krit-fir seed 31 | 60 | full after every edit | 0 / 0 | 0 / 0 | {} |
| S/name, real krit-fir seed 32 | 60 | full after every edit | 0 / 0 | 0 / 0 | {} |
| S/owner, real krit-fir seed 32 | 60 | full after every edit | 11 / 11 | 11 / 11 | {UNRESOLVED_REFERENCE=11} |
| R, synthetic 600 | 60 | full after every edit | 0 / 0 | 56 / 327 | {UNRESOLVED_IMPORT=654} |
| R, real krit-fir seed 31 | 60 | full after every edit | 0 / 0 | 0 / 0 | {} |
| R, real krit-fir seed 32 | 60 | full after every edit | 0 / 0 | 0 / 0 | {} |
| R, synthetic 2,000 | 60 | checked set + 250 random each edit; full every 20 | 0 / 0 | 20 / 32 | {UNRESOLVED_IMPORT=64} |
| R, AutoMobile 637 | 25 | checked set + 100 random each edit | 0 / 0 | 5 / 5 | {CANNOT_INFER_PARAMETER_TYPE=5, UNRESOLVED_REFERENCE=10} |
| R, synthetic 10,000 | 30 | checked set + 400 random each edit | 0 / 0 | 7 / 7 | {UNRESOLVED_IMPORT=14} |

Strict mismatches in slice sessions (S, R, D) on the synthetic projects are `UNRESOLVED_IMPORT` text/offset differences after deleting an imported file: in a slice, the package the import named no longer exists ("Unresolved reference 'p10'"), in the full tree only the class is missing ("'App1150'"). Both are compiler ERRORs, so the gating verdict is identical. B (full-module session) has zero strict mismatches. On AutoMobile, R's strict differences are 3 extra `CANNOT_INFER_PARAMETER_TYPE` / `UNRESOLVED_REFERENCE` errors in a file (`CtrlProxy.kt`) that already has 1,291 errors in the full tree (the slice lacks some declarations), so the file is gated either way. The rows for S/owner seed 32 are different in kind: their strict and verdict mismatches are the **stale verdicts left by the two missed dependents**, not slice artifacts. Fix for the artifacts if wanted: add one representative file per imported package to the slice.

Parity of the AA runner with the shipped path (measured): on 300 synthetic files, base state + 3 random edits, krit findings per (file, rule) are **equal** in all 4 rounds and the set of compiler-ERROR files is **equal** in all 4 rounds (`parity` mode against `AnalysisSession.check`). Per-request context reaches checkers: setting `FirRuleContext` (test-file classification) on the calling thread at **check time** changes findings (checkers run inline on the calling thread). Rule selection (`enabledRuleIds`) is **bound when the FIR session is built** (`KritFirCheckers`' constructor): changing the enabled set needs a new session. `ONLY_COMMON_CHECKERS` gives the same krit findings as `EXTENDED_AND_COMMON_CHECKERS` and matches the shipped compile, which does not report extended compiler warnings.

## 3. Goal 2: strategy C on the current Kotlin version

API present in 2.4.20 (checked with `javap`): `KaSourceModificationService` (`detectLocality`, `handleInvalidation`, `handleElementModification`), `KotlinModificationEvent` family (module state and out-of-block events, global variants, `publishModuleOutOfBlockModificationEvent` etc.), `KotlinModificationTrackerFactory` with an event-driven standalone implementation, and dangling-file support (`KaDanglingFileModule`, `KtFile.contextModule`, `copyOrigin`, `KaDanglingFileResolutionMode {PREFER_SELF, IGNORE_SELF}`). **Absent**: any way to hand a built standalone session new content or a new file set. `KotlinStaticProjectStructureProvider` only exposes `allModules` / `allSourceFiles`; `KtSourceModuleBuilder` takes files at build time (`addSourceRoots`, `addSourceVirtualFile(s)`); the standalone declaration index is filled in its constructor and has no public mutator.

Probes (200-file project, identical on-disk edit, result compared with a fresh session; measured, `stratc` mode):

| probe | what it does | result |
|---|---|---|
| P1 | publish module out-of-block + module state UPDATE + global source OOB events | diagnostics unchanged (stale); PSI keeps the old text |
| P2 | re-read PSI through `PsiManager.findFile(virtualFile)` | same PSI instance, old text |
| P3 | raw AST replacement (`rawRemoveAllChildren`/`rawAddChildren`) + `handleElementModification(Unknown)` + OOB event | `IncorrectOperationException: Cannot modify a read-only file` (`CoreLocalVirtualFile.isWritable()` is false) |
| P3w | same after swapping `AbstractFileViewProvider.myVirtualFile` for a writable delegating file (reflection) | passes the writable check, then `Somebody has requested stubbed spine during PSI operations` (standalone PSI is stub-backed), later `IndexOutOfBounds` in `SpineRef` |
| P5 | **supported**: build a session whose source module contains a `LightVirtualFile` with the unsaved text | correct (matches a fresh session) for a body edit and a member addition |
| E | **supported**: keep the session, analyze `KtPsiFactory.createFile(...)` with `originalFile = session PSI` | fast; correct on synthetic, not on real code, see below |

Verdicts:
- **Strategy C proper: still infeasible on 2.4.20.** The events are built for an IDE that owns PSI mutation; the standalone platform has no PSI-change pipe and the workaround needs private-API reflection that will not survive minor versions. Not recommended; nothing more to benchmark. Note the API churn: `StandaloneAnalysisAPISessionBuilder.registerCompilerPluginServices` is `@Deprecated(HIDDEN, "Obsolete Standalone API")` in 2.4.20; the runner registers `KotlinCompilerPluginsProvider` directly with `KotlinStandaloneFirCompilerPluginsProvider(CompilerConfiguration)` after setting `CompilerConfiguration.extensionsStorage`. Several entry points need `@KaPlatformInterface` / `CompilerConfiguration.Internals` / `@KaImplementationDetail` opt-ins.
- **E, dangling copy in a long-lived session: the supported substitute for C, not yet correct on real code.** Cost = create a file copy and analyze it; no rebuild; library and FIR state stay warm: p50 49 ms (idle, 2,000 files, 35 of 35 edits identical to a fresh check), 88 ms loaded, 200 ms on AutoMobile and 347 ms on krit-fir (both timing only). Valid only while the edit does not change the file's observable surface and every earlier edit since the session was built was surface-preserving (dependents resolve against the original file).
  - Fidelity probe (`fidelity` mode): the **unchanged** text of 60 krit-fir files analyzed as dangling copies with the default `PREFER_SELF` gave diagnostics identical to the original in 60/60; `IGNORE_SELF` differed in 4/60 (expected, it ignores the file's own declarations).
  - Failure on edited text (`dangling` mode, edits concentrated on a 6-file pool, 24 edits, each verified against a fresh session): 5 mismatches, all false compiler diagnostics: `FirRuleIsolation.kt` got 6 `NOTHING_TO_OVERRIDE` errors on 4 of 4 edits; `JdbcPreparedStatementNotClosed.kt` a `NO_TAIL_CALLS_FOUND` warning on its first copy only. A separate verified run (30 edits spread over all files) had 1 mismatch (`MUST_BE_INITIALIZED_OR_BE_ABSTRACT` errors on `AnalysisSession.kt`). `FirRuleIsolation.kt` uses context parameters (`-Xcontext-parameters` in krit-fir's own build); my guess, untested, is that dangling-file sessions do not inherit the module's language settings or resolve member scopes against stale origin state. Any compiler ERROR gates the file's krit verdict, so this would be a visible regression; root-cause before use.
- The prior spike's result (rebuild source module sessions) remains the only fully general approach; D/S/R are its cheaper, slice-based form.

## 4. Goal 3: dependents

### 4.1 What the spike implemented

- **Lexical index** (`Deps.kt`, a stand-in for what the Go scanner already has): per file, identifier tokens, package, imports, and declarations with an "observable header" (explicit-type declarations: the signature; inferred-type declarations: signature + initializer, with string/number literal contents normalized away). A changed item `(pkg, owner, name)` is visible to a file that mentions `name` and is in the declaring package, imports the name (explicit or wildcard) or sees the package through a Kotlin default import. Supertypes of a changed class header become items (exhaustive `when`, abstract members).
  - **owner-narrowed** (S/owner): a member item also requires the owner's name in the dependent. Precise, but unsafe: a member can be reached through an inferred receiver type (`collector.depTracker.recordPerFileDep(...)` never mentions `DepTracker`).
  - **name-only** (S/name): members match by name alone. Safe, but with repeated member names (`use`, `body`, `derived`, `name`, `build`) it matches nearly everything.
- **Resolved-surface gate** (S, R): `Runner.surface` renders every non-private declaration of the file, members included, with resolved/inferred types (e.g. `fun derived(): kotlin.Int`) and hashes it. Dependents are looked up only if the hash differs from the one recorded at the file's last check; each checked dependent's own surface is compared too, so the old "declares an inferred type so assume it leaks" closure becomes a fixpoint on real facts (bounded at 16 waves, then fall back). Where the previous surface is unknown the file is treated as changed (edited files) or by the inferred-declaration fallback (dependents). The spike primes edited files' previous surface by resolving their pre-edit text, untimed, to emulate a daemon that stored it.
- **Resolved reverse edges** (R): krit-types' `DepEdges.recordResolvedDependencyEdges` (copied into the spike unchanged except for the tracker type) walks a file inside the session and records the source files its resolved references reach, classified signature / propagating-body / local. R keeps `file -> targets` and the reverse map, refreshed for every checked file (cost included in the timings above), seeded by one untimed pass over the module (21 s for 2,000 files, 47-66 s for 10,000 files, 33 s for 123 real files, 63 s for AutoMobile, under load). Dependents of a surface-changed file = its reverse edges; new top-level names (new overloads, shadows, subtypes) additionally use the scope-aware lexical lookup; new member names additionally match files whose last verdict held `Unresolved reference 'name'`.

### 4.2 Measured quality

Ground truth = files (other than the edited ones) whose diagnostics differ between the fresh full check before and after the edit; only runs that do a full compare after every edit are used. "flagged" = dependents the strategy chose to re-check.

| run | kind (n) | flagged dependents avg | truly affected avg | missed (total) |
|---|---|---|---|---|
| D, lexical (owner-narrowed + inferred closure), real krit-fir seed 32 | add (6) | 0.0 | 0.0 | 0 |
| D, lexical (owner-narrowed + inferred closure), real krit-fir seed 32 | comment (3) | 0.0 | 0.0 | 0 |
| D, lexical (owner-narrowed + inferred closure), real krit-fir seed 32 | delete (4) | 0.0 | 0.0 | 0 |
| D, lexical (owner-narrowed + inferred closure), real krit-fir seed 32 | literal (17) | 0.0 | 0.0 | 0 |
| D, lexical (owner-narrowed + inferred closure), real krit-fir seed 32 | rename (10) | 11.6 | 1.1 | 2 |
| S/owner, synthetic 600 | add (1) | 0.0 | 0.0 | 0 |
| S/owner, synthetic 600 | body (18) | 0.0 | 0.0 | 0 |
| S/owner, synthetic 600 | comment (1) | 0.0 | 0.0 | 0 |
| S/owner, synthetic 600 | delete (12) | 1.3 | 0.9 | 0 |
| S/owner, synthetic 600 | hot (1) | 441.0 | 441.0 | 0 |
| S/owner, synthetic 600 | overload (5) | 7.2 | 5.8 | 0 |
| S/owner, synthetic 600 | sealed (4) | 1.0 | 1.0 | 0 |
| S/owner, synthetic 600 | shadow (3) | 0.0 | 0.0 | 0 |
| S/owner, synthetic 600 | sig (15) | 3.7 | 3.3 | 0 |
| S/owner, real krit-fir seed 31 | add (7) | 0.0 | 0.0 | 0 |
| S/owner, real krit-fir seed 31 | comment (6) | 0.0 | 0.0 | 0 |
| S/owner, real krit-fir seed 31 | delete (5) | 0.0 | 0.0 | 0 |
| S/owner, real krit-fir seed 31 | literal (29) | 0.0 | 0.0 | 0 |
| S/owner, real krit-fir seed 31 | rename (13) | 19.5 | 1.4 | 0 |
| S/name, synthetic 600 | body (13) | 2.8 | 0.0 | 0 |
| S/name, synthetic 600 | comment (1) | 0.0 | 0.0 | 0 |
| S/name, synthetic 600 | delete (8) | 445.5 | 1.0 | 0 |
| S/name, synthetic 600 | overload (3) | 442.3 | 6.7 | 0 |
| S/name, synthetic 600 | sealed (3) | 1.0 | 1.0 | 0 |
| S/name, synthetic 600 | shadow (2) | 0.0 | 0.0 | 0 |
| S/name, synthetic 600 | sig (10) | 444.2 | 3.2 | 0 |
| S/name, real krit-fir seed 31 | add (7) | 0.0 | 0.0 | 0 |
| S/name, real krit-fir seed 31 | comment (6) | 0.0 | 0.0 | 0 |
| S/name, real krit-fir seed 31 | delete (5) | 0.0 | 0.0 | 0 |
| S/name, real krit-fir seed 31 | literal (29) | 0.0 | 0.0 | 0 |
| S/name, real krit-fir seed 31 | rename (13) | 11.2 | 1.4 | 0 |
| S/name, real krit-fir seed 32 | add (8) | 0.0 | 0.0 | 0 |
| S/name, real krit-fir seed 32 | comment (7) | 0.0 | 0.0 | 0 |
| S/name, real krit-fir seed 32 | delete (5) | 0.0 | 0.0 | 0 |
| S/name, real krit-fir seed 32 | literal (26) | 0.0 | 0.0 | 0 |
| S/name, real krit-fir seed 32 | rename (14) | 10.6 | 1.5 | 0 |
| S/owner, real krit-fir seed 32 | add (8) | 0.0 | 0.0 | 0 |
| S/owner, real krit-fir seed 32 | comment (7) | 0.0 | 0.0 | 0 |
| S/owner, real krit-fir seed 32 | delete (5) | 0.0 | 0.0 | 0 |
| S/owner, real krit-fir seed 32 | literal (26) | 0.0 | 0.0 | 0 |
| S/owner, real krit-fir seed 32 | rename (14) | 9.9 | 1.5 | 2 |
| R, synthetic 600 | add (1) | 0.0 | 0.0 | 0 |
| R, synthetic 600 | body (18) | 0.2 | 0.0 | 0 |
| R, synthetic 600 | comment (1) | 0.0 | 0.0 | 0 |
| R, synthetic 600 | delete (12) | 1.2 | 0.9 | 0 |
| R, synthetic 600 | hot (1) | 441.0 | 441.0 | 0 |
| R, synthetic 600 | overload (5) | 17.4 | 5.8 | 0 |
| R, synthetic 600 | sealed (4) | 1.0 | 1.0 | 0 |
| R, synthetic 600 | shadow (3) | 0.0 | 0.0 | 0 |
| R, synthetic 600 | sig (15) | 12.9 | 3.3 | 0 |
| R, real krit-fir seed 31 | add (7) | 0.0 | 0.0 | 0 |
| R, real krit-fir seed 31 | comment (6) | 0.0 | 0.0 | 0 |
| R, real krit-fir seed 31 | delete (5) | 0.0 | 0.0 | 0 |
| R, real krit-fir seed 31 | literal (29) | 0.0 | 0.0 | 0 |
| R, real krit-fir seed 31 | rename (13) | 4.9 | 1.4 | 0 |
| R, real krit-fir seed 32 | add (8) | 0.0 | 0.0 | 0 |
| R, real krit-fir seed 32 | comment (7) | 0.0 | 0.0 | 0 |
| R, real krit-fir seed 32 | delete (5) | 0.0 | 0.0 | 0 |
| R, real krit-fir seed 32 | literal (26) | 0.0 | 0.0 | 0 |
| R, real krit-fir seed 32 | rename (14) | 6.6 | 1.5 | 0 |

Note on the rows: the S/owner rows other than "seed 32" (and the D row) ran an earlier build of S in which the lexical lookup also applied the inferred-type closure; the S/owner seed 32, S/name and R rows ran the final build (closure off, surface fixpoint instead). Timings in section 2 for S/owner are from the earlier build.

What the runs show:
- Without package/import scoping the `println` shadow edit flagged 132-664 files; with scoping 0-5. Without literal normalization, a string-literal edit on an expression-bodied property flagged 433 files on AutoMobile; the surface gate removes that class (the type is unchanged).
- **S/name flags 440+ of 600 files for signature, overload and delete edits on the synthetic project** (truth 1-7), and 7-8x the affected files on real renames (10.6-11.2 vs 1.4-1.5): name-only member matching is not usable.
- **S/owner is precise but unsafe**: the D run on real krit-fir seed 32 missed 2 real dependents (`OracleClassChecker.kt` still called the renamed `DepTracker.recordPerFileDep` without mentioning `DepTracker`), leaving 15 later edits with a stale verdict.
- **R missed nothing in any run** (3 full-oracle runs, 3 sampled runs) and flags 1-4.4x the affected files (real renames: 4.9 and 6.6 vs 1.4 and 1.5; synthetic signature edits 12.9 vs 3.3) (file-granular edges: a file that references any declaration of the changed file is flagged, not only the changed declaration). Declaration-granular edges (record the target symbol, not just its file) would close most of that gap; not measured.

### 4.3 What already exists in Go (read, not run)

- `internal/scanner/name_facts.go` `TopLevelDeclaredNames`: tree-sitter top-level non-private declarations (functions incl. extensions, properties, classes, objects, typealiases, `fun interface`) with an operator flag.
- `internal/oracle/name_collision.go` `demoteNameCollidingHits` (#846, fixes #826): when a changed file declares a name, cached facts of files that mention it are demoted if they are in the declaring package, import the name (explicit, wildcard, default imports) or spell it package-qualified; includes `operatorGates` (plus/minus/get/set/invoke/getValue/setValue/provideDelegate/compareTo/contains/iterator/next/hasNext/rangeTo/componentN) and fails closed on unreadable files. Stated limits: Java sources and typealiases are not scanned, the check is run-local, deleted files are left to closure verification (#847).
- `internal/oracle/cache.go`: per-file closure of transitive source dependencies verified by hashing the current contents of the closure; krit-fir's `DepTracker.depPathsByFile` feeds it, and **krit-types already has the propagating-edge recorder the R runner reuses** (`tools/krit-types/.../DepEdges.kt`: signature edges, inferred bodies, `const`, `inline`, contracts propagate; ordinary bodies do not). The FIR checker cache entry has a `ClosureFingerprint` field, but `CheckCacheFingerprint` keys the whole compilation.
- `internal/arch/abihash.go`, `internal/usedsymbols/`: per-module ABI hash and used symbols (module-level invalidation).

### 4.4 Recommended dependents approach

1. **Reverse resolved edges are the primary dependents source** (R): they are exact for references that exist, cover inferred receiver types, inherited members, operator conventions, delegates and destructuring (the recorder resolves them) and need no name matching. Move `DepEdges` into a shared module used by krit-types and krit-fir; make the incremental runner record edges for every file it checks and return them (with the surface hash) to Go, which persists them in the per-file cache entry. Cold start: edges come from the first full compile (krit-fir's `analyzeWithDeps` already returns `depPathsByFile`).
2. **Lexical lookup for names that edges cannot know about**, in Go, reusing `TopLevelDeclaredNames` and the #846 rules extracted to a shared package: new top-level names (overload in another file, shadowing, new subtype of a sealed type), plus the unresolved-name set for new members. This is exactly the #826 class, and the shadow edits in these runs (the #826 pattern) never produced a miss.
3. **Surface gate** from the session as above, so body-only edits cost one file and a signature edit only pulls what it affects; store the surface hash per file in the cache entry so the previous surface is known across processes.
4. Holes to close before shipping (inferred, not run): Java sources in modules (not in the name facts; the slice must include `.java` roots), typealiases, `expect/actual` and HMPP fragments, KSP/kapt generated sources, `internal` visibility across modules, `@DslMarker` scopes and implicit receivers (edges handle them, lexical does not), `import ... as` aliases.

## 5. Goal 4: integration design

### 5.1 Where it plugs in

A new runner **inside the krit-fir daemon**, chosen per request; `compileModule` (the single execution seam of `AnalysisSession`, used by `checkCompilation`, `analyzeCompilation` and `ModuleRunner`) stays as the full-compile tier and the fallback. The runner implements the existing request/response shapes (`CheckRequest` -> `BatchResult`, plus the oracle payload for `analyzeWithCheck`), so Go keeps one protocol; the visible changes are optional fields: `changed` (files the planner treats as edited), `runner` (`aa` / `cli`) and per-file `surface` / `deps` in the response.

```
Go planner                              krit-fir daemon
-----------                             ----------------
files + content hashes  -----------> check(files, changed?, rules, configs, facts)
per-file cache lookup                      |
  hit  -> done                             v
  miss -> M                           IncrementalRunner.plan(M)
                                           |  eligible? rules tier OK, warm edge state, |affected| <= threshold,
                                           |  no fragments/scripts/generated roots
                                           v
                                      AA slice session over M + dependents + forward closure
                                           |  check M, render surfaces, record edges
                                           |  surface changed? -> next wave over reverse edges + name lookup
                                           |  error / threshold / unsupported -> FULL COMPILE tier (compileModule)
                                           v
                                      BatchResult(+surface, +deps, +runner)
```

### 5.2 Session lifetime and memory budget

- Default tier: **per-request slice sessions** (D/S/R): created, used, disposed within a request. Measured flat memory over 150 edits; peak RSS 1.25-2.0 GB at 10,000 files with a 6 GB heap limit (peak heap 0.7-1.1 GB; R adds ~1.1 GB for the edge maps and the cold edge pass at 10,000 files), 1.4-1.8 GB at 2,000 files. Nothing outlives a request, so there is no stale-state class of bug and no invalidation logic.
- Optional second tier for large modules and the editor: one **long-lived session per module + dangling overlay (E)**, rebuilt when a surface-changing edit lands, when the rule set or any fingerprint input changes, after N overlays (suggest 200) or above a per-daemon RSS budget (suggest 1.5 GB). Gated on root-causing the false diagnostics.
- A whole-module session (B) is only acceptable as a fallback: 1.7 GB peak RSS at 2,000 files, 3.8 GB at 10,000 files. Cap sessions per daemon (not per module): an 8-module Android build would otherwise multiply it.

### 5.3 Cache key and fingerprint (replaces the whole-compilation key on this path)

Today `CheckCacheFingerprint = H(FirInvocationFingerprint, every .kt under sourceDirs by path + content)`, so any edit invalidates every entry (the 20.5 s / 0 hits row in the issue). Proposed per-file key:

`key(F) = H(path(F), contentHash(F), FirInvocationFingerprint, closureHash(F))`, validated at lookup by the name-collision demotion pass

- `FirInvocationFingerprint` unchanged (classpath, jar identity, rules, options, test-file classification, scan paths, SDK levels, JVM target).
- `closureHash(F)` = H over `(dep path, surfaceHash(dep))` for the resolved forward edges recorded at F's last check. First version: use the deps' **content** hash like `internal/oracle/cache.go` does (correct, over-invalidates: a body edit of a dependency invalidates its dependents); move to surface hashes once the runner returns them (a body edit then invalidates only the file itself).
- New or changed declared names are not hashed; the #846 demotion pass validates them at lookup (a changed or new file that declares a name F can see demotes F). This is what edges cannot express.
- Rules that need the whole compilation (section 6) keep the old whole-compilation key.
- `FirCacheVersion` 8: entries gain `surfaceHash`, `deps`, `runner`.

### 5.4 Error-gating semantics

The contract (`verdict.go`, `CheckResponse`): a requested file's verdict is authoritative unless it has a compiler ERROR, was not compiled, or a location-less global ERROR exists; `ruleErrors` mark (file, rule) pairs where Go keeps its own finding. The AA runner reproduces the per-file part (measured: error-file sets equal to the CLI in 4 of 4 parity rounds, 0 verdict mismatches in every correctness run above). Design points:
- There is no location-less "global" error in AA: session build failures, bad classpath entries or JDK problems must surface as "runner unavailable" and fall back to the full compile, never as per-file verdicts.
- Slice sessions do not see errors in files outside the slice. For the contract that is fine (other files' errors never gated a file), but conflicts that appear in two files (redeclaration, conflicting overloads, duplicate class names) need both files in the slice: add files that declare the same top-level name to the forward closure.
- Arm `FirRuleErrorRecorder` before the first `analyze()` of every session (it is captured when the checker extension is built). Without it one throwing rule aborts the whole file's diagnostics, which is exactly how the first AutoMobile run died. With it the per-rule/per-file records match the CLI semantics.
- Use `ONLY_COMMON_CHECKERS`.

### 5.5 Multi-module projects (inferred; the spike used two synthetic modules and single-module real trees)

One source `KaModule` per Gradle module with `addRegularDependency` edges from the existing `ModuleSpec.dependsOn` / `friends`; classpath libraries as library modules. **Upstream modules can be source modules in the slice, so the AA path does not need to compile upstream modules to class output** (the CLI module driver does, because downstream modules consume binaries): generated roots (`generatedSourceRoots`) and friend visibility must be handled, fragments (HMPP) stay on the CLI. Cross-module dependents: module graph + `arch.AbiHash` / `usedsymbols` to decide whether a downstream module is affected, then the same edge/name lookup inside it.

### 5.6 Fallback to a full compile

Triggers (any): estimated `|affected| > threshold`; wave count exceeded; session build/check exception other than isolated rule errors; any fingerprint input changed (classpath, jar, rules, configs, facts, JVM target); module with fragments, scripts, uncovered Java edits or generated roots; a `requiresWholeCompilation` rule enabled and its inputs changed; cold daemon (no edge/surface state yet); and a sampled **self-check** (compare AA against a full compile for a random sample and every N-th request; mismatch disables the runner for the module and logs). The existing path is untouched, so fallback is a branch.

Threshold model (derived from the measurements, AA cost ~ first-file cost + c x |set|; c = 15-27 ms/file synthetic, ~125 ms/file AutoMobile; CLI ~ 2.4 s at 2,000 files, 13.4 s at 10,000, 4.4 s on the 123-file krit-fir sources, 18.7 s on AutoMobile): ~90 files at 2,000 files, ~500 at 10,000, ~40 on krit-fir, ~135 on AutoMobile. Make it adaptive: record measured ms/file of both runners per module and compare predictions.

### 5.7 Interaction with #810

#810 makes one compile feed both the oracle (`analyzeWithDeps`) and the checkers. The AA runner **does not conflict and can subsume it incrementally**:
- The oracle facts are produced by the same FIR extension through the thread-local `OracleCollectorRegistry`. I activated a collector around AA checks and compared with `AnalysisSession.analyze` (declarations modulo order, expression types, lambda-suspend flags): 12/12 sampled files identical on the synthetic project and 12/12 on krit-fir's sources (measured). The diagnostics payload (it comes from the compiler message collector) was not compared; it would be mapped from `collectDiagnostics`. So one AA pass can return checker findings and oracle facts for the same file set.
- Plan: land #810's protocol (combined request, the broker deciding who sends it, `Prefetch` reuse) as designed, and implement the AA runner as an alternative executor behind `analyzeWithCheck`, selected by the planner: cold and large-set requests keep using the shared full compile, small-set requests use the AA pass with both outputs. #810's "oracle daemon serves at most 8 misses" limit is the same idea as the threshold above.
- Both cache layers (oracle entries, FIR entries) stay separate and both receive fresh entries for the AA-checked set.

## 6. Semantic gaps found while running real checkers under the Analysis API

1. `SerialVersionUIDInSerializableClass.computeSerializableSourceClassNames` calls `session.firProvider.getFirFilesByPackage`, which throws `IllegalStateException: Should not be called in FIR IDE` in `LLFirProvider`. Even with a replacement it is a whole-compilation rule: the supertype names of any source class decide a class's verdict ("Go resolves a simple name to a source class anywhere in the project"). A grep of the other checkers finds no other `firProvider` / file iteration; the `symbolProvider` lookups (WorldReadableFiles, WorldWriteableFiles, UseAnyOrNoneInsteadOfFind) worked under AA in every run. Recommendation: `FirRule.requiresWholeCompilation` (with a lint gate in the style of `make lint-rules`); such rules stay on the full-compile tier with the whole-compilation key, or get their name set from Go as an input.
2. Rule selection is bound when the session is built; `FirRuleContext` per-file data is read at check time (works in AA).
3. Dangling-copy analysis can yield false compiler errors (section 3).
4. Packaging: AA needs the unshrunk IntelliJ core jar *ahead of* `kotlin-compiler` on the classpath (krit-types does this with `standaloneIntellijCore`; without it `NoSuchMethodError: FileType.getDefaultExtension()`), plus caffeine and the `-for-ide` libraries: ~26 MB uncompressed; krit-types.jar is 83 MB vs krit-fir.jar 70.8 MB. krit-fir's shadow `minimize` needs the same excludes as krit-types. `settings.gradle.kts` needs the `intellij-repository` entry for `com.jetbrains.intellij.platform:core` (this branch adds it).

## 7. Recommended implementation plan

PR-sized steps, each shippable behind `KRIT_FIR_INCREMENTAL` (off by default) until step 6.

1. **krit-fir: AA runner skeleton, no behaviour change.** Add the AA dependencies and the `standaloneIntellijCore` ordering to the krit-fir shadow jar; `runner/incremental/` with the session builder (supported registration, `ONLY_COMMON_CHECKERS`, recorder armed), a slice builder from a caller-supplied file list, `check(files)`. Tests: run the existing `AnalysisSession*Test` fixtures through both runners and assert equal findings / errorFiles / ruleErrors. Risk: jar size and class-loading conflicts; this PR proves them.
2. **Rule tiering.** `FirRule.requiresWholeCompilation`, mark `SerialVersionUIDInSerializableClass`, lint gate, protocol field telling Go which rules the incremental runner did not evaluate; decide Go-fed input vs full-compile tier for that rule.
3. **Shared edge recorder.** Move `DepEdges.kt` and the `DepTracker` edge classes into a module shared by krit-types and krit-fir; krit-fir's runner records edges + resolved-surface hash for every file it checks.
4. **Daemon protocol (coordinate with #810).** `check` gains optional `changed`; the response gains `runner`, per-file `surface` and `deps`. Unknown fields are already ignored by the Go client. `analyzeWithCheck` keeps its signature; the planner picks the executor.
5. **Go: per-file cache key v2 and dependents planner.** `FirCacheEntry` v8 (`surfaceHash`, `deps`); key = path + content + invocation fingerprint + closure hash; keep the whole-compilation key for tiered rules and for runs without the runner. Extract `TopLevelDeclaredNames` and the #846 name-collision rules from `internal/oracle` into a shared package; planner = changed files + reverse edges + name/scope lookup + wave expansion on `surfaceHash` change; threshold, fallback, `-perf` counters.
6. **Self-check and enablement.** Sampled AA-vs-full comparison (`KRIT_FIR_INCREMENTAL_VERIFY`), disable-on-mismatch, CI job running the parity and correctness harness on playground plus a real corpus, then default on for the daemon path.
7. **Editor/LSP path.** Unsaved-buffer overlay (`LightVirtualFile`, probe P5) for FIR checks from `krit-lsp` (today the LSP uses the oracle but runs no FIR checkers; the closest existing entry, `analyzeFile` for plugin rules, compiles the whole module per request and already accepts unsaved text in `request.source`).
8. **Optional: long-lived session + dangling overlay (E)** for large modules after the false-diagnostic issue is root-caused; memory budget and rebuild policy as in 5.2.
9. **Multi-module.** Module sessions from `ModuleSpec`, cross-module dependents through the ABI hash / used symbols, drop upstream class-output compilation on the AA path, keep fragments and Android-variant edge cases on the CLI.

### Risks

| risk | likelihood / impact | mitigation |
|---|---|---|
| Entry points used are `@Deprecated(HIDDEN)` or opt-in and churn between Kotlin minors (already: `registerCompilerPluginServices` hidden in 2.4.20) | high / medium | isolate in one file, compile against the pinned version, a CI job running the parity tests on every Kotlin bump |
| Whole-compilation or provider-scanning rules silently wrong | medium / high | tiering flag + lint gate + recorder; self-check sampling |
| Dependents miss an edge (Java, typealiases, generated sources, unresolved-to-resolved changes, member added to an existing class) -> stale cached verdict | medium / high | resolved edges + #846 name rules + unresolved-name lookup; surface gate conservative where history is unknown; sampled self-check; fail closed on unreadable files |
| AA per-file cost makes large affected sets slower than the CLI | certain / medium | adaptive threshold and fallback; measured crossovers above |
| Memory: R edge maps and whole-module sessions at 10,000 files peak 3.8-4.0 GB RSS | medium / medium | slice sessions by default, per-daemon cap, store edges in the cache entry rather than in memory |
| False compiler errors from dangling analysis gate files | observed in 6 of 54 verified edits / medium | keep E opt-in until root-caused |
| Packaging: unshrunk-core ordering, +~12 MB, shadow `minimize` interactions | medium / low | mirror krit-types' build; size check in CI |
| Global state: `KotlinCoreApplicationEnvironment` shared by AA and the K2 CLI path in one JVM; `ModuleRunner` notes the environment is global | medium / medium | both ran in one JVM in the `parity` and `oracle` modes without interference; keep one request at a time as today; add an interleaving test |
| Noisy stderr (AWT NPE after dispose), non-daemon threads | certain / low | uncaught-exception filter, explicit exit |
| Generated/KSP sources and HMPP fragments unsupported in slices | medium / medium | planner routes those modules to the CLI |

## 8. Measured vs inferred

Measured: sections 2.2-2.6 (all latency, memory and correctness numbers), 3 (probes, fidelity), 4.2 (dependents quality), the AA-vs-CLI parity runs, oracle-facts parity (5.7), the `SerialVersionUIDInSerializableClass` failure and rule-isolation behaviour (6.1), jar sizes, the rule-context and checker-filter probes.

Inferred or not run: multi-module behaviour with more than two modules and with Gradle-sourced classpaths (5.5); Java sources, HMPP and KSP outputs; thresholds for projects other than the four measured; parallel checking across threads (process CPU exceeded wall by ~2.3x on the 500-file edits, so there is parallelism in the session already, but I did not try driving checks from several threads); declaration-granular edges (4.2); the Go-side planning cost (the Go code was read, not run); diagnostics payload parity for the oracle (only declarations / expressions / lambda flags were compared); anything about Kotlin versions other than 2.4.20 (CI's 2.4.10 was not run).

## 9. Reproducing

```
cd tools/krit-fir
./gradlew --no-daemon spikeClasspath          # compiles the spike source set; writes build/spike-classpath.txt and build/spike-env.txt
java -Xmx3g -Djava.awt.headless=true $(cat build/spike-env.txt) -cp "$(cat build/spike-classpath.txt)" spike.BenchKt <mode> [--opt=value ...]
```

Modes: `inc` (`--strategy=A|B|D|S|R --files=N --edits=N --warmup=N --oracle=0|1 --oracleSample=N --oracleFullEvery=N --coldCheck=true|false --real=dir1,dir2 --cp=file-with-classpath --excludeRules=A,B --scopeAware=false --ownerAware=true`), `cli` (shipped K2JVMCompiler baseline), `parity` (AA vs CLI findings/error files), `oracle` (oracle-facts parity), `dangling` / `fidelity` (strategy E), `stratc` (strategy C probes), `ctx` (rule context binding), `filters` (checker filter), `surf`, `depdebug`. Wrap timing runs in `flock /tmp/claude-0/krit-bench.lock`.

Source map: `Gen.kt` synthetic project and edits, `Proj.kt` real-project copy and edits, `Deps.kt` lexical dependents index, `Edges.kt` resolved-edge recorder (copy of krit-types' `DepEdges.kt`), `Runner.kt` session builder, checker driver, surface rendering, `Bench.kt` harness and reporting, `Cli.kt` baseline and parity, `Dangling.kt` strategy E, `StrategyC.kt` strategy C probes.
