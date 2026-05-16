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

`--baseline-audit` and `--rule-audit` are listed in the in-process bucket
because they are short-circuits that *consume* the live findings produced by
the same scan invocation. `RunBaselineAuditColumns`
(`internal/cli/scan/baseline_audit.go:34`) and the rule-audit short-circuit
(`internal/cli/scan/output_shortcircuits.go:30`, dispatching to
`RunRuleAuditColumns` at `internal/cli/scan/rule_audit.go:75`) both take a
`*scanner.FindingColumns` produced by the in-process analysis path and emit a
report instead of normal findings.

The daemon today returns serialized `Findings` bytes via
`AnalyzeProject` (`internal/cli/scan/daemon_delegate.go:48`), not the
column-oriented `FindingColumns` structure these audits walk. Routing audit
flags through the daemon would require either teaching the audits to operate
on serialized output (losing efficient access to per-finding columns) or
extending the daemon wire to return `FindingColumns` directly.

These flags are listed here for completeness — Bucket A's owner may decide to
move them onto the daemon path once `FindingColumns` is reachable over the
wire. If/when that happens, drop them from the meta bucket in
`daemonCompatibleFlags`.

## Oracle I/O & sampling

This bucket lists flags that look related to `--input-types` (which IS daemon-
routable) but stay in-process for distinct reasons.

### `--output-types`

`runOutputTypesFlag` at `internal/cli/scan/early_exits.go:404` is a standalone
oracle dump: it locates `krit-types.jar`, runs the JVM (or honors
`--no-cache-oracle` to bypass the cache), writes the resulting JSON to the
caller-supplied path, and exits *before* any rules are loaded or fired. The
daemon's `AnalyzeProject` verb is a rule-running path; routing `--output-types`
through it would either (a) need a brand-new dump-only verb that duplicates
what the daemon's resident `OracleDaemon` already produces internally, or
(b) waste the rule-dispatch work the user explicitly asked to skip.

Daemon routing would only make sense if we added a wire verb like
`DumpOracle` that returns the oracle daemon's current snapshot. That's a
new feature, not a routing tweak.

### `--delta`

`filterColumnsByDelta` at `internal/cli/scan/delta.go:18` shells out to `git
worktree add` for the base ref, re-execs `krit` itself inside the worktree to
produce a baseline JSON report, and then filters the current-scan findings to
only those NOT present in the baseline. The orchestration is entirely a CLI-
side wrapper: the *base* sub-scan can already hit the daemon (it's just
another `krit` invocation), so the only thing daemon-routing the wrapper
would save is the parent-process `git worktree add` — which is the dominant
cost.

Daemon routing would mean either teaching the daemon how to spawn worktrees
(adds filesystem-mutating side effects to a long-lived service) or returning
`FindingColumns` from the wire so the CLI can run the diff client-side
without re-execing. The latter is the same wire change the audit short-
circuits would benefit from; revisit together.

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
