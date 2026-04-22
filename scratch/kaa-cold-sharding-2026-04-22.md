# Cold KAA Sharding Benchmark - 2026-04-22

Issue: #393

Krit HEAD: `c58da80` plus local RSS/split changes.
Signal-Android: `f63bc27ede`.

Command shape:

```bash
for shards in 1 2 4; do
  ./krit -clear-cache /Users/jason/github/Signal-Android
  rm -f /Users/jason/github/Signal-Android/.krit/types.json
  KRIT_DAEMON_CACHE=off KRIT_TYPES_SHARDS=$shards KRIT_DAEMON_POOL=1 \
    ./krit --report json --perf --no-cache /Users/jason/github/Signal-Android \
    > /tmp/bench_kaa_cold_shards_${shards}.json \
    2> /tmp/bench_kaa_cold_shards_${shards}.err
done
```

## Results

| Shards | Total | typeOracle | jvmAnalyze | max kritTypesProcess | max buildSession | max analyzeFiles | max callResolve | max peak RSS | Findings |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| 1 | 26.060s | 15.347s | 15.217s | 14.788s | 2.951s | 11.195s | 6.311s | 1,919 MB | 7,387 |
| 2 | 25.483s | 14.723s | 14.659s | 14.387s | 3.326s | 10.462s | 6.196s | 1,947 MB | 7,387 |
| 4 | 30.920s | 20.512s | 20.427s | 20.169s | 4.719s | 14.939s | 9.142s | 1,470 MB | 7,387 |

Findings were equivalent after normalizing order:

```text
cmp_1_2=0
cmp_1_4=0
```

Warm follow-up after the 4-shard cold run:

```text
total 2.873s
typeOracle 56ms
findings 7387
cmp_1_warm4=0
```

No `kritTypesProcess` entry appeared in the warm follow-up, so the sharded cold run populated the oracle cache well enough for the unchanged warm run to skip the JVM.

## Split Diagnostics

The 2-shard greedy split was balanced by the new static cost model:

```text
shard 0: files=707 bytes=4,584,343 callTokens=55,504 cost=33,002,391 peakRSS=1,838 MB
shard 1: files=705 bytes=4,632,884 callTokens=55,409 cost=33,002,292 peakRSS=1,947 MB
```

The 4-shard split was also balanced by estimated cost, but all workers slowed down due to duplicated session build and memory/GC pressure. Peak RSS per process dropped, but aggregate concurrent RSS and build-session duplication made wall time worse.

## Conclusion

Keep `KRIT_TYPES_SHARDS` opt-in. On this Signal-Android sample, 2 shards improves max `kritTypesProcess` by only 2.7% and total wall by 2.2%, below the 20-25% target. 4 shards is worse. The RSS/per-shard perf fields are still useful for future machines and post-call-filter regressions.
