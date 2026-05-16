# Daemon flag routing

Krit's CLI auto-routes `krit <paths>` to a reachable daemon when the requested
flag set falls inside `daemonCompatibleFlags(f)` in
`internal/cli/scan/daemon_delegate.go`. Most analysis flags are eligible; the
flags listed below are intentionally held back on the in-process path.

This doc records *why* each flag stays in-process so future contributors do not
silently route them through the daemon and break user-visible behavior. Each
section also notes what would have to change for daemon routing to become a
sensible option.

## Meta / non-analysis flags

These flags do not run an analysis at all. The daemon's only client verb is
`AnalyzeProject` (see `buildDaemonAnalyzeArgs` at
`internal/cli/scan/daemon_delegate.go:100`); routing meta commands through it
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
results for `java` and `cwebp`, and whether `tools/krit-types/.../krit-types.jar`
or `~/.krit/krit-types.jar` exist on disk. Routing through the daemon would
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
`--baseline-audit`, `--rule-audit`, or `--delta` is present). The CLI
deserialises the segment into a `*scanner.FindingColumns` and replays
`RunBaselineAuditColumns` (`internal/cli/scan/baseline_audit.go:34`) /
`RunRuleAuditColumns` (`internal/cli/scan/rule_audit.go:75`) locally,
producing the same audit output the in-process flow does. The non-audit
common path stays on the original `{findings,stats[,dispatch_profile]}`
envelope: the `columns` field is `omitempty` and the fast-scan response
decoder treats it as another optional segment after `dispatch_profile`
(see `scanOptionalColumns` in `internal/daemon/response_scan.go`).

## Oracle I/O & sampling

This bucket lists flags that look related to `--input-types` (which IS daemon-
routable) but stay in-process for distinct reasons.

### `--output-types` (now routed via daemon)

`--output-types` routes through the daemon's `dump-types` meta verb
(`handleDumpTypes` at `internal/cli/serve/meta_verbs.go`). The verb runs
`scan.RunOutputTypesTo` against the requested scan paths, which locates
`krit-types.jar`, invokes the JVM (or honours `--no-cache-oracle` to bypass
the cache), and writes the resulting oracle JSON to the path the CLI provides.
The CLI absolutizes `--output-types` before forwarding (same convention as
`--input-types` and `--cpuprofile` / `--memprofile`) so the daemon — which has
its own CWD — writes to the user's intended location. Captured stderr and exit
code ride back in `MetaResult` and are replayed against the CLI's streams so
daemon-routed and in-process invocations remain byte-equivalent.

This routes the dump through the same resident oracle cache the daemon
populates for analyze-project, so warm `--output-types` calls land on a hot
cache instead of paying full JVM warmup again.

### `--delta`

`--delta` is **now routed via daemon** for the current-tree scan portion.
The CLI still owns the `git worktree add` + re-exec pass that produces the
base-ref snapshot (daemon-side worktree management would add filesystem-
mutating side effects to a long-lived service — explicitly out of scope).
Once the daemon serves the current-tree scan with `IncludeColumns=true`
the CLI applies `filterColumnsNewSince` (`internal/cli/scan/delta.go:111`)
locally against the deserialised `FindingColumns` and re-emits the filtered
set via `pipeline.OutputPhase{}.Run` so format, base path, and warning
promotion match the in-process flow. The wire addition was shared with
`--rule-audit` / `--baseline-audit`; see "Audits that ship with the
analysis" above for the response-shape rationale.

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

## Adding a new flag

When introducing a new CLI flag, decide which bucket it belongs in:

- **Daemon-eligible:** flag affects only analysis inputs/outputs that already
  cross the wire (`buildDaemonAnalyzeArgs`). Default; nothing to do.
- **In-process forever:** flag falls into one of the categories above
  (writes files at the client's CWD, mutates source under the repo, reports
  on the calling binary's environment, or short-circuits to a bespoke
  reporter). Add it to the appropriate group in `daemonCompatibleFlags` and
  extend this doc with a one-paragraph rationale.
- **Could be daemon-eligible later:** add to the in-process bucket today
  and note here what wire change would unlock daemon routing, so a future
  contributor can revisit without re-deriving the analysis.
