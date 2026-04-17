# Oracle cache optimization — notes from the 2026-04-14/15 session

Live document. Captures the design decisions, the things that were wrong
the first time, and the measurements that drove each subsequent fix.

## Goal

Make the krit-types oracle fast on warm re-runs. Primary target: the
developer dev loop ("I edited one file, run krit again"). Secondary: let
small-to-medium repos feel instantaneous when nothing has changed.

## Baseline we started from

| Repo | Files | Cold wall | Warm wall (before cache) |
|---|---|---|---|
| Signal-Android | 2431 | ~45 s | ~45 s (no cache) |
| kotlin/kotlin (exclude-glob on) | 18357 | ~5.5 min | ~5.5 min |

## First shippable cache — commit `279de70`

Content-addressable store keyed by `sha256(file content)`, entries under
`{repo}/.krit/types-cache/entries/{hash[:2]}/{hash[2:]}.json`. Each entry
carries its dep-closure fingerprint (sha256 of sorted dep-content-hashes).

Cold path: Go-side `InvokeCached` walks source dirs, classifies files into
hits vs misses, runs krit-types only on misses with `--files LISTFILE` +
`--cache-deps-out PATH`, writes fresh cache entries from the response,
assembles the final oracle JSON.

**Signal-Android**: cold 32.5 s → warm-no-edit **4.9 s**. 6.6× dev-loop win.

## The "5 s floor" mystery — the explanation I expected vs the real cause

I thought the 4.9 s warm floor on Signal was from 3 files that
deterministically crash krit-types (FIR resolver bugs in Analysis API).
Shipped a poison-entry marker (commit `c40f8c9`) so crashed files get
cached as "don't re-analyze" entries and get projected as hits on warm.

Then I re-measured and found: **there are no crash files on Signal-Android**
with the current jar. The actual floor cause was different.

The floor came from 6 *empty* files — typealiases, top-level extension
functions, extension property getters. krit-types analyzed them, produced
no `FileResult` (because of a `declarations.isNotEmpty() || expressions.isNotEmpty()`
guard), wrote nothing to the output map, the Go cache writer saw no data
for those paths, wrote no cache entry. Every warm run re-classified them
as misses → relaunched the JVM → analyzed them → same nothing → same
non-cache. 4.9 s of JVM+session startup paid for 6 files that would
never contribute data.

The 6 files on Signal were:

1. `ArchiveTypeAliases.kt` — 3 typealiases
2. `ActiveContactCount.kt` — 1 typealias
3. `AnyMappingModel.kt` — 1 typealias
4. `ExoPlayer.kt` — 2 top-level extension fns, body has only property assignments
5. `DocumentFileHack.kt` — 1 extension fn, body is `this is TreeDocumentFile`
6. `SizeUnit.kt` — 2 top-level extension properties

All share the same shape: no `KtClassOrObject` top-level declarations AND
no `KtCallExpression` bodies. The current `analyzeKtFile` only extracts
those two kinds of things, so it found nothing to emit for them. Valid
Kotlin files with real semantic content, but outside the oracle's current
scope.

**The fix (commit `c17d1fc`):** drop the guard, always emit a `FileResult`
for analyzed files even if `declarations` and `expressions` are both empty.
An empty entry is valid oracle data — downstream consumers already handle
it (the `oracle.Load` path iterates `file.Declarations` which is fine if
empty, and only populates the expressions index if the map is non-empty).

**Signal-Android after the fix:** warm-no-edit **0.56 s**. 80× vs pre-cache
baseline. 8.8× improvement on top of the flawed floor.

## Lesson 1: never trust a "this is probably why" story until you've
## actually observed the failure mode

The poison-entry mechanism I built is correct — it just wasn't solving the
problem I thought it was solving. If Analysis API gains new per-file crash
sites on a future Kotlin version, the mechanism will activate and do its
job. But today on Signal-Android, zero crash entries, and the floor came
from somewhere completely different.

## Lesson 2: "empty cache entries" are valuable data, not dead weight

A cached `FileResult{declarations: [], expressions: {}}` is 1 KB on disk
and turns a 4 s JVM launch into a 1 μs map lookup. The cost is noise, the
benefit is dominant. Always emit entries for analyzed files, even when
they look "empty" to the analyzer's current scope.

## Kotlin warm run — a second ceiling at ~35 s

Signal warm at 0.56 s. Ran the same thing on kotlin/kotlin (16k files):
**35 s warm**. Two distinct problems once I dug in.

### Problem 1: excluded files leak into the miss list

krit-types's `--exclude` glob (default `**/testData/**`, `**/test-resources/**`)
runs *inside* the jar. The Go side `CollectKtFiles` walks the whole tree
and finds 59,494 files. None of them are excluded. Every warm run:

- Go classifies 59,494 files
- 16,244 hit the cache (files the jar analyzed on cold)
- 43,250 are "misses" from Go's perspective
- Go writes the miss list to a tempfile, launches krit-types with `--files`
- krit-types runs its `--exclude` filter, drops 42,846 testData files
- Jar analyzes the remaining 404 files (which had file-path-based misses
  we'll get to in problem 2)
- Writes 404 new entries
- But the 42,846 excluded files are *still* misses in Go's view on the
  next run, and will go through the same pointless round-trip forever

**Fix**: Go-side `CollectKtFiles` mirrors the Kotlin default exclude. Directory
pruning for `testData` and `test-resources` at walk time plus a substring
backstop check. See `defaultExcludeSubstrings` in `invoke_cached.go`.

### Problem 2: content-hash dedup victims misclassified as misses

kotlin/kotlin has many files with identical content — empty package-info
shims, repeated test boilerplate, trivial forwarders. The content-hash
cache naturally dedupes: N files with identical bytes → one entry. But
the old `ClassifyFiles` had:

```go
if entry.FilePath != p {
    // Hash collision across repos or a stale entry for a moved file.
    // Treat as a miss and overwrite on write.
    misses = append(misses, p)
    continue
}
```

For dedup victims (file B with same content as file A, only A's entry on
disk), this produces a miss. krit-types re-analyzes B on every warm run,
writes an entry with FilePath=B, now A becomes the dedup victim, and the
two thrash forever.

**Fix**: content-hash match is a hit regardless of stored FilePath. When
the stored entry's FilePath differs from the caller's path, project the
entry onto the caller's path via a shallow copy with `synthetic.FilePath = p`.
Caller's AssembleOracle uses that rewritten path as the map key.

Commit holding both fixes is the same CL as the next round of classify
optimizations — see below.

## The third ceiling — classify itself was too slow

After fixing the two leaks above, kotlin warm dropped from 35 s to ... still
~36 s. Progress in misses (43250 → 1) but classify time went **up**, from
3.8 s to 5.4 s. Tracing:

- Classify input: 16,648 files
- Classify hits: 16,648
- Classify misses: 1 (the jar then launches for this one file, session build
  walks 59k sources, ~30 s spent analyzing zero files)

Classify was doing one `LoadEntry` (disk read + JSON parse) per file AND
one `VerifyClosure` per hit. `VerifyClosure` calls `closureFingerprint`
which re-reads every file in the closure from disk and re-hashes it.
Kotlin files average ~15 deps each → **16,648 × 15 = ~240,000 redundant
dep file reads per warm run**, most of which are the same files being
hashed again and again.

**Fix 1**: shared `hashCache` map across the whole classify pass. First
time a dep path is seen, hash and cache. Subsequent times, reuse. Each
unique file hashed at most once per pass.

**Fix 2**: in-memory cache hash index. Before classify, walk the
entries directory once and build a `map[contentHash]bool` of what's on
disk. Classify then fast-paths definite misses through the set
membership check — no `LoadEntry` stat+parse for files that were never
cached. `IndexCacheHashes` in `cache.go`.

Expected warm classify on kotlin after both: ~200-400 ms (dominated by
hashing the 16,648 source files themselves). Down from 5.4 s. Validation
pending — the cold run is repopulating as I write this doc.

## The 1709-file discrepancy — turned out to be `build/` pruning

Observation from the kotlin cold run after the Go-side exclude fix:

    Excluding 2 patterns; skipped 42845 files.
    --files: restricting to 16648 of 18357 files (16649 requested)

The jar enumerated 18,357 non-excluded files under `/Users/jason/github/kotlin/`.
Go sent 16,649 in the `--files` list. The intersection was 16,648. Go was
under-supplying by 1,709 files.

First hypothesis: Go's `FindSourceDirs` narrows to `kotlin`/`java` named
directories and misses files outside that convention. Checked with a
build-tag-gated probe test, and it was wrong: Go's walk from the kotlin
root found the same files as Go's walk via `FindSourceDirs`, both at 16,649.

Real cause: Go's walker pruned directories named `build` (alongside
`.gradle`, `.git`, `node_modules`) as a defensive measure for typical
Android/Gradle projects. kotlin/kotlin has **1,709 real, checked-in
`.kt` files under `build/` directories** — generated kotlin-reflect stubs
at `core/builtins/build/src-jvm/reflect/KType.kt` and friends. These
are the actual stdlib reflection interfaces that downstream code imports.

Fix: removed `build` from the prune list. Kept `.gradle`, `.git`,
`node_modules` — those are unambiguously non-source. If a project has a
genuinely noisy build directory, the `krit-types --exclude` glob is the
right knob, not the walker's hard prune.

Correctness impact without the fix: on kotlin, warm runs would assemble
an oracle with **1,709 fewer files than the cold run** — a silent
regression where downstream rules querying for `kotlin.reflect.KType`
would get nothing on warm while cold would see it. Caught by comparing
`CollectKtFiles` file counts to the jar's analyze count.

Probe test lives in `internal/oracle/probe_discrepancy_test.go` under
the `probe` build tag. Delete after the session.

## Lesson 3: any cache is a shadow of another data structure

Every shortcut in ClassifyFiles corresponds to something that was
computed elsewhere and could have been cached earlier. The `closureFingerprint`
fix isn't a new idea — it's *memoization of a pure function*. The
`IndexCacheHashes` fix isn't a new idea — it's *materializing the directory
enumeration once instead of implicitly per-file via `Stat`*. If the
operation is being done repeatedly over the same inputs, the question is
never "can we make it faster" but "can we not do it".

## Summary of the full cache commit chain

| Commit | What |
|---|---|
| `279de70` | Initial on-disk cache — files + per-file dep fragments |
| `c40f8c9` | Poison-entry markers for crashed files (insurance, not exercised yet) |
| `c17d1fc` | Always emit FileResult so empty files cache — Signal warm 4.9 → 0.56 s |
| (in flight) | Dedup-victim fix + Go-side exclude + classify in-memory index + shared hash cache |

Final targets (pending kotlin validation):

- Signal warm 0 edits: 0.56 s (banked)
- Signal warm 1 edit: ~23 s (banked)
- kotlin warm 0 edits: target ~1-2 s
- kotlin warm 1 edit: target ~25-40 s (reverse-dep fan-out sized)
