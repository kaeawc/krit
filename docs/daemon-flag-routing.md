# Daemon flag routing

Krit's CLI auto-routes `krit <paths>` to a reachable daemon when the requested
flag set falls inside `daemonCompatibleFlags(f)` in
`internal/cli/scan/daemon_delegate.go`. Most analysis flags are eligible; the
flags listed below are intentionally held back on the in-process path.

This doc records *why* each flag stays in-process so future contributors do not
silently route them through the daemon and break user-visible behavior. Each
section also notes what would have to change for daemon routing to become a
sensible option.

## Every scan flag

Every flag of the scan verb has one of three routes:

- **forwarded**: a delegated scan sends it to the daemon, as a field of the
  analyze-project request or as a meta verb in its place.
- **in-process**: setting it keeps the scan in-process.
- **not needed**: the daemon needs nothing for it. The CLI applies it to the
  daemon's result, it only picks the daemon, or it only matters next to a flag
  that already runs in-process.

A flag that is in none of these groups would be dropped silently by a delegated
scan, which then reports different findings than the same command in-process
(#1956). `internal/cli/scan/daemon_flag_routing_test.go` lists every flag in
one of three lists (`forwardedScanFlags`, `inProcessScanFlags`,
`irrelevantScanFlags`). It fails when a flag is in no list, sets each flag and
checks the delegation decision matches its list, and checks this table matches
the lists. A new flag needs an entry there, code that does what the entry says
(`buildDaemonAnalyzeArgs` sends it, or `daemonCompatibleFlags` keeps the scan
in-process) and a row here.

| Flag | Route | How it reaches the daemon, or why not |
|---|---|---|
| `-f`, `--format` | forwarded | `format`; checkstyle and plain are rendered by the CLI from the shipped columns |
| `--report` | forwarded | `format`; a `FORMAT=PATH` value runs in-process (see below) |
| `-o` | not needed | the CLI writes the daemon's report to the file |
| `-q` | not needed | the CLI prints the summary lines |
| `-v` | forwarded | `verbose`; the daemon streams its verbose output back |
| `-j` | not needed | worker count; findings do not depend on it |
| `--perf`, `--perf-rules` | forwarded | `show_perf`, `perf_rules` |
| `--profile-dispatch` | forwarded | `profile_dispatch` |
| `--cpuprofile`, `--memprofile` | forwarded | absolute paths; the daemon process is profiled |
| `--reproducible` | in-process | the daemon renders the envelope with its own timings and cache statistics |
| `--include-generated` | forwarded | `include_generated` |
| `--fix`, `--fix-binary`, `--remove-dead-code` | forwarded | `include_columns`; the CLI applies the fixes to the daemon's findings |
| `--fix-level` | forwarded | `fix_level` |
| `--dry-run` | forwarded | `dry_run` |
| `--fix-suffix`, `--finding-id` | not needed | the CLI applies the fixes |
| `--all-rules`, `--experimental`, `--enable-experimental` | forwarded | `all_rules`, `experimental` |
| `--strict` | forwarded | `strict` (#1956: the field existed, the CLI did not set it) |
| `--enable-rules`, `--disable-rules` | forwarded | `enable_rules`, `disable_rules` |
| `--disable-related` | forwarded | the CLI adds the related rules to `disable_rules` |
| `--max-cost`, `--only-scope`, `--exclude-scope` | in-process | the request carries no cost or scope filter |
| `--min-confidence`, `--warnings-as-errors` | forwarded | `min_confidence`, `warnings_as_errors` |
| `--baseline`, `--create-baseline` | forwarded | `baseline`, `create_baseline`; the CLI writes the new baseline file |
| `--base-path`, `--absolute-paths` | forwarded | `base_path` (absolute), `absolute_paths` |
| `--config` | in-process | unless it names the config at the daemon's root, which the daemon loads |
| `--detekt-config`, `--no-detekt-config` | in-process | the request carries no detekt options |
| `--enable-editorconfig` | in-process | rewrites the config; the daemon keeps one resident config |
| `--experiment`, `--experiment-off` | in-process | process-wide experiment set, and no findings cache is keyed on it |
| `--no-cache` | forwarded | `no_cache` |
| `--no-parse-cache` | forwarded | `no_parse_cache`, confirmed by `parse_cache_bypassed` |
| `--no-cross-file-cache` | in-process | the daemon also keeps a code index in memory; with `--no-cache` the scan is delegated |
| `--parse-cache-cap-mb` | in-process | the daemon's parse cache is resident and keeps its default cap |
| `--cache-dir`, `--store-dir` | in-process | the daemon keeps its caches in its own state directory |
| `--no-resource-cache` | not needed | the daemon keeps no on-disk resource index cache |
| `--clear-cache`, `--clear-matrix-cache` | in-process | a running daemon's clear verbs serve them first; with no daemon they clear in-process |
| `--no-matrix-cache` | not needed | read by `--experiment-matrix`, which runs in-process |
| `--no-type-inference` | forwarded | `no_type_oracle`: in-process the flag turns the type oracle off, and source inference still runs |
| `--no-type-oracle`, `--no-cache-oracle` | forwarded | `no_type_oracle`, `no_cache_oracle` |
| `--oracle-backend` | forwarded | `oracle_backend`, validated by the daemon |
| `--no-oracle-filter` | in-process | changes which files the resident type oracle loads; it is the findings-equivalence baseline for the filter |
| `--daemon` | not needed | picks the krit-fir oracle daemon of an in-process scan; the krit daemon keeps its oracle resident |
| `--input-types` | forwarded | `input_types_path` (absolute) |
| `--output-types` | forwarded | `dump-types` meta verb |
| `--dump-oracle-diagnostics` | in-process | meta command with no daemon verb |
| `--gradle-model`, `--no-gradle-model` | forwarded | `gradle_model`, `no_gradle_model` |
| `--generated-srcjar` | forwarded | the CLI unpacks the jars into `oracle_source_dirs` and `oracle_generated_source_dirs` |
| `--gradle-project-outputs` | in-process | the request carries no project outputs |
| `--custom-rule-jars` | forwarded | `custom_rule_jars` |
| `--fir`, `--no-fir` | forwarded | `fir`, `no_fir` |
| `--no-fir-daemon` | forwarded | `no_fir_daemon` |
| `--fail-on-degraded` | not needed | the CLI applies it to the daemon's FIR status |
| `--min-fir-coverage` | in-process | the daemon's delta and fix paths carry no coverage gate |
| `--min-sdk`, `--target-sdk` | in-process | set a process-wide default a daemon must not keep |
| `--diff` | forwarded | `diff`; in PR mode the scan runs in-process (see below) |
| `--delta` | forwarded | `include_columns`; the CLI filters against the base scan. In PR mode the scan runs in-process |
| `--no-pr-mode` | not needed | without it a PR-mode scan runs in-process; with it the scan is delegated as usual |
| `--pr-files` | not needed | PR mode of `--delta`'s base scan, which runs in-process when there is JVM work |
| `--depth` | forwarded | the preset's flags (`fast` sets `no_type_oracle`) |
| `--sample-rule`, `--sample-count`, `--sample-context` | not needed | the CLI samples the daemon's JSON findings |
| `--rule-audit`, `--baseline-audit` | forwarded | `include_columns`; the CLI runs the audit |
| `--rule-audit-min-findings`, `--rule-audit-details`, `--rule-audit-samples`, `--rule-audit-context`, `--rule-audit-cluster` | not needed | the CLI runs the audit |
| `--list-rules`, `--validate-config`, `--oracle-filter-fingerprint`, `--list-experiments` | forwarded | meta verbs of the same names |
| `--maturity`, `--cwe` | not needed | read by `--list-rules`, which sends them in the list-rules verb |
| `--version`, `--init`, `--doctor`, `--completions`, `--generate-schema` | in-process | meta commands about the calling binary (see below) |
| `--experiment-matrix`, `--promote-experiment`, `--deprecate-experiment`, `--new-experiment` | in-process | experiment workflow tools (see below) |
| `--experiment-candidates`, `--experiment-intent`, `--experiment-runs`, `--experiment-targets` | not needed | read by `--experiment-matrix`, which runs in-process |
| `--new-experiment-description`, `--new-experiment-intent`, `--new-experiment-target-rules`, `--new-experiment-wire-file` | not needed | read by `--new-experiment`, which runs in-process |
| `--no-daemon` | in-process | asks for an in-process scan |
| `--daemon-socket` | not needed | picks the daemon |

## Working directory

A daemon is found by repo directory, not by working directory, so one daemon
serves `krit` invocations from any directory under its root and starts in
whichever directory first spawned it. Scan paths go on the wire exactly as
typed, together with the caller's working directory (`Cwd` on
`AnalyzeProjectArgs`, `OracleFilterFingerprintArgs` and `DumpTypesArgs`). The
daemon enters that directory before resolving anything (`enterCallerCwd` in
`internal/cli/serve/caller_cwd.go`), so relative paths mean what they meant to
the caller and findings come back in the caller's spelling, identical to the
in-process run.

Consequences worth knowing when adding a verb or resident state:

- The working directory is process-wide. Every verb that resolves a
  caller-relative path must hold `analyzeMu` and call `enterCallerCwd`.
- Resident state keyed by relative path spellings is dropped when the caller's
  directory changes, and the next scan runs cold. On-disk caches are shared
  with in-process runs and must not assume one spelling per location; the
  findings-bundle manifest key covers both the absolute path and the spelling.
- A request whose directory the daemon cannot enter fails, and the CLI falls
  back to the in-process path.

## Socket location

The daemon listens on `<root>/.krit/daemon.sock`, where `<root>` is the
absolute, symlink-resolved project root. A unix socket path is limited to 108
bytes on Linux (104 on macOS), so for deeply nested checkouts the conventional
path would make `bind` fail. When the conventional path is longer than 100
bytes, the socket moves to `<dir>/<16 hex of sha256(root)>.sock`, where `<dir>`
is the first usable of `$XDG_RUNTIME_DIR/krit`, `$TMPDIR/krit-<uid>` and
`/tmp/krit-<uid>` whose full path fits (relative `TMPDIR`/`XDG_RUNTIME_DIR`
values are skipped). That directory is created `0700`; an existing one must be
a real directory (not a symlink) owned by the current user, and its parent
must be owned by us or root and not group/other-writable without the sticky
bit. Lookups (`status`, discovery) only inspect these directories; creating
them happens when a daemon is spawned or binds. If none qualifies, spawning
fails with an error naming each candidate.

How clients and daemons stay in agreement and stay safe:

- **One daemon per repo.** `krit serve` takes the per-repo flock
  (`.krit/daemon.lock`, which also writes `.krit/daemon.pid`). A second serve
  for the same root exits with status 2 and "another daemon is running for this
  repo", even if its environment would derive a different socket path. A client
  whose spawned child exits 2 keeps waiting for the winner.
- **Spawn.** The client passes the socket it resolved to the child it spawns
  (`krit serve --root R --socket S`), so a different `TMPDIR` in the spawned
  environment cannot move it.
- **Pointer file.** A daemon on a fallback socket must write the real path to
  `<root>/.krit/daemon.sock.path` (mode `0600`, atomic temp file and rename);
  failing to do so aborts startup. It removes the pointer on stop. A daemon on
  the conventional socket removes any stale pointer at startup. Clients only
  honor a pointer when the conventional path is too long, the file is a
  regular file owned by the user and small, and its target is
  `<krit dir>/<this root's hash>.sock`; the target is tried first and, if it
  does not answer, the derived path is tried, so a stale pointer cannot hide a
  live daemon. `krit daemon status --json` reports the real `socketPath`;
  tools should read that rather than hard-coding `.krit/daemon.sock`.
- **Peer and root checks.** Every connection is checked for the peer uid
  (`SO_PEERCRED` / `LOCAL_PEERCRED`) on both ends: the daemon drops
  connections from other users and clients will not talk to a daemon of
  another user. Discovery treats such a daemon as absent, but
  `EnsureRunning`/`EnsureCompatible` stop with "daemon at X runs as uid N; stop
  it as that user or use --no-daemon" instead of spawning over it. Discovery
  also asks the daemon for its `status` and requires the reported root to equal
  the requested root (for every socket, since `.krit` can be a symlink in a
  repository); a mismatch is refused with an error and no daemon is spawned
  over it. An explicit `--daemon-socket` skips the root check: it is the
  caller's choice of daemon.

The `daemon` package is Unix-only today (the repo lock uses `syscall.Flock`).

## Meta / non-analysis flags

These flags do not run an analysis at all. The daemon's only client verb is
`AnalyzeProject` (see `buildDaemonAnalyzeArgs` in
`internal/cli/scan/daemon_delegate.go`); routing meta commands through it
would require new wire verbs and would still produce confusing semantics
because the answer the user wants is about the *calling* CLI, not a
long-lived background process.

### `--init`

`runInitFlag` at `internal/cli/scan/early_exits.go:140` writes `krit.yml` into
the *calling process's* current working directory and short-circuits before any
analysis. The daemon may have been started from a different working directory
(it is keyed by repo dir, not CWD) and lives across many shells, so writing the
config from the daemon side would either land in the wrong place or require
extra wire plumbing for "create file at this absolute path on the client" — at
which point the daemon adds nothing over the direct `os.WriteFile` call.

To route through the daemon we would have to standardize all path-sensitive
write operations on a "resolve relative to client CWD" wire convention; not
worth it for a one-shot bootstrap command.

### `--doctor`

`runDoctorFlag` at `internal/cli/scan/early_exits.go:159` reports diagnostics
about the *calling* CLI's environment: the binary's version, `exec.LookPath`
results for `java` and `cwebp`, and whether the `krit-fir` oracle jar
exists on disk. Routing through the daemon would
report the daemon process's PATH and binary version, which can drift from the
shell the user just ran the command in. For a diagnostic command that is
actively wrong.

To route through the daemon we would have to (a) add a wire verb that returns
raw env probes and (b) accept that the answer no longer reflects the user's
shell. Both defeat the purpose of `--doctor`.

### `--version`

`runVersionFlag` at `internal/cli/scan/early_exits.go:40` prints the CLI's
linker-injected version string. The daemon's binary hash is checked separately
via `daemonclient.CurrentBinaryHash()` (used in `buildDaemonAnalyzeArgs`); when
the CLI and daemon binaries diverge the daemon will reject requests with
`IsBinaryHashMismatch`. Asking the daemon for its version when the user typed
`krit --version` would mask exactly the mismatch the user is trying to debug.

Daemon routing would only make sense if `--version` deliberately reported both
versions; today it reports one, and that one should be the CLI.

### `--completions`

`runCompletionsFlag` at `internal/cli/scan/early_exits.go:94` prints a static
shell completion script embedded into the CLI binary via `//go:embed
completions`. There is no project context, no analysis, and the script content
is a property of the CLI build. Routing through the daemon would just add a
network hop to a `cat` of an embedded asset.

Daemon routing would only become interesting if completions ever became
project-aware (e.g. listing rule IDs from the live registry); that would be a
new feature, not a change to this flag.

### `--generate-schema`

`runGenerateSchemaFlag` at `internal/cli/scan/early_exits.go:208` builds the
JSON schema from `schema.CollectRuleMeta()` — the rule descriptors compiled
into the running binary. The daemon's view of rules can be stale relative to
the CLI when binaries differ; serving the schema from the daemon would produce
a schema that does not match the binary the user just invoked.

Daemon routing only becomes safe once the binary-hash gate is extended to
*reject* schema requests across binary versions, at which point the user is
back to needing the CLI binary anyway.

## Registry / experiment mutators

These flags rewrite Go source under `internal/experiment/` and run `go build`.
They are workflow tools for krit contributors, not for analyzing user code, and
they require a writable repo checkout — not a shared long-lived service.

### `--promote-experiment`, `--deprecate-experiment`

Both flow through `PromoteExperiment` at
`internal/cli/scan/experiment_lifecycle.go:115`, which reads
`internal/experiment/experiment.go` (`experimentCatalogPath` at
`internal/cli/scan/experiment_lifecycle.go:17`), rewrites the Status field for
the named experiment, writes the file back, runs `go build ./...`, and reverts
the file if the build fails. Routing through the daemon would mean the daemon
process holds the rewrite lock, which races with any other developer or agent
editing the same file in the same checkout, and the post-rewrite `go build` is
a property of the dev box's toolchain, not the daemon's.

To route through the daemon we would have to define how concurrent write
attempts coordinate across multiple shells and how the daemon's `go build`
toolchain relates to the user's. Today the simpler in-process model is the
right call.

### `--new-experiment`

`runNewExperimentScaffoldFlag` at `internal/cli/scan/early_exits.go:223` calls
`RunNewExperimentScaffold` (`internal/cli/scan/new_experiment.go:26`), which
appends to `internal/experiment/experiment.go` and edits the rule wire-file
specified by `--new-experiment-wire-file`. Same write-coordination and toolchain
arguments as `--promote-experiment`.

### `--experiment-matrix`

`runExperimentMatrixFlag` at `internal/cli/scan/experiment_matrix.go:78`
re-execs the krit binary in a child process for every matrix case
(`runExperimentMatrixCase` at `internal/cli/scan/experiment_matrix.go:298`),
varying `--enable-rules` and other flags per case. The matrix orchestrator
*expects* to spawn fresh processes — the whole point is to measure independent
runs and rule-set permutations. A long-lived daemon would defeat that.

Daemon routing only becomes interesting if a future redesign drives matrix
cases as parallel daemon RPC calls instead of subprocesses; that is a much
larger change than wiring a flag through.

## Audits that ship with the analysis

`--baseline-audit` and `--rule-audit` are **now routed via daemon**. The
daemon emits an optional `columns` segment in the analyze-project response
when `AnalyzeProjectArgs.IncludeColumns` is true (the CLI sets it whenever
`--baseline-audit`, `--rule-audit`, `--delta`, `--fix`, `--fix-binary`,
or `--remove-dead-code` is present). The CLI deserialises the segment
into a `*scanner.FindingColumns` and replays
`RunBaselineAuditColumns` (`internal/cli/scan/baseline_audit.go:34`) /
`RunRuleAuditColumns` (`internal/cli/scan/rule_audit.go:75`) locally,
producing the same audit output the in-process flow does. The non-audit
common path stays on the original `{findings,stats[,dispatch_profile]}`
envelope: the `columns` field is `omitempty` and the fast-scan response
decoder treats it as another optional segment after `dispatch_profile`
(see `scanOptionalColumns` in `internal/daemon/response_scan.go`).

## Fixes that ship with the analysis

`--fix`, `--fix-binary`, and `--remove-dead-code` are **now routed via
daemon**. The same `columns` wire segment used by `--rule-audit` /
`--baseline-audit` / `--delta` carries the post-pipeline
`FindingColumns` — which serialise `FixPool` / `BinaryFixPool`
alongside the finding rows (see `internal/scanner/findings_json.go`)
— so no new fix-payload schema was required. The CLI flips
`IncludeColumns=true` in `buildDaemonAnalyzeArgs` whenever any of the
three flags is set.

The daemon-side pipeline never invokes `FixupPhase` apply because
`args.Fix` / `args.FixBinary` are not forwarded across the wire —
`runFixupPhase` (`internal/pipeline/project.go`) short-circuits when
none of `Fix`, `FixBinary`, `DryRun` is set, returning the raw
`CrossFileResult` unchanged. CLI-side replay:

- `runDaemonFix` (`internal/cli/scan/daemon_fix.go`) decodes the
  columns, runs `pipeline.FixupPhase{}` locally with
  `Apply=*f.Fix && !*f.DryRun`, `ApplyBinary=*f.FixBinary`, and the
  resolved `MaxFixLevel`, then replays the same
  `info: Applied N fix(es)…` / `info: %d binary fix(es) applied.`
  lines `runner.printFixupResult` emits in-process. The same short-
  circuit conditions (`--output==""`, `--format=json`, `--report==""`,
  `--fix`) drive an early return with exit code 0 / 1; otherwise the
  daemon-routed flow re-renders the post-strip columns through
  `pipeline.OutputPhase` locally so the findings emit matches the
  in-process flow.

- `runDaemonRemoveDeadCode` decodes the columns and calls
  `RunDeadCodeRemovalColumns(cols, format, *f.DryRun, *f.FixSuffix)`
  — the same entry point the in-process `applyBaselinesAndDiff` uses.

This preserves the "daemon never writes user files" invariant: every
`os.WriteFile`, `os.Rename`, and `cwebp` / `optipng` subprocess
launched by `fixer.ApplyAllFixesColumns` /
`fixer.ApplyBinaryFixesBatchColumns` runs with the CLI's CWD and
permissions, not the daemon's.

## Oracle I/O & sampling

This bucket lists flags that look related to `--input-types` (which IS daemon-
routable) but stay in-process for distinct reasons.

### `--output-types` (now routed via daemon)

`--output-types` routes through the daemon's `dump-types` meta verb
(`handleDumpTypes` at `internal/cli/serve/meta_verbs.go`). The verb runs
`scan.RunOutputTypesTo` against the requested scan paths, which locates
the `krit-fir` jar, invokes the JVM (or honours `--no-cache-oracle` to bypass
the cache), and writes the resulting oracle JSON to the path the CLI provides.
The CLI absolutizes `--output-types` before forwarding (same convention as
`--input-types` and `--cpuprofile` / `--memprofile`) so the daemon — which has
its own CWD — writes to the user's intended location. Captured stderr and exit
code ride back in `MetaResult` and are replayed against the CLI's streams so
daemon-routed and in-process invocations remain byte-equivalent.

This routes the dump through the same resident oracle cache the daemon
populates for analyze-project, so warm `--output-types` calls land on a hot
cache instead of paying full JVM warmup again. The wire carries the CLI's
oracle compile context (classpath, model source roots, JVM target and Kotlin
settings) and `--no-fir-daemon`, as `AnalyzeProjectArgs` does.

### `--delta`

`--delta --no-pr-mode` is **routed via daemon** for the current-tree scan
portion (in PR mode it stays in-process, see below).
The CLI still owns the `git worktree add` + re-exec pass that produces the
base-ref snapshot (daemon-side worktree management would add filesystem-
mutating side effects to a long-lived service — explicitly out of scope).
Once the daemon serves the current-tree scan with `IncludeColumns=true`
the CLI applies `filterColumnsNewSince` (`internal/cli/scan/delta.go`)
locally against the deserialised `FindingColumns` and re-emits the filtered
set via `pipeline.OutputPhase{}.Run` so format, base path, and warning
promotion match the in-process flow. The wire addition was shared with
`--rule-audit` / `--baseline-audit`; see "Audits that ship with the
analysis" above for the response-shape rationale.

### `--diff` and `--delta` (PR mode)

`--diff REF` and `--delta REF` stay in-process unless `--no-pr-mode` is
given (#1660), and so does `--delta`'s base-ref scan (`--pr-files`). A
delegated `--delta` would analyze the current tree whole while its base scan
ran in PR mode, and every unchanged file's FIR verdict would differ between
the two sides. A scan with no JVM work (`--no-fir --no-type-oracle`, or no
Java launcher found) is routed as before, since PR mode changes nothing for
it (`prModeRequested`). PR mode compiles only the modules that hold the changed Kotlin files for the type
oracle and the FIR check (`runPRModules` in `internal/pipeline/index.go`),
and the analyze-project wire carries no changed-file list. To route it, the
wire would need the PR file list and the daemon's index phase would need to
pass it through as `IndexInput.PRFiles`. With `--no-pr-mode` the scan is
eligible as before and the daemon applies the `--diff` filter itself (for
`--delta`, the CLI filters the daemon's columns as described above).

## Rule-set filters and reproducible output

### `--only-scope`, `--exclude-scope`, `--max-cost`

`AnalyzeProjectArgs` carries `--enable-rules`, `--disable-rules`,
`--all-rules` and `--experimental`, but no scope or cost filter, so a delegated
scan would run the full rule set and report findings the caller filtered out.
`daemonCompatibleFlags` keeps these scans in-process. Routing them would need
the filters on the wire and in the daemon's findings-cache key.

### `--reproducible`

The daemon renders the JSON envelope, with its own duration and cache
statistics, and the wire has no switch to drop them, so a `--reproducible`
scan stays in-process.

### `--report FORMAT=PATH`

The daemon renders one report per request, so a scan writing several report
files (#1578) stays in-process and renders all of them from one result set.

### `--gradle-project-outputs FILE`

The analyze-project wire does not carry the built outputs of project
dependencies (#1581), so a scan that reads them stays in-process.

### `--min-sdk N`, `--target-sdk N`

These set the Android SDK levels of files whose build files declare none
(#1682). The analyze-project wire does not carry them, and the default they set
is process-wide (`android.SetDefaultSDKLevels`), which a daemon serving other
scans must not keep. So a scan with either flag stays in-process.

### `KRIT_FIR_DAEMON_POOL`

This environment variable keys the FIR check daemon by a pool slot instead of
the scan's source dirs, so the per-project scans of one build share a warm JVM
(#1582). A delegated scan would run with the daemon's environment, not the
caller's, so a scan with the variable set stays in-process.

## Cache clears

### `--clear-cache` (now routed via daemon)

Routes through the `clear-cache` verb (`handleClearCache` in
`internal/cli/serve/clear_cache.go`). The daemon serialises against any
concurrent `analyze-project` on `state.analyzeMu`, then calls
`cacheutil.ClearAll`, removes the on-disk analysis-cache file, resets the
parse cache, drops every resident `WorkspaceState` slot, and clears the
manifest cache so the next analyze rebuilds from cold rather than
resurrecting state from in-memory snapshots that no longer have a disk
backing.

When the daemon socket is unreachable (or `--no-daemon` is passed) the CLI
falls back to `runClearCacheFlag` in-process; behaviour is equivalent for
the on-disk caches but no resident-state invalidation happens because there
is no resident state in a one-shot CLI invocation.

### `--clear-matrix-cache` (now routed via daemon)

Routes through the `clear-matrix-cache` verb (`handleClearMatrixCache` in
`internal/cli/serve/clear_cache.go`). The daemon calls
`scan.ClearMatrixCache`, the same function the in-process early-exit
(`runClearMatrixCacheFlag` in `internal/cli/scan/early_exits.go:49`) uses.

The matrix baseline cache lives at a host-wide path
(`~/.cache/krit/matrix-baseline`), so multiple per-repo daemons may share
it. We do not add a host-level lock for the following reason: the matrix
cache is best-effort by design — `saveBaseline` swallows write errors,
and `tryLoadBaseline` treats any missing or mismatched entry as a miss,
which the matrix runner handles by recomputing. A clear that races a
concurrent write from another daemon at worst causes one experiment case
to be recomputed; it cannot corrupt durable state because the matrix
runner re-execs `krit` for every case and never holds the cache open
across a recompute.

Inside a single daemon the clear is serialised on `state.analyzeMu` so it
cannot race with that daemon's own `analyze-project` enumerations of the
`cacheutil` registry.

The daemon also imports `internal/cli/scan` transitively (via
`internal/cli/serve/meta_verbs.go`'s use of `scan.RunOutputTypesTo`), so
`scan/matrix_cache.go`'s `init()` registers the matrix cache with
`cacheutil` in the daemon process. As a side effect, `--clear-cache`
already wipes the matrix cache too. The standalone `--clear-matrix-cache`
verb keeps the host-wide directory deletable without also dropping the
daemon's resident WorkspaceState / analysis cache / parse cache.

## Cache bypass flags

`--no-cache` rides on `AnalyzeProjectArgs.NoCache`: the daemon skips every
on-disk cache for that request, including its parse cache, but keeps its
resident `WorkspaceState` slots (parsed trees, type facts, resolver, code
index), which are keyed on file content.

`--no-parse-cache` rides on `AnalyzeProjectArgs.NoParseCache` (#1919): the
request parses every Kotlin and Java file it does not replay from the findings
or analysis cache, neither reading nor writing the daemon's parse cache or its
resident parsed trees (which are filled from the parse cache, so a stale entry
could otherwise reach the request through them). Without `--no-cache` the
findings bundle still replays, in-process and delegated alike. Both caches stay
as they are for later requests. The daemon confirms the bypass in
`AnalyzeProjectStats.ParseCacheBypassed`; when a daemon that predates the field
leaves it unset, the CLI discards the result and scans in-process. Older
daemons normally never see the request, because the binary hash handshake
already rejects or replaces a daemon built from another binary. A
`--no-parse-cache -v` scan does not stream the daemon's verbose output: it is
printed once the result is accepted, so a fallback does not print it twice.

These per-scan cache flags keep the scan in-process (#1956), since the daemon
keeps its caches in its own state directory and memory: `--no-cross-file-cache`
on its own (the daemon also keeps a code index in memory; with `--no-cache`,
which implies it, the scan is delegated), `--cache-dir` and `--store-dir` (the
daemon does not use the caches they point at) and `--parse-cache-cap-mb` (the
daemon's parse cache is resident with the default cap). Forwarding them is
#1947. `--no-resource-cache` has nothing to skip in the daemon, which keeps no
on-disk resource index cache.

`--clear-cache` and `--clear-matrix-cache` are served by a running daemon's
clear verbs (see "Cache clears" above). With no daemon running they clear
in-process; before #1956 such a run started a daemon, scanned and cleared
nothing.

## Process-wide state

The daemon serves every request from one process, so a flag that changes
process-wide state for one scan keeps that scan in-process (#1956):

- `--experiment` and `--experiment-off` set the experiment set rules read
  while they run (`experiment.SetCurrent`). No findings cache is keyed on it,
  so a daemon would replay findings computed under another experiment set.
- `--enable-editorconfig` merges `.editorconfig` properties into the config and
  re-applies it to the rule registry; the daemon keeps one resident config.
- `--no-oracle-filter` feeds every file to the type oracle. It is the
  findings-equivalence baseline for the oracle filter, and would change what
  the daemon's resident oracle has loaded.

`--no-type-inference` is forwarded as `no_type_oracle`. In-process it only
turns the type oracle off: the resolver it gates is not pre-built, and the
index phase builds the source-level resolver anyway, so a delegated scan asks
the daemon for no type oracle and gets the same findings. `--disable-related`
is resolved by the CLI, which adds the related rules (from each rule's
`RelatedRules` metadata) to the `disable_rules` list it sends. `--strict` rides
on the `strict` field the daemon has honoured since #229.

## Adding a new flag

When introducing a new CLI flag, add it to one of the three lists in
`internal/cli/scan/daemon_flag_routing_test.go` and a row to the table at the
top of this doc; the test fails until both exist and the code matches:

- **forwarded:** the daemon can honour the flag per request. Send it in
  `buildDaemonAnalyzeArgs` (a new wire field also needs the daemon to apply it
  for that request only, and a way for an older daemon to be caught: the
  binary hash handshake, or a confirmation in the response as
  `parse_cache_bypassed` does).
- **in-process:** the flag falls into one of the categories above (writes files
  at the client's CWD, mutates source under the repo, reports on the calling
  binary's environment, short-circuits to a bespoke reporter, changes
  process-wide or resident daemon state). Keep the scan in-process in
  `daemonCompatibleFlags` and extend this doc with a one-paragraph rationale.
  If it could be routed later, note what wire change would unlock it.
- **not needed:** the CLI applies the flag to the daemon's result, or the flag
  only matters next to one that already runs in-process. Say which.
