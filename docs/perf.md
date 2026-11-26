# Performance Tuning

Krit aims to be fast out of the box. This page lists the knobs available for
people who need to push further — large repositories, CI tight on wall-clock, or
laptops on battery. Each option lists what it changes, when it's worth using,
and what you give up.

If you're unsure where to start, the daemon plus `--diff` covers the vast
majority of "make my CI faster" asks.

## The single biggest win: run the daemon

Cold `krit .` reparses every file, rebuilds the cross-file index, and warms
type inference from scratch. That's fine for one-off runs, but in IDEs, pre-
commit hooks, and CI it adds up.

```bash
krit serve --root . &       # long-lived process per project root
krit --daemon .             # uses the running daemon
```

Subsequent calls reuse the warm parse tree, cross-file index, resolver, and
library facts. On a large Kotlin/Android repo this typically takes
analyze-project from seconds cold to a fraction of a second warm.

The daemon also keeps an LSP and MCP server in the same process, so editors
and AI agents share the warm state.

See `--no-daemon` to force the cold path explicitly.

The ready line's `module Kotlin files` count covers `.kt` files in the
discovered Gradle modules' conventional `src/<source-set>/kotlin` and
`src/<source-set>/java` roots. It is not a repository-wide inventory: Java
files, custom source layouts, and files outside those roots are not counted.
Consequently, `0 module Kotlin files` does not mean an empty repository. The
scan response reports the files actually scanned separately.

### Helper JVM lifetime and shutdown

The Go project daemon (`krit serve`) and the JVM helpers are separate
processes. Even a one-shot CLI invocation can leave persistent krit-fir
helpers running: the FIR checker and cached oracle compilation can reuse or
start TCP helpers, then release their connections without shutting down the
JVMs. Later invocations reuse their warm compiler state. Not every scan starts
a helper; cache hits and one-shot compilation paths can avoid one.

Persistent TCP helpers exit after **30 minutes waiting for a client**. A new
connection that sends no request has the same timeout. After its first
request, an established connection has no read-idle timeout, so a long-lived
client can keep a helper alive. The Go daemon's `--idle-timeout` is a separate
timer; it does not configure these JVM timeouts.

- `--no-daemon` bypasses the Go project daemon for this invocation. It neither
  stops an existing daemon nor disables persistent FIR/oracle helpers.
- `--no-fir-daemon` makes the FIR checker and the type oracle's cache-miss
  analysis use one-shot JVMs. It does not disable `--daemon`'s resident oracle
  or stop helpers already running.
- To avoid persistent helper startup for an in-process scan, combine
  `--no-daemon --no-fir-daemon` (without `--daemon` or custom rule jars).
  `--output-types` also runs one-shot under `--no-daemon` alone.
- `KRIT_EPHEMERAL_DAEMONS=1` scopes **newly started** TCP helpers to their
  launching process: they poll parent liveness every two seconds and use a
  two-minute client-wait timeout. It does not retroactively change a reused
  persistent helper.

`krit daemon stop --repo .` stops the Go project daemon. On graceful shutdown
it closes its oracle handles: owned JVMs shut down, while reused/shared JVMs
only lose that connection. It does not stop independently persistent
helpers.

`krit daemon stop --all` stops this project's Go daemon and then every
krit-fir and oracle helper recorded in the registry, for all projects. A
helper is signalled only when its recorded start time proves the live PID is
that helper, and a record another krit holds locked is left alone; stale
records are removed. The command prints how many helpers it stopped and how
many it left.

To stop a single helper by hand instead, inspect its PID record in
`~/.krit/cache/daemons/` (or `KRIT_DAEMON_REGISTRY_DIR` when set). Oracle records
use `*.pid`; FIR checker records use `*.krit-fir.pid`. Verify the live process
with `ps -p PID -o pid=,args=` and confirm that its jar and source arguments
belong to the intended project before sending `kill -TERM PID`. Do not signal
a PID solely because a record exists: it can be stale or reused. Otherwise,
release clients and let the helper's idle timeout stop it.

## Filesystem watching

When the daemon is running, it watches the project tree for edits and
invalidates exactly the cache slots that need rebuilding. Two backends:

| Backend | Platforms | Setup | Best for |
|---|---|---|---|
| `fsnotify` (default) | macOS, Linux, Windows | None | All users |
| `fanotify` | Linux 5.17+ | `setcap` (see below) | Repos with >50k directories |

Default behavior (`--watch-backend auto`):

- On Linux, probe fanotify. If `CAP_SYS_ADMIN` + `CAP_DAC_READ_SEARCH` are
  available, use it. Otherwise silently fall back to fsnotify.
- On every other platform, fsnotify.

The daemon prints which backend resolved on its `krit daemon: ready` line.

### Enabling fanotify

fanotify uses a single filesystem-wide kernel mark instead of one inotify
watch per directory. On repos with tens of thousands of directories this
avoids hitting the per-user inotify watch limit and skips the recursive walk
to register every dir.

It requires two Linux capabilities the krit binary doesn't have by default.
Grant them once:

```bash
sudo setcap cap_sys_admin,cap_dac_read_search+ep "$(command -v krit)"
```

Then restart the daemon. The `krit daemon: ready (..., watcher=fanotify)`
line confirms it took effect. If you ever re-install or upgrade krit, rerun
the `setcap` command — capabilities don't survive a binary swap.

To force one backend or the other:

```bash
krit serve --watch-backend fsnotify    # skip the fanotify probe
krit serve --watch-backend fanotify    # require fanotify; warn on fallback
```

The explicit `fanotify` value logs a warning on fallback (the user asked for
it, so a silent fsnotify swap would hide misconfigured caps). Use `auto` if
you're fine with whichever the system supports.

### When fanotify probably isn't worth it

- Small and mid-sized repos. The inotify cost is negligible.
- Networked / virtual filesystems (NFS, sshfs, virtio-fs). Many don't export
  file handles, which fanotify needs.
- CI runners that recreate the worktree per job. The daemon doesn't live
  long enough to amortize anything.

## Memory limits

### `--max-parse-bytes N`

```bash
krit serve --max-parse-bytes 2147483648    # 2 GiB cap on resident parse trees
```

Caps the bytes held by the resident parse-cache. Over the cap, the watcher's
LRU evicts least-recently-used entries. Reclaimed via `runtime.GC()` after
each eviction batch.

Default `0` = unbounded. Set this if you've seen the daemon's RSS grow
without bound on huge repos, or if you're sharing a laptop with other heavy
processes.

### `--parse-cache-cap-mb N`

Caps the **on-disk** parse-cache size (default 1024 MB; `0` = use config or
default; negative = unlimited). Independent of `--max-parse-bytes` — that
one bounds RAM, this one bounds disk.

### `--idle-timeout DURATION`

```bash
krit serve --idle-timeout 10m      # default 30m, 0 disables
```

Daemon auto-shuts down after this much time without a request. The next call
spins it back up with `krit --daemon`. Useful on laptops to avoid keeping the
Go project daemon alive overnight; shared JVM helpers have their own lifetime
(see above).

## Limit the work per scan

### `--diff REF` / `--delta REF`

```bash
krit --diff origin/main .          # findings on lines changed since main
krit --delta origin/main .         # findings *newly introduced* since main
```

The best perf knob for PR/CI runs. Both report only findings in changed
files (`--diff` only on changed lines).

With `--diff`, the JVM work is scoped to the change as well (PR mode,
#1660). With a Gradle model, the type oracle and the FIR check compile only
the modules that hold changed Kotlin files: each module with its
dependencies as built jars, or as source context when they are not built,
the way module sharding does (see [module sharding](module-sharding.md)).
Every other file keeps Go's verdict, and `--diff` drops its findings
anyway. A change with no Kotlin file starts no JVM at all. `-v` reports
`PR mode: the type oracle and FIR analyzed 3 changed files in 2 modules`,
and the FIR summary line says how many files stayed with Go.

PR mode falls back to the whole-repository compile, and says why under
`-v`, when:
- there is no Gradle model or no krit-fir jar or JDK;
- `KRIT_MODULE_SHARDS=merged` is set;
- a changed file's module would compile without some of its dependencies
  (their unbuilt sources exceed 3000 files);
- too few changed files get facts.

A dependency whose own sources changed is compiled from those sources, not
from a built jar that predates the change. PR-mode verdicts are kept out of
the FIR check cache, which holds whole-repository verdicts only, so a later
`--no-pr-mode` scan never replays them.

`--no-pr-mode` keeps the whole-repository compile. It also takes precedence
over `--daemon`'s resident oracle, and PR-mode scans run in-process rather
than through the krit daemon. Module compiles are cached by content under
`.krit/module-facts`, so rescanning an unchanged module costs no JVM.

A module compile sees its own module and its dependencies, not the rest of
the repository, so a verdict can differ from the whole-repository compile
where module boundaries matter: smart casts across modules, `internal`
visibility, and classes the merged compile reported as redeclared between
variant source sets (which PR mode no longer sees). `--delta` does not use
PR mode yet: it compares against a full scan of the base ref.

### `--max-cost TIER`

```bash
krit --max-cost ast .              # ast or below: trivial + line + ast
krit --max-cost crossfile .        # ast + crossfile
krit --max-cost oracle .           # everything except FIR
```

Rules are tagged by cost class. `--max-cost` excludes everything above the
given tier. Useful for pre-commit hooks (fast tiers only) or smoke checks
during development.

Tiers, cheapest to most expensive: `trivial` < `line` < `ast` < `crossfile`
< `oracle` < `fir`.

### `--j N`

```bash
krit -j 4 .                        # parallel job count
```

Defaults to `runtime.NumCPU()`. Lower it if you're sharing the machine; raise
it if you have headroom and Krit is CPU-bound.

## Disable subsystems for speed

These reduce precision in exchange for time. Off by default; flip when you
need the throughput more than the accuracy.

### `--no-type-inference`

Skips source-level type inference. Rules that depend on resolved types
either degrade to AST-only matching or skip entirely. Saves ~10–30% on a
typical Kotlin scan but suppresses some findings.

### `--no-type-oracle`

Skips the JVM-backed krit-fir type oracle entirely. Strictly faster
than `--no-cache-oracle`. Loses precision on rules tagged `NeedsOracle` —
they're skipped instead of running with degraded data.

### `--no-fir`

Disables the FIR checker pass (separate krit-fir JVM subprocess). FIR is
enabled by default; use this flag to disable it. Incremental FIR checking is
also on by default (see [Cache location](configuration.md#cache-location)):
it recompiles only changed files and their dependents when it can (for example,
when the previous module compile completed without errors); otherwise it falls
back to a full compile. See
[The `--fir` pass](fir-checker-candidates.md#the---fir-pass).

### `--no-fir-daemon`

Forces one-shot mode for the FIR checker and for the type oracle's cache-miss
analysis in an in-process scan. A scan delegated to the krit daemon still uses
that daemon's resident oracle, `--daemon` still starts one, and
already-running helpers are not stopped. See [Helper JVM lifetime and shutdown](#helper-jvm-lifetime-and-shutdown)
for combinations suitable for hermetic CI runners.

## On-disk caches

All caches live under `.krit/` and are safe to delete. Disabling them mostly
helps debug stale-cache symptoms; the perf cost of an enabled cache is
near-zero on a warm run.

| Flag | What it disables |
|---|---|
| `--no-cache` | Every cache that can change findings: the incremental analysis cache, the cross-file index and the cross-file findings cache, the findings bundle, the Android findings cache, the resource index, the file-walk cache, the type-index and library-profile caches, the incremental oracle cache, the `--fir` check cache `.krit/fir-cache` and the incremental FIR bundle `.krit/fir-incremental`. None is read or written. The parse cache stays on. A daemon-delegated scan forwards the flag, and the daemon also skips its parse cache for that call |
| `--no-parse-cache` | Tree-sitter parse-tree cache |
| `--no-resource-cache` | Android values-XML ResourceIndex cache |
| `--no-cross-file-cache` | Cross-file index cache |
| `--no-cache-oracle` | Incremental oracle cache (forces full JVM run) |
| `--no-matrix-cache` | Experiment-matrix baseline cache |
| `--clear-cache` | Delete all caches (including `.krit/fir-cache` and `.krit/fir-incremental`) and exit |
| `--clear-matrix-cache` | Delete the experiment-matrix baseline cache and exit |
| `--cache-dir PATH` | Override incremental cache directory |
| `--store-dir PATH` | Use the unified store-backed cache |

### Cache inventory and invalidation

Every cache below sits under `<repo>/.krit/` unless noted. "Build" means the
running Krit build: `buildid.Token()` is the VCS revision of a clean `go build`,
or a SHA-256 of the executable when the tree is dirty or carries no VCS data.
It is stable across runs of one binary and differs between any two builds, so
dev builds that all print `krit dev` still invalidate each other's caches. The
rule hash that keys findings is `cache.ComputeCacheKeyHash`: active rule IDs,
the resolved config, the editorconfig switch and the build, with the oracle jar
and schema identity mixed in.

Findings and rule-dependent derived data. A different build never reads these:

| Cache | Holds | Key inputs | Invalidated by |
|---|---|---|---|
| `cache/` (per-file analysis) | Per-file rule findings | File content hash and git blob ID, rule hash, oracle fact hash | Rule hash: rules, config, build, oracle jar, and project facts (module SDK levels, Gradle library facts) |
| `findings-bundle-cache/` | Whole-run findings and delta manifests | Run fingerprint: rule hash, source set, cross-file structure, oracle facts, Gradle paths and library facts, compiler settings, XML inputs (manifests, resources, module SDK levels) | Any fingerprint field |
| `cross-findings-cache/` | Cross-file rule findings | Cross-file index fingerprint, rule hash | Index fingerprint (sources, XML inputs, build), rule hash |
| `cross-file-cache/` | Cross-file symbol and reference index | Source and XML content hashes, build, bloom library version | `CrossFileCacheVersion`, the shard version, the build |
| `android-findings-cache/` | Manifest, resource, icon, Gradle, resource-source and manifest-source findings, merged resource indexes | Rule hash (with module SDK levels and RTL main-project facts), library facts, Java semantic facts, content fingerprint of the unit (a manifest also keys on its path and the module's `res/` and build script) | `androidFindingsCacheVersion`, any key input |
| `fir-cache/`, `fir-incremental/` | FIR checker verdicts and incremental compiler state | Source hash, classpath, jar identity, rules and configs, per-file SDK levels, Kotlin settings; `fir-cache/` keys paths relative to the repository and jars by content, so it survives a move to another checkout path or `GRADLE_USER_HOME` | `FirCacheVersion`, the fingerprint, the build |
| `types.json`, `types-cache/` | Type oracle facts | File hashes, backend jar token, `CacheVersion`, fact schema | Jar contents, schema versions, build |
| `type-index-cache/` | Source type index (scopes, smart casts, declarations) | File path and content hash | `typeIndexCacheVersion`, the build (the directory is emptied when another build wrote it) |
| `librarymodel-cache/` | Gradle project profile (SDK levels, dependencies, catalogs) | Gradle and catalog file hashes | `projectProfileCacheVersion`, the build |
| `resource-cache/` | Parsed values, layout and drawable `ResourceIndex` per res dir | Content fingerprint of the dir, scan kinds | `resourceCacheVersion`, the build |
| `parse-cache/xml/` | Parsed XML node trees | File content hash | `xmlParseCacheVersion`, grammar version, the build |

Pure parse and discovery caches. They hold no rule output and change only with
the grammar or the repair code, so they invalidate by version constant and a
change to that code must bump it:

| Cache | Holds | Invalidated by |
|---|---|---|
| `parse-cache/kotlin/`, `parse-cache/java/` | Tree-sitter trees after pre-parse rewrites and repair | `parseCacheVersion`, tree-sitter and grammar version, `kotlinTreeRepairRevision` |
| `filewalk-cache/` | Directory listings per scan root | `filewalkCacheVersion`, filters, directory mtimes |
| `tracked-files-cache/` | `git ls-files` listing | `trackedFilesCacheVersion`, git HEAD and index state |
| `resource-graph-cache/` | Resource definition and reference graph | `resourceGraphVersion`, resource content |
| `gradle-model/` | Gradle export written by `kritExportModel`, not by Krit | Re-exporting the model |

Outside the repository: the experiment-matrix baseline cache
(`~/.cache/krit/matrix-baseline`, keyed on the executable hash), the
executable-hash memo (`buildid-hashes.json` in the user cache dir), and the AOT
and daemon state under the system temp dir.

Project-level inputs are part of the keys, not only file content. A layout's
findings can depend on a manifest (`supportsRtl`, `targetSdkVersion`), on which
apps include the module, and on a module's `minSdk` from `build.gradle(.kts)`,
none of which change the layout's own hash. The Android phase hashes the
module SDK levels and the RTL main-project facts into its rule hash, and the
per-file analysis cache adds the SDK levels and the Gradle library facts.

Classpath jars, class directories' files and the `krit-fir` jar are keyed by
path and content, never by size and modification time, so a key is the same
in every process for the same bytes. The hash memos make that cheap: the
in-process memo and `hash-memo.gob` (and `buildid-hashes.json` for the
`krit-fir` jar) serve an unchanged file's hash for one stat.

Those memos, the per-file analysis cache's mtime fast path and directory
listings still trust a stat. Filesystems stamp mtimes from a coarse clock
(4 ms on ext4, a second on HFS+, 2 s on FAT), so a same-size rewrite in the
same tick keeps the stat. Krit applies git's racy-clean rule with a 2 s
window (`hashutil.RacyWindow`): a stat whose mtime is not at least 2 s older
than the moment it was taken is never memoized, and an analysis-cache entry
for such a file is recorded with an mtime no file has, so the next run checks
the content. "Now" is the clock of the filesystem holding `.krit/`, measured
each run by writing and stat-ing a file there, so a network or VM mount with
a skewed clock is judged by its own clock; on that filesystem an mtime more
than 2 s in the future is trusted too, since no write made now can reproduce
it. Files on other filesystems are judged by the local clock.

In a git checkout, `cache/` serves a file without a stat or a content hash
only when git vouches for it: `git diff` reports the file unchanged from the
index, and the index's blob ID equals the blob ID of the content the entry was
computed from. That is what makes a `cache/` written at another commit safe
to restore (a CI cache restored by prefix, or a checkout or pull between two
runs): a file that changed between the commits has another blob ID and is
re-analyzed. Dirty, unmerged, `--assume-unchanged`, skip-worktree, symlinked
and submodule paths fall back to the stat and content-hash check.

If a scan with the default cache disagrees with `--no-cache`, that is a bug in
a key: run `krit --clear-cache` to recover, then report which cache held the
stale data (move cache directories aside one at a time to find it).

### When the oracle cache re-runs the JVM

The oracle and FIR checker reuse a persistent helper only while its jar is
unchanged. Replacing the jar starts a fresh helper and stops the old one for
that project.

The oracle cache stores each Kotlin file's compiler facts and reuses them while
the file is unchanged. With incremental FIR checking enabled, krit-fir
recompiles only changed files and their dependents when it can (for example,
when the previous module compile completed without errors); otherwise it falls
back to a full compile. It reports facts for all source files, so any run can
refresh the whole cache.
Each cached entry also records the compilation it came from: every Kotlin
file's path and content, the classpath, and the krit-fir jar. When nothing krit
needs has changed but the compilation has (for example, a file outside the oracle
filter was edited or deleted, or a library was updated), krit runs the
compiler once to refresh the cache.

## Profiling

When something's slow and you want a fact instead of a theory:

```bash
krit --perf .                              # wall-clock timing in output
krit --perf-rules .                        # plus per-rule ranking (stderr table)
krit --profile-dispatch .                  # per-file dispatch distribution
krit --cpuprofile cpu.prof .               # standard pprof CPU profile
krit --memprofile mem.prof .               # heap profile
```

Then:

```bash
go tool pprof -http=:8080 cpu.prof
go tool pprof -http=:8080 mem.prof
```

Verbose `--fir` scans print an `incremental FIR:` line when planner or
compiler-error telemetry is available, and explain when `--no-cache` disables
snapshot persistence. The same message appears as the `incremental` detail on
the `firCheckOutcome` perf entry; an all-cache-hit run has no planner line.

In daemon mode `--cpuprofile` and `--memprofile` profile the daemon process,
not the short-lived CLI. Pair with `--no-daemon` if you want to profile a
single in-process run instead.

## Configuration-level perf

Most flags above have config-file equivalents. See `docs/configuration.md`
for the schema. The most useful for CI tuning:

- `excludes:` — global glob excludes prune files before they reach the
  dispatcher
- `rules.<RuleName>.excludes:` — per-rule excludes for noisy spots
- `strict: true` — drops `NoisinessNoisy` rules globally

## Recommended setups

**Pre-commit hook:** `krit --diff HEAD --max-cost ast --no-fir .`

**PR CI:** `krit --diff origin/main .` (PR mode compiles only the changed
modules; restore `.krit/` from the base branch to reuse its module cache)

**Full repo daily lint:** `krit --daemon .` (or no daemon if the runner is
ephemeral)

**Laptop dev daemon (battery-aware):** `krit serve --idle-timeout 10m
--max-parse-bytes 2147483648`

**Power-user setup on Linux:** `setcap` as above, then `krit serve` (auto
picks fanotify).

## Performance regression guard

Every pull request runs the `perf-guard` CI job (`scripts/perf-guard.sh`,
or `make perf-guard` locally). It builds krit at the PR's base and at its
head and times both **on the same runner**, alternating them for five
rounds and flipping which one goes first each round. It compares medians.
There is no stored baseline: shared CI runners vary by tens of percent
between machines, but two binaries timed side by side on one machine agree
within a few percent. An A/A run (base = head) differed by 0.3% on total
time.

The corpora are the pinned `nowinandroid` and `okhttp` commits from
`tests/corpus/repos.json`, a synthetic Android-shaped project (`perfguard
gen`: modules, manifests with activities/services/receivers/providers,
component classes behind base classes, Compose functions, resources), and
`playground/android-app`. Each scan is source-only and cold: `--no-fir --no-type-oracle --no-daemon`,
every on-disk cache off (`--no-cache --no-parse-cache`; it also passes
`--no-cross-file-cache --no-resource-cache`, which `--no-cache` now implies),
and the corpus's `.krit/` deleted before each run.

The job fails when head is slower than both thresholds of a gate:

| Metric | Gate |
|---|---|
| Total `durationMs` | > 15% and >= 300ms |
| A `perfTiming` phase | > 25% and >= 200ms |
| A rule (`perfRuleStats`) | > 30% and >= 100ms |
| A rule's share of rule time | newly above 25% (and >= 100ms slower) |
| A rule new in head | >= 250ms |

The **scaling check** times head on the synthetic project at two sizes, 6×
apart. It fails when a leaf phase or a rule that costs >= 500ms grows more
than 12× (2× worse than linear). It times the base binary at both sizes too, and a metric the base already scales as badly (exponent within 0.15) is reported but doesn't fail. A PR fails only for scaling it made worse, not for a borderline phase already on main. That catches O(n²) work, such as #1658's
per-call class map, independently of runner speed.
`internal/devtools/perfguard/known-superlinear.txt` lists the phases already
known to be super-linear, each with its issue. The list only shrinks: when
the fix lands, the guard fails until the fix's PR deletes the line.

The job posts the comparison as a sticky PR comment, showing the top phases
and rules with their deltas and the rules new in head, so reviewers see
what a rule port costs. When a regression is real but intended, say why in
the PR. The gates are flags on `perfguard compare` / `perfguard scale`.

## When to file a perf issue

If a warm-daemon `analyze-project` takes more than ~300ms on a mid-sized repo,
that is likely a bug. Attach:

- `krit --perf-rules` output
- `krit --version`
- Daemon's first line on startup (`krit daemon: ready ...`)
- A reproducer if possible (or repo size + ratios)

File against [kaeawc/krit](https://github.com/kaeawc/krit/issues).
