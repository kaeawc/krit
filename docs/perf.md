# Performance Tuning

Krit aims to be fast out of the box. This page lists the knobs available for
people who need to push further — large monorepos, CI tight on wall-clock, or
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
library facts. On a 50k-file Kotlin/Android repo this typically takes
analyze-project from ~3s cold to ~50–150ms warm.

The daemon also keeps an LSP and MCP server in the same process, so editors
and AI agents share the warm state.

See `--no-daemon` to force the cold path explicitly.

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

- Repos under ~10k files. The inotify cost is negligible.
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
spins it back up with `krit --daemon`. Useful on laptops to avoid keeping a
warm JVM-attached process alive overnight.

## Limit the work per scan

### `--diff REF` / `--delta REF`

```bash
krit --diff origin/main .          # findings on lines changed since main
krit --delta origin/main .         # findings *newly introduced* since main
```

The best perf knob for PR/CI runs. Krit still analyses changed files but
skips reporting on untouched ones. Combine with `--daemon` for compounding
wins.

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

Skips the JVM-backed Kotlin Analysis API oracle entirely. Strictly faster
than `--no-cache-oracle`. Loses precision on rules tagged `NeedsOracle` —
they're skipped instead of running with degraded data.

### `--no-fir`

Disables the FIR checker pass (separate krit-fir JVM subprocess). FIR is
opt-in to begin with; use this to override a config that turned it on.

### `--no-fir-daemon`

Forces one-shot mode for the FIR checker — no persistent daemon. Useful for
hermetic CI runners that won't tolerate stray processes.

## On-disk caches

All caches live under `.krit/` and are safe to delete. Disabling them mostly
helps debug stale-cache symptoms; the perf cost of an enabled cache is
near-zero on a warm run.

| Flag | What it disables |
|---|---|
| `--no-cache` | Incremental analysis cache (findings reuse) |
| `--no-parse-cache` | Tree-sitter parse-tree cache |
| `--no-resource-cache` | Android values-XML ResourceIndex cache |
| `--no-cross-file-cache` | Cross-file index cache |
| `--no-cache-oracle` | Incremental oracle cache (forces full JVM run) |
| `--no-matrix-cache` | Experiment-matrix baseline cache |
| `--clear-cache` | Delete all caches and exit |
| `--clear-matrix-cache` | Delete the experiment-matrix baseline cache and exit |
| `--cache-dir PATH` | Override incremental cache directory |
| `--store-dir PATH` | Use the unified store-backed cache |

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

**PR CI:** `krit --diff origin/main --daemon .`

**Full repo daily lint:** `krit --daemon .` (or no daemon if the runner is
ephemeral)

**Laptop dev daemon (battery-aware):** `krit serve --idle-timeout 10m
--max-parse-bytes 2147483648`

**Power-user setup on Linux:** `setcap` as above, then `krit serve` (auto
picks fanotify).

## When to file a perf issue

If a warm-daemon `analyze-project` takes more than ~300ms on a repo under
50k files, that's likely a bug. Attach:

- `krit --perf-rules` output
- `krit --version`
- Daemon's first line on startup (`krit daemon: ready ...`)
- A reproducer if possible (or repo size + ratios)

File against [kaeawc/krit](https://github.com/kaeawc/krit/issues).
