# AnalysisDaemon

**Cluster:** [build-integration](README.md) · **Status:** planned · **Severity:** n/a (tool mode)

## Catches

Cold-start cost that erases the value of every other feature in
this cluster. [`abi-hash.md`](abi-hash.md),
[`used-symbol-extraction.md`](used-symbol-extraction.md),
[`cross-module-dead-code.md`](cross-module-dead-code.md), and
[`symbol-impact-api.md`](symbol-impact-api.md) all require the
scanner to have parsed the project, the oracle to have loaded
classpath types, and typeinfer to have warmed its resolution
caches. A build tool that re-invokes `krit` per compile action
pays that cost tens or hundreds of times per build — more than
the savings those features provide.

`krit serve` (working title) keeps a long-lived process with the
project's parse trees, cross-file index, oracle state, and
typeinfer cache resident in memory, watches the source tree for
changes, invalidates incrementally, and exposes every build-integration
feature as a request on a local socket.

This is a **delivery mechanism**, not a feature in itself. It does
not compute anything new; it just keeps the other features usable
in a real build.

## Shape

```
$ krit serve --root . --socket .krit/daemon.sock &
krit daemon: ready (1240 files, 86 modules, warm in 4.3s)

$ krit abi-hash :core                # same verbs as non-daemon mode
:core  3fa1c2d9e7b85af4               # returns in <10ms from daemon

$ krit used-symbols --json :feature-profile
{"module":":feature-profile","symbols":[...]}

$ krit impact --since HEAD~1
...
```

Any verb in this cluster should transparently prefer the daemon
when a socket is present, and fall back to in-process execution
otherwise. The CLI surface is identical either way.

Daemon lifecycle:

- **Start.** `krit serve` parses the project, builds the scanner
  index, warms oracle, and reports ready.
- **Watch.** Subscribes to filesystem events under the project
  root (use Go's `fsnotify` or equivalent). On change: re-parse
  only the touched files, invalidate their index entries, drop
  the affected cache rows.
- **Serve.** Accepts line-delimited JSON (or protobuf) requests
  on a Unix socket. Each request names a verb and its arguments;
  each response is a JSON blob or streamed events.
- **Stop.** `krit serve --stop` or SIGTERM. The daemon persists
  its parse cache to
  [`internal/cache/cache.go`](/Users/jason/kaeawc/krit/internal/cache/cache.go)
  before exit so the next cold start is warm.

## Dispatch

Wire request routing in a new `internal/daemon/` package (or
reuse/extend the LSP server scaffolding in
[`internal/lsp/server.go`](/Users/jason/kaeawc/krit/internal/lsp/server.go)
— it already has the socket + request-dispatch plumbing and could
serve both LSP and krit's own verb protocol from one process).

Each verb in this cluster becomes a request type:

- `abi-hash` → `{verb:"abi-hash", target:"<module-or-path>"}`
- `used-symbols` → similar
- `impact` → `{verb:"impact", symbols:[...]}` or `{verb:"impact", since:"..."}`
- `dead-code` → `{verb:"dead-code", scope:"project"}`

The file watcher invalidates three caches in order when a file
changes: (1) parse tree, (2) scanner index entries for that file,
(3) oracle/typeinfer resolution cache rows keyed by that file's
FQNs. The inverted reference index used by
[`symbol-impact-api.md`](symbol-impact-api.md) updates in-place
(remove old entries for the file, add new).

## Infra Reuse

- LSP server scaffolding:
  [`internal/lsp/server.go`](/Users/jason/kaeawc/krit/internal/lsp/server.go),
  [`internal/lsp/protocol.go`](/Users/jason/kaeawc/krit/internal/lsp/protocol.go)
  — already handles long-lived socket, request dispatch, and
  incremental document syncing. The krit daemon protocol should
  cohabit this server rather than standing up a second one.
- AppCDS JVM startup cache:
  [`internal/oracle/daemon.go`](/Users/jason/kaeawc/krit/internal/oracle/daemon.go)
  — despite the name, this is about warming the *JVM* that oracle
  shells out to, not about a krit query daemon. Independent
  concerns, but both matter for cold-start cost and both should
  be exercised by the benchmark suite for this feature.
- Persistent cache across daemon restarts:
  [`internal/cache/cache.go`](/Users/jason/kaeawc/krit/internal/cache/cache.go).
- Parse / index / resolve: same building blocks as every other
  concept in this cluster. The daemon adds no new analysis.

## Open questions

- **Invalidation soundness.** The hardest problem here. A change
  to a `.gradle.kts` file can change the module graph, which can
  change which files are in which module, which can change every
  reachability answer. An `import` change can add a new symbol
  that suddenly resolves an ambiguous reference. Start by
  invalidating coarsely (any build-script change → full rebuild
  of the index) and tighten over time with targeted benchmarks.
- **Security.** A daemon listening on a socket, parsing and
  typechecking arbitrary Kotlin in the repo, is a local-privilege
  target. Use a user-owned Unix socket with `0600`, never a TCP
  port by default, and document this.
- **Process ownership.** Who starts `krit serve`? Options: the
  user manually, a `direnv`-style hook, the build tool (grit)
  spawning it on demand, or a system-level launchd agent. Leave
  the mechanism to the caller; ship the daemon itself without
  choosing.
- **Multi-root.** Monorepos may want one daemon per repo, or one
  daemon serving several. The protocol should allow a root
  prefix in every request so a single daemon can host more than
  one project at once. Defer until asked for.

## Links

- Parent: [`../README.md`](../README.md)
- Related: all other concepts in [`../build-integration/`](README.md)
- Adjacent: [`../sdlc/lsp-integration/`](../sdlc/lsp-integration/) — the LSP cluster and this one share a server process.
