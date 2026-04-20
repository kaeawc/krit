# CacheInfraConsolidation — Execution Plan

Tracks GitHub issue [kaeawc/krit#292](https://github.com/kaeawc/krit/issues/292). Extracts three patterns — atomic write, content hashing, version-gated cache dir + sharded entry path + cache-clear registry — into `internal/fsutil/`, `internal/hashutil/`, and `internal/cacheutil/`, and migrates all call sites.

**Non-goal**: performance or behavior changes. Cache formats stay byte-stable; `--perf` within noise.

## Context a smaller model needs up front

- Go-only repo. Build + vet: `go build -o krit ./cmd/krit/ && go vet ./...`. Tests: `go test ./... -count=1`.
- Cache formats **must not drift**. Each subsystem migration lands as a single commit that both introduces the helper call and deletes the old inlined code. Never leave both paths live.
- The issue explicitly warns: "callers may pattern-match on error text today" and "two cache shapes in main: flat-file (oracle) vs. sharded (#288, #289). Helper API must express both explicitly." Preserve the exact `fmt.Errorf` wrappers each call site currently emits — wrap the helper's return, do not replace it.
- The issue says atomicity should be **"tempfile + fsync + rename"** and asks to "pick the strictest (likely oracle's)". Audit shows **no current site calls `tmp.Sync()`** — so the helper adds fsync everywhere. This is a behavioral upgrade (stronger durability), not a regression; gate it behind the new helper so all subsystems get it at once.
- `--clear-cache` today does not clear the cross-file-cache (PR #289's addition) or the oracle cache. The `CacheSet` registry added here will close those gaps; call it out in the commit message so the bug-fix is intentional.

## Inventory of inlined patterns (audit, do not edit)

| Site | Atomic write | sha256+hex | Version-gated dir | Sharded path |
| --- | --- | --- | --- | --- |
| [internal/oracle/cache.go](internal/oracle/cache.go) | `WriteEntry` L271-301 | `ContentHash` L110-121, `closureFingerprint` L164-198, `oracleVersionHash` L553-558 | `CacheDir` L75-92 | `entryPath` L126-131 |
| [internal/oracle/invoke_cached.go](internal/oracle/invoke_cached.go) | `writeOracleJSON` L510-546 | — | — | — |
| [internal/scanner/parse_cache.go](internal/scanner/parse_cache.go) | `saveEntry` L206-246 | `hashContent` L121-124 | `NewParseCache` L81-111 (two version files: `version` + `grammar-version`) | `entryPath` L129-134 |
| [internal/scanner/index_cache.go](internal/scanner/index_cache.go) | `writeFileAtomic` L372-394, `encodeGob` L339-358 | `contentHashBytes` L50-53, `computeCrossFileFingerprint` L63-92 | `CrossFileCacheDir` L43-48 (version embedded in meta.json instead of a `version` file) | — (single-dir layout) |
| [internal/store/file.go](internal/store/file.go) | `Put` L71-96 | `entryPath` uses pre-computed hex L54-59 | — (no version gating today) | `entryPath` L54-59 (1-level shard, includes RuleSetHash suffix) |
| [internal/scanner/baseline.go](internal/scanner/baseline.go) | `SaveBaselineJSON` L121-125 (tmp+rename, no CreateTemp) | — | — | — |
| [internal/cache/cache.go](internal/cache/cache.go) | `Save` L191-199 (custom tmp name with timestamp+rand) | `ComputeRuleHash` L243-249, `ComputeConfigHash` L260-279, cache-file-hash L117-129 | — | — |
| [cmd/krit/matrix_cache.go](cmd/krit/matrix_cache.go) | `saveBaseline` L284-294 | `computeMatrixBaselineCacheKey` L55-116, `hashFileContents` L112-116, `hashTargetTree` L152-182 | — | — |
| [internal/fixer/binary.go](internal/fixer/binary.go) | `WriteFile` L170, `Rename` L188 | — | — | — |
| [internal/onboarding/writer.go](internal/onboarding/writer.go) | (check during migration) | — | — | — |
| [internal/android/icons.go](internal/android/icons.go) | — | `sha256.Sum256` L236 | — | — |

The `internal/baseline/` and `internal/matrix/` paths named in the issue map to `internal/scanner/baseline.go` and `cmd/krit/matrix_cache.go` respectively — there are no packages by those names in `HEAD`.

## Phase 0 — foundation packages (parallel-safe)

Three independent packages. Spawn as **three parallel subagents** with the specs below. Each must land with complete unit tests in the same package before Phase 1 starts.

### Task A — `internal/fsutil/atomic.go`

**Agent type**: general-purpose.

**Goal**: one helper that replaces every inlined tempfile+rename block.

**API**:

```go
package fsutil

// WriteFileAtomic writes data to path with perm, using tempfile + fsync + rename
// so a concurrent reader never sees a truncated file and a crash mid-write never
// leaves a torn file on disk.
//
// Parent directories must already exist — callers that want MkdirAll should call
// it themselves, to keep this helper one syscall wide.
//
// The tempfile is created in the same directory as path (same filesystem =
// rename is atomic on POSIX) with prefix derived from filepath.Base(path).
// On any error after tempfile creation, the tempfile is removed best-effort.
func WriteFileAtomic(path string, data []byte, perm os.FileMode) error

// WriteFileAtomicStream is like WriteFileAtomic but streams into a caller-provided
// callback. Used for gob/json encoders that want to write directly into the
// tempfile without materialising the full payload.
//
//   err := WriteFileAtomicStream(path, perm, func(w io.Writer) error {
//       return gob.NewEncoder(w).Encode(v)
//   })
//
// The writer is buffered; flush is handled by the helper. fsync runs after
// flush and before rename.
func WriteFileAtomicStream(path string, perm os.FileMode, write func(io.Writer) error) error
```

**Implementation notes**:
- Use `os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")`.
- Call `f.Sync()` before `f.Close()` — this is the upgrade over today's inlined code.
- On Windows, `Sync` is still safe; do not branch on GOOS.
- On error after CreateTemp: `tmp.Close(); os.Remove(tmpName)`. Wrap the returned error with the operation that failed (`"write tempfile"`, `"sync tempfile"`, `"close tempfile"`, `"rename tempfile"`). Callers rewrap with their own domain prefix.
- Tempfile prefix must start with `"."` so it doesn't get picked up by `IndexCacheHashes`-style walkers that expect only `.json`/`.gob` extensions. See `internal/oracle/cache.go:233` and `internal/scanner/index_cache.go` for the scan pattern.

**Tests** (`internal/fsutil/atomic_test.go`):
- Round-trip small payload.
- Overwrite existing file — contents swap atomically.
- Parent dir missing → returns error (do not auto-create).
- Tempfile cleanup on simulated `Sync` failure (use a fake `io.Writer` that panics partway). Verify no `.tmp-*` files remain in dir.
- Stream variant: assert data written equals what the callback produced.
- Two goroutines racing on the same path both succeed; exactly one final content wins.

**Validation**: `go test ./internal/fsutil/ -count=1 -race`.

### Task B — `internal/hashutil/hash.go`

**Agent type**: general-purpose.

**Goal**: one place that computes SHA-256 + hex encoding.

**API**:

```go
package hashutil

// HashBytes returns the raw SHA-256 digest of b. Use this when the consumer
// wants [32]byte (e.g. store.Key.FileHash).
func HashBytes(b []byte) [32]byte

// HashHex returns the SHA-256 digest of b as a lowercase hex string. Use this
// for on-disk cache keys, fingerprints, and any user-visible identifier.
func HashHex(b []byte) string

// HashReader streams from r into a SHA-256, returning the lowercase hex digest.
// Used by the oracle content-hash path to avoid reading whole files into memory.
func HashReader(r io.Reader) (string, error)

// HashFile is a convenience wrapper that opens path, streams it through
// HashReader, and returns the hex digest. Returns the underlying os error
// on open failure (do not wrap — oracle.ContentHash's current callers
// check os.IsNotExist on the return).
func HashFile(path string) (string, error)
```

**Implementation notes**:
- All four are thin wrappers. Keep them allocation-minimal.
- `HashFile` must return the *unwrapped* error on open failure so `os.IsNotExist(err)` still works at every call site that checks it today. Grep for `os.IsNotExist` calls on the return of `ContentHash` before finalising.
- Do not add a streaming file helper with fsync/locking — scope creep.

**Tests** (`internal/hashutil/hash_test.go`):
- Known-vector: `HashHex([]byte("abc")) == "ba7816bf..."` (the canonical SHA-256 test vector).
- `HashBytes` + `hex.EncodeToString` on its slice equals `HashHex` output.
- `HashReader` on a 1 MiB buffer equals `HashHex` of the same bytes.
- `HashFile` on missing path returns an error for which `os.IsNotExist` is true.

**Validation**: `go test ./internal/hashutil/ -count=1`.

### Task C — `internal/cacheutil/`

**Agent type**: general-purpose.

**Goal**: version-gated directory lifecycle + sharded entry path + clear-cache registry.

**Files**:
- `internal/cacheutil/versioned_dir.go`
- `internal/cacheutil/sharded.go`
- `internal/cacheutil/registry.go`
- tests alongside each.

**API**:

```go
package cacheutil

// VersionedDir is a cache directory whose contents are invalidated when any of
// the declared schema tokens change. Each token is a (name, value) pair written
// to a sidecar file inside the directory; on open, a mismatch of any token
// triggers a nuke of the entries subtree.
type VersionedDir struct {
    Root       string // absolute path to the cache root, e.g. repoDir/.krit/parse-cache
    EntriesDir string // subdir under Root whose contents are subject to nuke-on-mismatch (default "entries")
    Tokens     []SchemaToken // written to {Root}/{Name} sidecar files
}

// SchemaToken is one named-version dimension. parse-cache today declares two
// (schema version "1", grammar version "smacker/go-tree-sitter@vX.Y.Z"); the
// oracle cache declares one ("version", "1").
type SchemaToken struct {
    Name  string // filename under Root, e.g. "version" or "grammar-version"
    Value string
}

// Open ensures the directory tree exists, checks every token against its
// sidecar, and removes-and-recreates EntriesDir if any mismatch is found.
// Missing sidecars on first run are written without nuking (fresh repo).
// Returns the absolute path to EntriesDir.
func (v VersionedDir) Open() (entriesDir string, err error)

// Clear removes the entire cache root. Safe to call when the dir is missing.
// Used by --clear-cache via the registry.
func (v VersionedDir) Clear() error

// ShardedEntryPath returns "{root}/{hash[:2]}/{hash[2:]}{ext}".
// ext must include the leading dot (".json", ".gob"). Hashes shorter than 3
// chars fall back to "{root}/_/{hash}{ext}" to match today's oracle fallback.
func ShardedEntryPath(root, hash, ext string) string

// Registered is anything that can be enumerated and cleared wholesale.
type Registered interface {
    Name() string      // human-readable label for --clear-cache output
    Clear() error
}

// Register adds c to the global registry. Called from init() blocks in
// subsystems that own caches. Registry order is insertion order; ClearAll
// iterates in registration order and continues past individual failures,
// returning a joined error.
func Register(c Registered)

// ClearAll invokes Clear() on every registered cache. Uses errors.Join;
// never short-circuits.
func ClearAll() error

// Registered returns a snapshot of every currently-registered cache. Used by
// --verbose output and tests.
func Registered() []Registered
```

**Implementation notes**:
- `VersionedDir.Open` atomically writes sidecars with `fsutil.WriteFileAtomic` so a crash mid-bump doesn't leave the dir claiming a version it didn't actually nuke for. Do this *after* the nuke+mkdir, so a reader that sees the new sidecar can trust the entries subtree matches.
- Registry is process-global; use a `sync.Mutex`-guarded slice. Tests need a `clearRegistryForTesting()` hook gated by a `_test.go` export to avoid cross-test pollution.
- `ShardedEntryPath` behavior must match the existing two sites bit-for-bit: `filepath.Join(root, hash[:2], hash[2:]+ext)`, with the `"_"` fallback when `len(hash) < 3`. Verify by comparing against `oracle.entryPath` and `ParseCache.entryPath` on random hashes in the unit test.

**Tests**:
- `VersionedDir.Open` first-run creates sidecars, leaves empty entries dir.
- Change one token → entries dir contents nuked, sidecar updated.
- Preserve unchanged token values across Open calls.
- `ShardedEntryPath` parity test with the two inlined versions (checked into test as gold values).
- Registry: register two fakes, ClearAll calls both even if first errors; errors.Join carries both.

**Validation**: `go test ./internal/cacheutil/ -count=1 -race`.

## Phase 1 — migrate call sites (parallel after Phase 0 lands)

**Ordering rule**: each migration is **one commit per subsystem**, landed atomically (replaces all usages + deletes inlined helpers in that subsystem). Do **not** leave a subsystem half-migrated across commits.

Per the issue: cache formats stay stable. That means the **on-disk bytes must be identical** to pre-migration. The helpers produce the same tempfile semantics, the same hex-lowercase hashes, and the same sharded layout — but verify by running the existing tests for each subsystem after migration and making sure the golden files and round-trip tests still pass.

Each task below lists: files to edit, APIs to replace, tests that must still pass, commit-message hint.

### Task D — `internal/oracle/*.go`

**Dependencies**: Phase 0 complete.

**Edits**:
- `internal/oracle/cache.go`:
  - Replace `ContentHash` body with `hashutil.HashFile(path)` (keep the function name and error shape — callers pattern-match via `os.IsNotExist`).
  - Replace `WriteEntry` atomic block with `fsutil.WriteFileAtomic` (json-marshal payload first, MkdirAll stays in-line since parents may not exist for first-shard writes).
  - Replace `entryPath` with `cacheutil.ShardedEntryPath(filepath.Join(cacheDir, "entries"), hash, ".json")`.
  - Replace `CacheDir` with a `cacheutil.VersionedDir{Root, EntriesDir: "entries", Tokens: []SchemaToken{{"version", fmt.Sprintf("%d", CacheVersion)}}}.Open()` construction.
  - Replace `closureFingerprint`'s empty-case `sha256.Sum256(nil)` + hex with `hashutil.HashHex(nil)`; the per-dep loop stays (it's a streaming hash of sorted hashes, keep as-is).
  - Replace `oracleVersionHash` body's `sha256.Sum256(...)` with `hashutil.HashBytes(...)` → copy first 16 bytes.
- `internal/oracle/invoke_cached.go`:
  - Replace `writeOracleJSON` atomic block with `fsutil.WriteFileAtomic`.
- Add `init()` in oracle that registers a `Registered` wrapper around the cache-root dir (derived from the repo dir lookup). The wrapper's `Clear()` calls `cacheutil.VersionedDir{...}.Clear()` or `os.RemoveAll` of the cache root — simplest is the latter since `--clear-cache` blows everything away regardless of version state.

**Tests that must still pass**:
- `go test ./internal/oracle/ -count=1` — includes `cache_test.go` round-trip tests for `WriteEntry`/`LoadEntry` and classify tests that assert the entries dir layout.

**Commit message**: `refactor(oracle): route cache I/O through fsutil + hashutil + cacheutil (#292)`

### Task E — `internal/scanner/parse_cache.go`

**Dependencies**: Phase 0 complete.

**Edits**:
- Replace `hashContent` with `hashutil.HashHex`.
- Replace `entryPath` with `cacheutil.ShardedEntryPath(filepath.Join(pc.dir, parseCacheEntries), hash, parseCacheExt)`.
- Replace `NewParseCache`'s version + grammar-version + nuke dance with a single `cacheutil.VersionedDir{Root: dir, EntriesDir: parseCacheEntries, Tokens: []SchemaToken{{"version", parseCacheVersionStr}, {"grammar-version", GrammarVersion()}}}.Open()`. This is the best stress test for the two-token case; if it fails behavioral parity, the helper's wrong.
- Replace `saveEntry` atomic block with `fsutil.WriteFileAtomicStream` (gob encoder → bufio writer → tempfile) so we avoid materialising the encoded bytes in memory.
- Keep `ClearParseCache(repoDir)`, but have it delegate to the registered `Registered` entry rather than re-deriving the path. Register in `init()`.

**Tests that must still pass**:
- `go test ./internal/scanner/ -run 'Parse|Grammar' -count=1`.
- `TestClearParseCache_Removes` in `parse_cache_test.go`.

**Commit message**: `refactor(scanner): route parse-cache through fsutil + hashutil + cacheutil (#292)`

### Task F — `internal/scanner/index_cache.go`

**Dependencies**: Phase 0 complete.

**Edits**:
- Delete local `writeFileAtomic` and `encodeGob` helpers; use `fsutil.WriteFileAtomic` for meta.json and `fsutil.WriteFileAtomicStream` for the gob payload.
- Replace `contentHashBytes` with `hashutil.HashHex`.
- `CrossFileCacheDir` currently has no version file — the version lives in `meta.json`. Do **not** change that; wrap it in a `Registered` whose `Clear()` calls `ClearCrossFileCache(dir)`. Register in `init()` — **this closes the bug where `--clear-cache` misses the cross-file cache**.
- `computeCrossFileFingerprint`'s streaming `sha256.New()` loop stays (it's not a bytes-in-one-shot call; the helper doesn't fit).

**Tests that must still pass**:
- `go test ./internal/scanner/ -run 'CrossFile|Index' -count=1`.
- `TestClearCrossFileCache`.

**Commit message**: `refactor(scanner): route cross-file cache through fsutil + hashutil + cacheutil (#292)`

### Task G — `internal/store/file.go`

**Dependencies**: Phase 0 complete.

**Edits**:
- Replace `Put`'s atomic block with `fsutil.WriteFileAtomic`.
- `entryPath`'s 1-level shard + ruleSetHash suffix does **not** match the 2-level `ShardedEntryPath`. Do **not** migrate it; leave as-is. Note this in the commit message — it's intentional divergence because the store has different key semantics (RuleSetHash suffix).
- Register a `Registered` that clears the store root.

**Tests that must still pass**:
- `go test ./internal/store/ -count=1`.

**Commit message**: `refactor(store): route Put atomicity through fsutil (#292)`

### Task H — `internal/scanner/baseline.go`

**Dependencies**: Phase 0 complete.

**Edits**:
- Replace `SaveBaselineJSON`'s `tmp := path + ".tmp"; os.WriteFile; os.Rename` with `fsutil.WriteFileAtomic`.
- This site does not own a version-gated dir; no registry entry.

**Tests that must still pass**:
- `go test ./internal/scanner/ -run Baseline -count=1`.

**Commit message**: `refactor(scanner): use fsutil.WriteFileAtomic for baseline writes (#292)`

### Task I — `internal/cache/cache.go`

**Dependencies**: Phase 0 complete.

**Edits**:
- Replace `Save`'s custom `path + ".tmp.<time>.<rand>"` with `fsutil.WriteFileAtomic`. This also drops the `math/rand` import and the timestamp hack.
- Replace `ComputeRuleHash`, `ComputeConfigHash`, and the file-hash in `newEntry` (around `L117-129`) with `hashutil.HashHex` + truncation. Truncation stays in the caller so we don't need to add `HashHex16` variants.
- `ClearSharedCache` + `Clear` + `ClearDir` remain public API (CLI calls them) but each should be wrapped by a `Registered` entry that routes through them. Register in `init()` (guard against duplicate registration if tests re-init).

**Tests that must still pass**:
- `go test ./internal/cache/ -count=1` — including `TestClearSharedCache` and the save/load round-trip.

**Commit message**: `refactor(cache): route incremental cache through fsutil + hashutil + cacheutil (#292)`

### Task J — `cmd/krit/matrix_cache.go`

**Dependencies**: Phase 0 complete.

**Edits**:
- Replace `saveBaseline`'s tmp+rename with `fsutil.WriteFileAtomic`.
- Replace `hashFileContents`, `hashTargetTree`, and the `computeMatrixBaselineCacheKey` scalars with `hashutil` equivalents where the call is a one-shot `sha256.Sum256` / hex. The streaming `sha256.New()` loops in `computeMatrixBaselineCacheKey` stay (they mix many writes; not a fit for `HashHex`).
- Register `clearMatrixCache` via a `Registered` entry.

**Tests that must still pass**:
- `go test ./cmd/krit/ -count=1` (if there are matrix_cache tests; otherwise the top-level smoke test).

**Commit message**: `refactor(matrix): route cache I/O through fsutil + hashutil + cacheutil (#292)`

### Task K — `internal/fixer/binary.go` atomic file writes

**Dependencies**: Phase 0 complete.

**Edits**:
- `binary.go:170` does `os.WriteFile(bf.TargetPath, bf.Content, 0644)` for the "create file" pass. Replace with `fsutil.WriteFileAtomic(bf.TargetPath, bf.Content, 0644)` after the existing `os.MkdirAll` so a crash during binary-fix apply doesn't leave a truncated output file.
- `os.Rename` in the move pass is fine as-is (rename is already atomic).

**Tests that must still pass**:
- `go test ./internal/fixer/ -count=1`.

**Commit message**: `refactor(fixer): use fsutil.WriteFileAtomic for binary create pass (#292)`

## Phase 2 — wire `--clear-cache` to the registry

**Dependencies**: Phase 1 complete (every subsystem has registered itself).

**Edits in [cmd/krit/main.go](cmd/krit/main.go)** (around L588-610):
- Replace the hand-rolled sequence of `cache.ClearSharedCache` / `cache.Clear` / `scanner.ClearParseCache` calls with a single `cacheutil.ClearAll()`.
- Keep the `--cache-dir` branch for the incremental cache because that one takes a user-provided directory (not the default `.krit/` layout). After `ClearAll()`, if `*cacheDirFlag != ""`, also call `cache.ClearSharedCache(*cacheDirFlag)` for the override case.
- Keep the existing "info: Cache cleared." stderr message. Optional: if `--verbose`, enumerate `cacheutil.Registered()` and print each name (low-cost observability win).
- Call `oracle.FindRepoDir(paths)` once and pass it into whatever registration mechanism needs a repo root. If `VersionedDir.Clear()` needs a root, the registered entry should capture it at registration time — this means registration has to happen **after** `FindRepoDir` resolves, not in `init()`. Two options:
  1. **Preferred**: `cacheutil.Register` takes a closure that receives the repo dir at `ClearAll` time. Add a `BindRepoDir(dir string)` call that's the contract between `cmd/krit/main.go` and subsystems, invoked once early in `main` before any cache work.
  2. Subsystems register eagerly with a lazy-resolved repo dir via a package-level `SetRepoDir(dir)` hook called from `main`.

  Resolve this during Phase 0, Task C — pick whichever API shape lands cleanest in tests. Document the choice at the top of `registry.go`.

**Tests**:
- New test `cmd/krit/clear_cache_test.go` that: creates a temp repo with files under each subsystem's cache path, runs the equivalent of `--clear-cache`, and asserts every cache dir is empty. This is the closing proof that the registry covers what the old hand-rolled sequence covered and more.

**Commit message**: `refactor(cli): route --clear-cache through cacheutil.ClearAll (#292)`

## Validation checklist (run before opening PR)

Run in the worktree root:

```bash
go build -o krit ./cmd/krit/
go vet ./...
go test ./... -count=1
go test ./internal/fsutil/ ./internal/hashutil/ ./internal/cacheutil/ -race -count=1
```

**Manual smoke tests**:
1. `./krit --clear-cache <some repo>` and confirm every `.krit/*-cache` and `.krit-cache` under the repo is gone.
2. `./krit <some repo> --perf` cold vs. warm run: warm timings within noise of `main` (per the issue: "refactor, not optimization"). Signal-Android is the canonical target if available; otherwise use `playground/` samples.
3. `go test ./internal/oracle/ ./internal/scanner/ -run Cache -v` to make sure every cache round-trip test still passes.

**Cross-check the atomicity upgrade**: `go test ./internal/fsutil/ -run TestSyncCalled -race` — add a test that the temp file handle had `Sync()` called before `Rename`. This is the new durability guarantee and the regression test for anyone who later "cleans up" the helper.

## Risk notes the executor must honor

- **Error text**: at each migration site, read the old `fmt.Errorf("...: %w", err)` wrapper before the change and preserve exactly that prefix text on the outer wrap. Example: oracle's `WriteEntry` currently returns `"tempfile: %w"`, `"write tempfile: %w"`, `"close tempfile: %w"`, `"rename %s -> %s: %w"`. The new code should map each `fsutil.WriteFileAtomic` error into the same prefix set via a switch on the wrapped error string, OR accept that the error strings change and update any test that pattern-matches on them. Grep for `strings.Contains` + the prefix before choosing.
- **Registration order**: if Phase 1 Tasks D-J run in parallel worktrees, two or more may register the same subsystem name. Make `cacheutil.Register` idempotent-by-Name (replace-on-duplicate with a `log.Printf` warning under `-verbose`) so parallel agents cannot deadlock the registry.
- **Commit-per-subsystem rule**: Phase 1 agents each produce one commit. Do not combine into a single mega-commit — the issue specifically says "Migrate + delete old code path in one commit per subsystem" to keep bisection useful.
- **Fsync is a behavioral change**: call it out in the Phase 0 Task A commit message explicitly ("Adds fsync to atomic writes; previous inlined sites did not call Sync."). If `--perf` regresses more than noise on warm runs, fsync is the suspect — but do not revert; tune by batching writes, not by dropping durability.

## Parallelism map (for fan-out-fan-in)

- **Round 1** (parallel, 3 agents): Tasks A, B, C.
- **Gate**: Phase 0 merged into the working branch. Helpers available to importers.
- **Round 2** (parallel, 8 agents): Tasks D, E, F, G, H, I, J, K.
- **Gate**: all migrations merged.
- **Round 3** (1 agent): Task (Phase 2) `--clear-cache` wire-up.
- **Round 4** (1 agent): final validation pass + PR description.

Each Round-2 task touches a disjoint file set, so parallel worktrees will not conflict. Round-3 touches `cmd/krit/main.go` which is untouched in Rounds 1-2, so no rebase hazard.
