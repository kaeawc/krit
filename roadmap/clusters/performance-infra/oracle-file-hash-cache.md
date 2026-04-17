# OracleFileHashCache

**Cluster:** [performance-infra](README.md) · **Status:** shipped ·
**Severity:** n/a (infra) · **Default:** on (opt out via `-no-cache-oracle`)

## What it does

Incrementalises the per-file output of `krit-types` with an on-disk
content-addressable cache keyed by `SHA-256(file_bytes)` plus a fingerprint
over the file's direct source dependency closure. On a warm dev-loop
re-run where nothing changed, the oracle is served **entirely from disk
without launching the JVM**. When a single file is edited, only that file
and its reverse-dependents are re-analyzed.

## Relationship to item 21 (Daemon Startup Optimization)

Complementary. Item 21 targets **JVM + session startup cost** (AppCDS,
persistent daemon, CRaC). This cluster targets the **per-file analysis
cost itself**: even with a zero-cost daemon, a warm dev loop still pays
to re-walk every source file through the Analysis API. The cache lets us
skip that walk for files whose result cannot have changed.

The two stack cleanly:
- With cache only: warm no-edit = 4.9s on Signal-Android (JVM still cold
  for the persistent-error files' retry — see Known limitations)
- With daemon only: warm no-edit ≈ session-ready + full reanalyze
- With cache + daemon: target is < 1s warm no-edit once the cache owns
  miss-list handoff to the daemon instead of launching a fresh JVM

## Cache layout

```
{repo}/.krit/types-cache/
├── version                               # integer; bump nukes entries/
└── entries/
    └── {hash[:2]}/{hash[2:]}.json        # content-addressable entry
```

Two-level sharding (`{hash[:2]}`) caps any single directory at 256
shards even in mega-repos.

## Entry schema

```json
{
  "v": 1,
  "content_hash": "sha256hex",
  "file_path": "/abs/path/to/File.kt",
  "file_result": { /* OracleFile as produced by krit-types */ },
  "per_file_deps": {
    "java.lang.AutoCloseable": { /* ClassResult */ },
    "kotlinx.coroutines.Job":   { /* ClassResult */ }
  },
  "closure": {
    "dep_paths": ["/abs/path/Foo.kt", "/abs/path/Bar.kt"],
    "fingerprint": "sha256hex"
  },
  "approximation": "symbol-resolved-sources"
}
```

`closure.fingerprint = sha256(sorted(sha256(read(dep_paths[i])) for all i))`.

## Correctness model

Findings-equivalent, not byte-identical. A lookup is a HIT iff:

1. `ContentHash(path) == entry.content_hash` — the file itself is unchanged
2. Every `entry.closure.dep_paths[i]` still exists on disk
3. `recompute(closure.fingerprint)` matches the stored fingerprint

Any failure falls through to a miss, and the corresponding entry is
either rewritten (normal miss) or deleted (corrupt / version mismatch).

Assembled oracle JSON (merged from hits + fresh misses) has:
- **same `Files` keys** as a full `-no-cache-oracle` run
- **same `Dependencies` keys** (unioned from per-file fragments + fresh)
- **same declaration FQNs** for each file

Validated against Signal-Android: 2428 files / 103 deps / 0 mismatches
on cold, warm-no-edits, and warm-one-edit runs.

## Dep-closure tracking (Kotlin side)

The Analysis API does not expose "which `KtFile`s did FIR lazy-load
during this `analyze {}` block". We approximate the per-file dep set via
three signals, all walked within a single `analyze` block using only
public API:

1. **Import resolution.** For each `KtImportDirective.importedFqName`, we
   call `KaSession.findClass(ClassId.topLevel(fqn))` and record the
   resolved symbol's source file (`symbol.psi?.containingFile?.virtualFile?.path`)
   if the origin is `SOURCE`.
2. **Supertype walking.** For each class declared in the file, we walk
   `symbol.superTypes` and record any source-origin ancestors the same
   way — catches cross-file inheritance not mediated by an explicit import.
3. **Property / function type refs.** For each declared class's member
   scope, we record source-origin return types and parameter types.

This is tagged `approximation: "symbol-resolved-sources"` in each entry.
It is **unsound** relative to a true FIR lazy-load trace: transitive
changes in a file that is not directly referenced but whose result was
consulted during type inference (e.g. inferred generic upper bounds
through a chain of supertypes) are not captured. In practice, the
transitivity is recovered "from the bottom up" because any such
intermediate file whose _own_ analysis result changes will have a
different content hash → itself misses → gets re-analyzed → its own
cached closure is then consulted by downstream files on the next run.

Entries carrying a different `approximation` string are treated as a
miss, so upgrading the tracker (e.g. a future `PsiFileManager`
delegate-based approach) retires old entries automatically.

## Measured wall times (Signal-Android, 2428 .kt/.kts files)

| Scenario          | Wall    | JVM launched? | Miss count |
| ----------------- | ------- | ------------- | ---------- |
| Cold              | 32.5 s  | yes           | 2428       |
| Warm, no edits    | 4.9 s   | yes*          | 3          |
| Warm, one edit    | 23.1 s  | yes           | 496        |
| `-no-cache-oracle`| 35.4 s  | yes           | 2428       |

Cache footprint: 2429 entries, ~34 MB on disk.

\* The JVM is still launched for the 3 files that fail analysis with
`KotlinIllegalArgumentExceptionWithAttachments` on every run — those
never get a cache entry (no FileResult → `WriteFreshEntries` skips
them) and are classified as misses forever. Removing this retry
overhead is future work (mark them as "known failing" with a poison
entry, so the miss-list stays empty on warm runs).

The warm-one-edit number is for editing `core-util-jvm/.../Log.kt`, a
file referenced by 493 other files — the worst realistic case. Editing
a leaf file (0 reverse-deps) re-analyzes ~4 files and runs in roughly
the same 5-7 s range as the warm-no-edits case.

## Go <-> Kotlin protocol

```
krit-types --sources ... \
           --files /tmp/miss-list.txt \      # NEW: one abs path per line
           --cache-deps-out /tmp/deps.json \ # NEW: per-file dep fragments
           --output /tmp/fresh.json
```

`--files` restricts the analyze loop to the intersection of the full
source module and the listed paths. The full source module is still
built because Analysis API needs the whole session for cross-file
resolution — we are only narrowing what gets _walked_, not what gets
_seen_.

`--cache-deps-out` writes a JSON file of
`{ "files": { path: { "depPaths": [...], "perFileDeps": {...} } } }`
shape alongside the main output. The Go side parses this, computes the
closure fingerprint per-file from current disk state, and writes one
atomic cache entry per fresh FileResult.

## Ownership split (Go vs Kotlin)

- **Go** owns: cache dir layout, content hashing, closure fingerprint
  computation, hit/miss classification, atomic entry writes, corrupt
  entry cleanup, version invalidation, final oracle JSON assembly.
- **Kotlin** owns: actual analysis of miss-list files, extraction of
  per-file dep fragments (direct source imports + supertypes + member
  type refs), emission of the `--cache-deps-out` JSON.

The Go side never inspects the closure semantics beyond "did this set
of files change on disk". The Kotlin side never touches the cache
directory.

## Known limitations

- **Unsound approximation** (`symbol-resolved-sources`): see above.
  Fallback-ish relative to a true PSI-manager-delegate trace. Safe in
  practice for the dev loop because any transitive file whose content
  changes gets re-analyzed directly.
- **Persistent-error retry**: 3 files in Signal-Android crash the
  Analysis API on every run and are reclassified as misses each time.
  A "poison entry" marker would let the cache serve those as permanent
  misses without relaunching the JVM.
- **No kts cross-module resolution**: `.gradle.kts` files get walked
  but their dep tracking resolves only top-level ClassIds, which is a
  subset of what a real Gradle script compiler would see. The oracle
  output for .kts files is findings-equivalent but their closure is
  trivially approximated ("no deps" is common).
- **CI reuse is out of scope**: this cache targets the dev loop only.
  On CI the cache dir would need to be warm-loaded from a previous job
  — not addressed here.
- **Daemon integration**: the cache currently short-circuits before
  the daemon path, not after. A future change will route miss-list
  re-analysis through the persistent daemon when it's available,
  trimming the 5 s JVM startup from warm runs.
