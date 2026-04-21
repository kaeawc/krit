# Benchmark Reproduction Prompt — krit on Signal-Android

Self-contained instructions for running cold / warm / single-file-edit
benchmarks against Signal-Android. Hand this to a fresh agent or run
it yourself; all commands are cut-and-paste.

---

## Prerequisites

- Krit repo checked out at `~/kaeawc/krit` (or adjust `KRIT_DIR` below).
- Signal-Android checked out at `~/github/Signal-Android` (or adjust
  `REPO` below). Any recent `main` of
  https://github.com/signalapp/Signal-Android works; results below
  assume roughly 3877 Kotlin / 3259 Java / 9292 XML files.
- Go 1.22+ on `PATH`. Python 3 on `PATH` for the JSON parsing helper.
- macOS or Linux. APFS or ext4 both fine; file-count characteristics
  are similar.

```bash
export KRIT_DIR=~/kaeawc/krit
export REPO=~/github/Signal-Android
cd "$KRIT_DIR"
```

## Step 1 — Build a fresh binary

Always build against the commit you're benchmarking. Don't reuse an
old binary.

```bash
cd "$KRIT_DIR"
go build -o krit ./cmd/krit/
./krit --version
```

## Step 2 — Record the baseline revision

So future comparisons know what they're comparing against.

```bash
git -C "$KRIT_DIR" log --oneline -1
git -C "$REPO" log --oneline -1
```

## Step 3 — Clear all on-disk caches for Signal

Every cache (parse-cache, cross-file-cache, cross-file-shards,
resource-cache, xml-parse-cache, oracle-cache) lives under
`{REPO}/.krit/`. Nuke the whole directory for a deterministic cold
run.

```bash
"$KRIT_DIR/krit" -clear-cache "$REPO"
# Verify:
ls -la "$REPO/.krit/" 2>/dev/null || echo "no .krit/ — clean"
```

## Step 4 — Cold run (no caches, no oracle)

The oracle is a JVM subprocess — disable it with `--no-type-oracle` so
the benchmark measures only Go-side work. The `--no-cache` flag
disables the **incremental-analysis cache** (the rule-finding cache),
which would otherwise let the warm run skip `ruleExecution` and
under-count the phase.

```bash
"$KRIT_DIR/krit" \
  --report json --perf --no-type-oracle --no-cache \
  "$REPO" > /tmp/bench_cold.json 2> /tmp/bench_cold.err
echo "exit=$?"
grep -E "Found|info:" /tmp/bench_cold.err
```

Expected: exit=1 (any findings → non-zero exit), `Found ~9700 issue(s)`,
wall around 25–40s on Apple Silicon.

## Step 5 — Warm runs (three, use the median)

Each run populates every cache on the first miss and reads them on
subsequent runs. Run three times; report the median. The first warm
run is often slightly slower because the monolithic cross-file cache
is being written on that run.

```bash
for i in 1 2 3; do
  "$KRIT_DIR/krit" \
    --report json --perf --no-type-oracle --no-cache \
    "$REPO" > /tmp/bench_warm${i}.json 2> /tmp/bench_warm${i}.err
  echo "warm-$i: $(grep -o 'in [0-9.]*s' /tmp/bench_warm${i}.err)"
done
```

Expected: each run 2–3s wall, numbers within ~5% of each other.

## Step 6 — Single-file-edit (SFE)

Simulates the editor/LSP workflow: the user changed one file, krit
runs against the whole repo but should reuse every other file's
caches.

**`touch` is not enough** — it only updates mtime, not content. Krit
keys caches on content hashes (xxh3-256 via the hashutil memo), so
mtime-only changes still hit the monolithic cache. Append a blank
line to force a genuine content-hash change, then revert.

```bash
# Pick any .kt file that's tracked by git (so revert works).
EDITED="$REPO/app/src/main/java/org/thoughtcrime/securesms/MainActivity.kt"
echo "" >> "$EDITED"

"$KRIT_DIR/krit" \
  --report json --perf --no-type-oracle \
  "$REPO" > /tmp/bench_sfe.json 2> /tmp/bench_sfe.err

# Revert. Use perl chomp if the file isn't tracked by git.
git -C "$REPO" checkout -- app/src/main/java/org/thoughtcrime/securesms/MainActivity.kt 2>/dev/null \
  || perl -pi -e 'chomp if eof' "$EDITED"

echo "SFE: $(grep -o 'in [0-9.]*s' /tmp/bench_sfe.err)"
```

Note: SFE **does not** pass `--no-cache`. We want the incremental
cache active (that's the realistic editor scenario — one file changed,
rules re-run only on that file). Wall should be 3–5s.

## Step 7 — Restore steady-state

The SFE run invalidates the monolithic cross-file cache and writes
fresh shards. Run one more warm to confirm the cache re-stabilises.

```bash
"$KRIT_DIR/krit" \
  --report json --perf --no-type-oracle --no-cache \
  "$REPO" > /tmp/bench_warm_post.json 2> /tmp/bench_warm_post.err
echo "post-SFE warm: $(grep -o 'in [0-9.]*s' /tmp/bench_warm_post.err)"
```

Expected: back under 3s.

## Step 8 — Parse results into a phase table

```bash
python3 - <<'PY'
import json

runs = [
    ('cold',          '/tmp/bench_cold.json'),
    ('warm-1',        '/tmp/bench_warm1.json'),
    ('warm-2',        '/tmp/bench_warm2.json'),
    ('warm-3',        '/tmp/bench_warm3.json'),
    ('SFE',           '/tmp/bench_sfe.json'),
    ('warm-post-SFE', '/tmp/bench_warm_post.json'),
]

# Phases worth comparing across runs.
PHASES = [
    'collectFiles','parse','typeIndex','ruleExecution','cacheSave',
    'crossFileAnalysis','javaIndexing','indexBuild',
    'kotlinIndexCollection','javaReferenceCollection','xmlReferenceCollection',
    'lookupMapBuild','crossRules',
    'androidProjectAnalysis','resourceDirScan','valuesXMLParseCPU',
    'layoutDirScan','manifestParse',
    'total',
]

def flatten(nodes, out=None):
    if out is None: out={}
    for n in nodes:
        out[n['name']] = n.get('durationMs', 0)
        flatten(n.get('children', []), out)
    return out

header = f"{'Phase':<28}" + ''.join(f"{r[0]:>16}" for r in runs)
print(header)
print('-' * len(header))

for phase in PHASES:
    row = [f"{phase:<28}"]
    for _, path in runs:
        try:
            d = json.load(open(path))
            flat = flatten(d.get('perfTiming', []))
            v = flat.get(phase, 0)
            row.append(f"{v:>13}ms" if v else f"{'-':>15}")
        except Exception:
            row.append(f"{'err':>15}")
    print(''.join(row))
PY
```

## Step 9 — Inspect cache stats and budget

```bash
python3 - <<'PY'
import json

for label, path in [('warm-steady', '/tmp/bench_warm2.json'),
                    ('SFE',         '/tmp/bench_sfe.json')]:
    d = json.load(open(path))
    print(f'=== {label} — caches ===')
    for c in d.get('caches', []):
        s = c['stats']
        t = s['hits'] + s['misses']
        rate = f"{100*s['hits']//t}%" if t else '—'
        print(f"  {c['name']:<25} entries={s['entries']:>8} "
              f"bytes={s['bytes']//1024//1024:>4}MB  "
              f"hits={s['hits']:>5}  misses={s['misses']:>4}  "
              f"evictions={s['evictions']:>3}  hit-rate={rate}")
    b = d.get('cacheBudget')
    if b:
        print(f"  cap={b['capBytes']//1024//1024}MB  used={b['usedBytes']//1024//1024}MB "
              f"({100*b['usedBytes']//b['capBytes']}%)")
    print()
PY
```

## What "good" looks like on Signal

Rough targets on current `main` (as of 2026-04-21, post PRs
#311–#347, pre #348 bugfix):

| Metric | Expected range |
|---|---:|
| Cold total | 25–35s |
| Cold `parse` | 4–6s |
| Cold `crossFileAnalysis` | 18–25s |
| Cold `androidProjectAnalysis` | 3–4s |
| Warm steady total | 2.5–3s |
| Warm `parse` | 80–120ms |
| Warm `crossFileAnalysis` | 500–600ms |
| Warm `androidProjectAnalysis` | 100–120ms |
| Warm `lookupMapBuild` | 0ms (loaded from #309 cache) |
| SFE total | 3.5–4.5s |
| SFE `lookupMapBuild` | 380–480ms |
| SFE cross-file-shards hit rate | ~99.98% (5498/5499) |
| Warm-cache bytes (parse+xml+resource+cross-file) | 200–250MB |

Deviation >20% from these ranges on the same hardware should prompt
a git bisect.

## Flags cheat-sheet

| Flag | When to include |
|---|---|
| `--no-type-oracle` | **Always** for Go-side benchmarks. Otherwise a JVM spawn dominates wall time. |
| `--no-cache` | For cold + warm baseline runs (disables **incremental** cache, the rule-finding cache). Omit for SFE — the realistic editor scenario keeps it on. |
| `--no-parse-cache` | Rarely. Use only when isolating parse-phase cost. |
| `--no-cross-file-cache` | Rarely. Use only when isolating cross-file phase cost. |
| `--report json --perf` | Required for programmatic parsing. `--perf` emits the `perfTiming` tree; `--report json` emits the `caches` + `cacheBudget` sections. |
| `--clear-cache` | Only to reset between benchmark sessions. **Not** part of any individual run. |

## Troubleshooting

- **`exit=1` is normal.** Krit exits non-zero when findings exist.
  Only worry if exit is 2+ (error) or stderr mentions `panic`.
- **Warm run fluctuates widely.** First warm is often +20% over
  subsequent warms because the monolithic cache is being written,
  not just read. Run at least three, ignore the first.
- **SFE feels slow.** Check the cache stats section — if
  `cross-file-shards` shows ~100% hits, the shard path is working
  and the bottleneck is `lookupMapBuild` + shard I/O (tracked in
  issue #335 and pack-file / bbolt discussion).
- **Cache bytes exceed cap.** Each cache has its own cap
  (parse-cache 200MB, resource-cache 128MB, etc.); they are NOT a
  shared pool. The `cacheBudget` section sums them as if they were,
  which is misleading until the shared-budget work lands.
- **Numbers differ from the runbook "good" ranges by 2–3×.** Check
  `sysctl -n hw.ncpu`; the parallel phases scale with core count.
  Runbook figures were collected on an M-series Mac with 10+ cores.

## Capture for later comparison

To save a run for diffing after a change:

```bash
mkdir -p "$KRIT_DIR/scratch/benchmarks"
ts=$(date +%Y-%m-%d_%H%M%S)
rev=$(git -C "$KRIT_DIR" rev-parse --short HEAD)
cp /tmp/bench_cold.json       "$KRIT_DIR/scratch/benchmarks/${ts}_${rev}_cold.json"
cp /tmp/bench_warm2.json      "$KRIT_DIR/scratch/benchmarks/${ts}_${rev}_warm.json"
cp /tmp/bench_sfe.json        "$KRIT_DIR/scratch/benchmarks/${ts}_${rev}_sfe.json"
echo "saved under scratch/benchmarks/ with prefix ${ts}_${rev}_"
```

To diff later:

```bash
python3 "$KRIT_DIR/scratch/benchmark-diff.py" \
  old.json new.json  # (script not yet written; run the Step 8 parser
                     #  on each file and eyeball the diff for now)
```
