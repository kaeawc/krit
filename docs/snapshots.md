# Structural snapshots

Krit can capture the structural state of a project at each commit and
persist it under `.krit/snapshots/`. Captured snapshots support
**timeline** queries (a metric over the history of a module or file),
**diff** between any two captured shas, and a **CI gate** that fails
when a delta crosses a threshold.

A captured snapshot has three sidecar files per commit:

- `graph.gob.zst` — modules, files, symbols (cold-path; loaded only
  for diff and similar deep queries).
- `metrics.gob.zst` — per-file LOC / bytes / symbols / cyclomatic and
  per-module rollups with FanIn / FanOut (hot path; timeline reads).
- `manifest.json` — schema versions, krit version, parent shas, and
  count summaries (greppable without krit).

## Capture

```sh
krit snapshot capture           # capture HEAD
krit snapshot capture v1.2.3    # capture any ref or sha
krit snapshot status            # list captured shas
krit snapshot info HEAD         # print one snapshot's manifest
```

`krit snapshot capture HEAD` is meant to run as a post-commit hook so
the timeline grows automatically as you commit:

```sh
krit snapshot install-hook              # writes .git/hooks/post-commit
krit snapshot install-hook --uninstall  # removes it
krit snapshot install-hook --print      # print the script (for core.hooksPath setups)
```

`install-hook` also appends `.krit/snapshots/` to the repo's root
`.gitignore` (idempotent, best-effort) so captured blobs and manifests
don't get committed by accident.

The hook runs `krit snapshot capture HEAD` detached, so a slow capture
never blocks `git commit`. Capture latency on a warm parse cache is a
few hundred milliseconds on small-to-medium repos; the first capture of
a project pays the full parse cost.

## Backfill

To populate the timeline retrospectively, walk past commits with
parallel git worktrees:

```sh
krit snapshot backfill --since 720h --workers 4   # last 30 days
krit snapshot backfill --max 100                   # last 100 commits
krit snapshot backfill --branch main               # walk main rather than HEAD
krit snapshot backfill --force                     # recapture even if a snapshot exists
```

Backfill is resumable: shas with an existing graph blob are skipped on
re-runs. Each worker spins up a detached `git worktree add`, runs
capture, and tears the worktree down — your active checkout is never
touched.

## Timeline

Project a single scalar metric across every captured snapshot, sorted
by capture time:

```sh
krit snapshot timeline --metric loc
krit snapshot timeline --scope module --target :feature:checkout --metric fan_in
krit snapshot timeline --scope file --target src/main/kotlin/Order.kt --metric cyclomatic
```

Available metrics depend on scope:

- **repo**: `loc`, `files`, `symbols`, `public_symbols`, `cyclomatic`,
  `modules`
- **module**: `loc`, `files`, `symbols`, `public_symbols`,
  `cyclomatic`, `fan_in`, `fan_out`
- **file**: `loc`, `bytes`, `symbols`, `public_symbols`, `cyclomatic`

Snapshots that don't carry the requested target produce a sparse series
(no zero-fill), mirroring git history rather than fabricating points.

## Diff

```sh
krit snapshot diff <from> <to>                 # text output
krit snapshot diff <from> <to> --format json   # machine-readable
```

Surfaces added/removed files, FQN+Signature-keyed symbol deltas, added
or removed modules and module dependency edges, and per-metric repo /
module deltas. Refuses cross-version diffs when blob schemas differ;
both args resolve through `git rev-parse` so refs and short shas work.

## Simulate a rule across history

Answer "would this rule have been useful if I'd shipped it six months
ago?" by walking history and scoring the rule against each commit:

```sh
krit snapshot simulate MagicNumber --since 720h --workers 4
krit snapshot simulate LongMethod --max 100 --format json
```

Each commit is checked out into a detached worktree, krit is invoked
with `-f json -enable-rules <rule>` against it, and the resulting
`summary.byRule[<rule>]` count becomes one point in the time series.
Slow per call (one full krit run per commit) — the design assumes
rule-tuning is a one-off rather than a hot path.

## CI gate

Fail (exit 2) when a delta crosses a threshold. Threshold flags are
repeatable:

```sh
krit snapshot gate origin/main HEAD \
    --max-pct loc=5 \
    --max-pct cyclomatic=10 \
    --max-delta files=20
```

- `--max-abs metric=v` — cap on the to-side absolute reading
- `--max-delta metric=v` — cap on the absolute increase
- `--max-pct metric=v` — cap on the percent increase from the from-side

Multiple flags on the same metric stack independently. Repo-scope only
in v1.

## MCP

The `snapshot` MCP tool exposes the read-only operations to AI agents:

| Operation  | Inputs                              |
|------------|-------------------------------------|
| `status`   | `repo_root`                         |
| `info`     | `repo_root`, `commit_sha`           |
| `timeline` | `repo_root`, `scope`, `target`, `metric` |
| `diff`     | `repo_root`, `from`, `to`           |

Capture, backfill, and gate stay CLI-only — they mutate state, run
long, or are CI-flavored exit-code drivers.
