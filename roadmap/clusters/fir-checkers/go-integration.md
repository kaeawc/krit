# B.2 — Go-side integration

**Cluster:** [fir-checkers](README.md) · **Status:** planned · **Track:** B · **Severity:** n/a (tool mode)

## Catches

The moment the `krit-fir` JVM subprocess exists, the Go CLI needs
to (a) spawn it, (b) reuse it across invocations, (c) cache its
findings on disk keyed by content hash, (d) decide which files
each checker needs to see, and (e) merge the resulting findings
into the same pipeline the tree-sitter path uses. Every single
one of these is already solved for `krit-types` / `internal/oracle/`
and needs to be copied, not reinvented.

## Shape

```
internal/firchecks/                    ← new Go package
  client.go                            ← spawn, daemon handshake, request/response
  cache.go                             ← content-hash cache of finding results
  filter.go                            ← per-rule file filter
  invoke.go                            ← one-shot fallback when daemon unavailable
  invoke_cached.go                     ← warm path through cache → daemon
  findings.go                          ← decode krit-fir JSON → []scanner.Finding
  fake.go                              ← interface-based fake for tests
  *_test.go                            ← behavioral + layered daemon tests
```

The package mirrors `internal/oracle/` one-for-one. Every file
here has a sibling in oracle. The shape is:

- **`client.go`** — the transport. Spawn `java -jar krit-fir.jar
  --daemon --port 0`, read the readiness handshake, keep the
  socket open for reuse. Copies `daemon.go` wholesale with a few
  renames.
- **`cache.go`** — on-disk content-hash cache of finding results
  under `{repo}/.krit/fir-cache/entries/{hash[:2]}/{hash[2:]}.json`.
  Each entry stores the finding list plus a closure fingerprint
  so dep-closure edits invalidate downstream entries. Poison
  markers for checker-crash files. Directly parallel to
  `internal/oracle/cache.go`.
- **`filter.go`** — per-rule `OracleFilter`-equivalent. Each FIR
  checker declares an `Identifiers` list; `CollectFirCheckFiles`
  partitions the file set so only files matching any declared
  identifier go to the JVM. Conservative default (`AllFiles: true`)
  for unaudited rules.
- **`invoke_cached.go`** — warm path. For each file, look up the
  content hash in the cache; on hit, return the cached findings;
  on miss, batch the missing files into a daemon request; on
  daemon unavailable, fall through to one-shot mode via
  `invoke.go`.
- **`findings.go`** — decode `{path, line, col, rule, severity,
  message, confidence, fix}` into `scanner.Finding` with the
  `ruleSet` field set to the FIR-rule's home category so the
  SARIF / JSON output layer treats it identically to tree-sitter
  findings.

## Daemon reuse

The `krit-fir` daemon lives at the same PID-file directory layout
as `krit-types`:

```
~/.krit/daemons/{sourceDirsHash}.krit-fir.{pid,port}
```

Per-repo hash routing means two repos can keep two warm
`krit-fir` daemons without colliding. The `Release` and
cross-invocation reuse logic from `internal/oracle/daemon.go`
(commits `fbd22ff`, `e22fd85` from the 2026-04-14/15 push)
applies directly. Copy the file, rename the struct, done.

Idle timeout: 30 minutes, same as the oracle daemon. On idle,
the daemon exits cleanly via `exitProcess` so krit-fir doesn't
leak zombie JVMs (see item 21's `f6ef017` for the discipline).

## Cache invalidation

The finding cache's dep-closure fingerprint is the same idea as
the oracle cache: each entry records the set of source files it
transitively depends on plus a content hash of each one. On lookup,
the cache recomputes the fingerprint for the current file set; a
mismatch invalidates the entry. This is what makes warm-1-edit fast
— an edit to a leaf file only invalidates that file's entry, not
its callers.

One subtlety: FIR checker findings can depend on types resolved
through compiled dependencies (classpath jars), which are not
source files. For those, the cache records the classpath hash
(same pattern as the oracle). A jar change invalidates the entire
cache, which is fine because jar changes are rare.

## CLI wiring

Add `--fir` / `--no-fir` flags to `cmd/krit/main.go`. Default off
during the Track B.4 pilot phase, default on once at least three
FIR rules ship in the default catalog. The flag gates two things:

1. Whether `internal/firchecks.InvokeCached` is called at all
2. Whether FIR findings are included in the output

Also wire `--fir-daemon` / `--no-fir-daemon` as escape hatches
(same as `--oracle-daemon`) so CI can force one-shot mode on
hermetic runners where persistent daemons don't make sense.

`--perf` should log `FirCheck.Stats()` hit/miss counters alongside
the existing `Oracle.Stats()` counters so we can see cache
effectiveness on real runs.

## Merge with tree-sitter findings

Merging is a sort + dedup pass at the end of the rule phase. If a
Go rule and a FIR rule both fire on the same `(file, line, col,
ruleCategory)`, the Go finding wins (tree-sitter ran first, FIR
is a refinement). In practice the rule namespaces are expected
to be disjoint — a rule is either a Go walker or a FIR checker,
not both — but during the pilot migration (see
[pilot-rules.md](pilot-rules.md)) both will coexist for parity
oracle testing.

## Definition of done

- `internal/firchecks/` compiles and has unit tests paralleling
  `internal/oracle/`'s test layout (client, daemon, cache, filter,
  invoke_cached)
- `krit check --fir` on Signal-Android returns findings from both
  passes and dedupes correctly
- Warm `krit check --fir` on Signal-Android is within 2× of
  `krit check` (tree-sitter-only) — the FIR pass should amortize
  nearly to zero on warm runs via the cache
- Cold `krit check --fir` on Signal-Android finishes inside 90 s
- Cache hit/miss counters visible under `--perf`

## Non-goals (for this concept)

- Checker implementation details — see
  [checker-api.md](checker-api.md)
- Rule selection for pilots — see
  [pilot-rules.md](pilot-rules.md)
- Kotlinc plugin packaging — see
  [plugin-packaging.md](plugin-packaging.md); Track B never
  runs inside a user's build
