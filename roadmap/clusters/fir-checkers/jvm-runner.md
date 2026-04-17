# B.1 — krit-fir JVM runner

**Cluster:** [fir-checkers](README.md) · **Status:** planned · **Track:** B · **Severity:** n/a (tool mode)

## Catches

The single biggest unlock for FIR-aware rules: a JVM subprocess
krit can spawn, daemonize, and stream files to, that loads the K2
frontend, runs a set of `FirAdditionalCheckersExtension` checkers
over each file, and emits findings back as JSON. Structurally it
is a sibling of `tools/krit-types/`, not a replacement — types and
findings are distinct payloads and may need different per-file
budgets, but they share the daemon lifecycle, the Analysis API
session setup, and the classpath handling.

## Shape

```
tools/krit-fir/                              ← new Gradle module
  build.gradle.kts                           ← compileOnly kotlin-compiler-embeddable
  src/main/kotlin/dev/krit/fir/
    Main.kt                                  ← entry, stdio protocol, daemon mode
    runner/
      AnalysisSession.kt                     ← buildStandaloneAnalysisAPISession wiring
      CheckerRegistrar.kt                    ← FirExtensionRegistrar with our checkers
      FindingCollector.kt                    ← DiagnosticReporter → krit Finding JSON
    rules/                                    ← checker classes live here (see checker-api.md)
```

The runner is a single fat JAR launched as a subprocess from
`internal/firchecks/`. It speaks the same stdio JSON protocol as
`krit-types` — one request object per line, one response object
per line — so the Go side can reuse the same framing, buffering,
and readiness handshake code with minor adjustments.

### Protocol (request)

```json
{
  "id": 42,
  "command": "check",
  "files": [
    {"path": "app/src/main/kotlin/com/acme/Foo.kt", "contentHash": "sha256..."}
  ],
  "sourceDirs": ["app/src/main/kotlin"],
  "classpath": ["/.../kotlin-stdlib.jar", "/.../retrofit.jar"],
  "rules": ["ComposeRememberWithoutKey", "FlowCollectInOnCreate"]
}
```

### Protocol (response)

```json
{
  "id": 42,
  "succeeded": 1,
  "skipped": 0,
  "findings": [
    {
      "path": "app/src/main/kotlin/com/acme/Foo.kt",
      "line": 39,
      "col": 12,
      "rule": "FlowCollectInOnCreate",
      "severity": "warning",
      "message": "Flow.collect in onCreate without lifecycleScope",
      "confidence": 0.95
    }
  ],
  "crashed": {}
}
```

The `crashed` map reuses the poison-marker discipline from the
oracle cache — a file that crashes the checker is recorded with a
short error string so downstream invocations can mark its cache
entry as poison and skip it.

## How checkers are invoked

Two options, both viable; this cluster commits to **Option A** for
Track B to minimize divergence from Metro:

### Option A (commit) — Embedded kotlinc via `FirExtensionRegistrar`

Run the full kotlinc frontend in-process against the supplied
source set. Register a `FirExtensionRegistrar` whose `configurePlugin`
installs a single `FirAdditionalCheckersExtension` containing our
checker set. Let kotlinc drive the FIR walk; checkers fire as
part of the normal diagnostic pass. Collect diagnostics out of the
`DiagnosticReporter` at the end of each file.

**Pros:** bit-for-bit the same execution path as a kotlinc plugin
build — which means checker code written for Track B is *already*
ready for Track C. Full `FirClassChecker` / `FirFunctionCallChecker`
base class inheritance, context parameters, `MppCheckerKind.Common`,
everything. Metro's checker classes would drop in with zero code
changes.

**Cons:** depends on the unstable kotlinc internal API. Every
Kotlin version bump is a maintenance event. The compat shim under
Track C.1 has to exist for Track B too.

### Option B (deferred) — Analysis API read-only walk

Use `buildStandaloneAnalysisAPISession` from the Analysis API and
manually walk each `KaSession`, invoking checker closures that
query the session for the facts they need. No `FirAdditionalCheckersExtension`
registration — we write our own dispatch loop.

**Pros:** Analysis API surface is more stable than raw FIR. Fewer
compat subprojects.

**Cons:** checkers can't inherit from `FirClassChecker` etc — they
become krit-specific closures, which means Track B code is not
reusable for Track C. The cluster would fork.

**Decision:** Option A. The cost of maintaining two checker
shapes is higher than the cost of maintaining one compat shim,
and Option A is the only path that makes Track C a packaging
exercise instead of a rewrite.

## Session management

The runner reuses the three-state session model from
`krit-types`: cold start, warm reuse, rebuild. On a classpath
change or `sourceDirs` change, the session is rebuilt; otherwise
it is retained across requests. Per-file analysis is done inside
the retained session's `analyze {}` read action.

The same [`KT-64167`](https://youtrack.jetbrains.com/issue/KT-64167)
constraint applies: `KotlinCoreEnvironment.ourApplicationEnvironment`
is a JVM-wide singleton, so only one session-read action runs at
a time inside one `krit-fir` JVM. Multiple krit-fir daemons
running in different repos are fine — the per-repo PID routing
from item 21 already handles that.

## Relationship to `krit-types`

Open question: merge `krit-fir` into `krit-types` as a second
command, or keep separate. Arguments both ways.

**Merge** (single JAR, two modes `--types` / `--check`): smaller
footprint, shared daemon, shared Analysis API session, one JVM
heap to tune. Downside: a crashing checker brings down the types
path; coupling of release cadence.

**Separate** (two JARs, two daemons): clean failure isolation,
independent JVM tuning (check-mode might want different heap
settings), parallel development. Downside: two copies of the
session build cost, double the cold-start memory.

**Recommendation:** ship Track B as separate JARs to de-risk the
first shipping version, then evaluate merging after B.4 based on
measured memory + cold-start numbers. If they're within ~20% of
double, merge.

## Definition of done

- `tools/krit-fir/` builds via `./gradlew :tools:krit-fir:shadowJar`
- Running the JAR with a sample request on a trivial `Foo.kt`
  emits one finding from a smoke-test checker that flags all
  class declarations named `Smoke`
- Daemon mode (`--daemon --port 0`) stays up across three
  sequential requests without a session rebuild
- Cold run on Signal-Android finishes inside 60 s; warm run on
  the same file set finishes inside the daemon's idle timeout
  (~30 s budget)
- Crash in one file does not prevent findings being returned for
  the other files in the same batch (per-file try/catch around
  `analyze {}`)

## Non-goals (for this concept)

- Cache integration — see [go-integration.md](go-integration.md)
- Per-rule file filtering — see [go-integration.md](go-integration.md)
- Multi-version compat shim — see
  [plugin-packaging.md](plugin-packaging.md); Track B runs against
  whatever `kotlin-compiler-embeddable` version we pin in
  `build.gradle.kts` and upgrades atomically
- Test harness — see [checker-api.md](checker-api.md)
