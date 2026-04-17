# PerFileArenaAllocation

**Cluster:** [performance-infra](README.md) · **Status:** shipped ·
**Severity:** n/a (infra) · **Default:** n/a

## What it does

Allocate all per-file temporaries (intermediate slices, finding
structs before columnar conversion, string builder buffers) in a
bulk-freeable arena. When the file is done, free the entire arena
in one operation — the GC never traces any of it.

## Current cost

Each file scan allocates hundreds of small objects: temporary
string slices in rule helpers, intermediate finding structs,
regex match results, etc. These are short-lived (die when the
file processing goroutine moves to the next file) but the GC
doesn't know that — it traces them all every cycle.

## Go arena support

Go 1.20 introduced `arena` behind `GOEXPERIMENT=arenas`. The API:

```go
a := arena.NewArena()
defer a.Free()

// Allocate inside the arena
s := arena.MakeSlice[FlatNode](a, 0, 2048)
m := arena.New[MyStruct](a)

// a.Free() releases everything — no GC involvement
```

If the arena experiment isn't stabilized by the time this ships,
the same effect can be achieved with a simpler bump allocator
backed by a `sync.Pool` of large byte slices:

```go
type BumpArena struct {
    buf []byte
    off int
}

func (a *BumpArena) Alloc(size int) unsafe.Pointer {
    aligned := (a.off + 7) &^ 7
    if aligned+size > len(a.buf) {
        // grow or panic
    }
    p := unsafe.Pointer(&a.buf[aligned])
    a.off = aligned + size
    return p
}

func (a *BumpArena) Reset() { a.off = 0 }
```

The bump allocator is returned to a `sync.Pool` after each file.
Zero GC involvement for the per-file data.

## Scope of arena-managed data

- `[]FlatNode` from the tree flattening pass
- Per-rule intermediate slices (e.g., collected modifier nodes)
- Temporary `Finding` structs before `FindingCollector.Append`
  converts them to columnar
- String builder buffers for finding messages

**Not** arena-managed:
- The file content `[]byte` (mmap'd or pool'd separately)
- The interned string pool (lives across files)
- The global `FindingColumns` (lives across files)

## Expected impact

Dramatic reduction in GC work on large scans. On a 5000-file repo,
each file currently generates ~200 GC-traced objects; with arena
allocation, the GC sees zero per-file objects. Combined with
columnar finding storage, the total GC-traced set shrinks to the
cross-file structures only (intern pool, finding columns, code
index).

## Acceptance criteria

- `go test -bench` with `GODEBUG=gctrace=1` shows ≥ 50% fewer GC
  cycles on a 500-file synthetic benchmark.
- Works with both the `GOEXPERIMENT=arenas` path and the fallback
  bump allocator.
- No unsafe memory access — arena-allocated data is never referenced
  after `arena.Free()` / `Reset()`.

## Links

- Parent: [`roadmap/65-performance-infra.md`](../../65-performance-infra.md)
- Depends on: [`flat-node-representation.md`](flat-node-representation.md)
  (the `[]FlatNode` slice is the primary arena consumer)
- Depends on: [`columnar-finding-storage.md`](columnar-finding-storage.md)
  (findings escape the arena into the global columns)
