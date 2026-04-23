=== Cold KAA Oracle Benchmark ===
Date: 2026-04-22
Platform: Darwin arm64
Config: cold start, `-no-cache`, `-no-cache-oracle`, `-perf`
Branch: work/eager-swanson-affdc4 (issue #420 infrastructure landed)
Binary: krit dev (worktree, includes DeclarationProfile wiring — full profile default)
Signal-Android: 2436 Kotlin files / 2432 analyzed by KAA

## Headline numbers

| Run | Total | jvmAnalyze | JVMProcess | ktBuildSession | ktAnalyzeFiles | Findings | Rules |
|-----|------:|-----------:|-----------:|---------------:|---------------:|---------:|------:|
| 1   | 21413ms | 18050ms | 17942ms | 2875ms | 14373ms | 7535 | 127 |
| 2   | 21830ms | 18865ms | 18755ms | 3063ms | 14932ms | 7535 | 127 |

Average `kotlinAnalyzeFiles`: ~14,652ms  
Average `jvmAnalyze`: ~18,458ms  
Average total: ~21,622ms

## Phase breakdown

### typeOracle phase (19s wall)
- `jvmAnalyze`: ~18.5s — entire JVM invocation from fork to JSON write
  - `kritTypesProcess`: ~18.3s — JVM subprocess clock time
  - `kotlinBuildSession`: ~3.0s — KAA session + module setup
  - `kotlinAnalyzeFiles`: ~14.7s — **main declaration extraction phase** (issue #420 target)
  - `kotlinOracleJsonBuild`: ~70ms — JSON serialization

### Non-oracle phases
- `parse`: 150–370ms (second run benefits from parse cache)
- `typeIndex`: ~27ms
- `ruleExecution`: ~1800ms

## Oracle filter and call filter

- Oracle filter: 2468/2468 files (AllFiles short-circuit — no reduction)
- Call filter: 80 callee names, 25 lexical hints, 34 lexical skips
  - `lexicalSkips: 34` reflects PR #453 work (lexical skip filtering)

## Comparison to prior baselines

The `kotlinAnalyzeFiles` phase shows 14–15s. The issue #420 description cited ~6.8s
from the original benchmark. Discrepancy is consistent with:
- AppCDS not active (no pre-warmed class-data archive on this run)
- Darwin arm64 vs prior run environment
- Higher file count (2436 vs whatever was used originally)

## What this baseline establishes

1. **Current state = full profile, no optimization yet.** Issue #420 wired the
   `--declaration-profile` flag end-to-end and added the fingerprint cache key,
   but `BuildOracleDeclarationProfileV2` still returns `FullDeclarationProfile()`.
   No rules have opted into narrower profiles yet.

2. **PR #453 baseline is already baked in.** The `lexicalSkips: 34` and
   `calleeNames: 80` in the call filter reflect the lexical-skip work from #453.
   The oracle call filter is active and filtering.

3. **Speedup target for issue #420.** When rules declare narrow
   `DeclarationProfile{ClassShell: true, Supertypes: true}` (skipping Members,
   MemberSignatures, MemberAnnotations), the `kotlinAnalyzeFiles` phase should
   drop. Target: ≥20% reduction (→ ≤11.7s) for a balanced rule set.

## Benchmark script

```bash
scripts/benchmark-oracle.sh /path/to/project [runs]
```

Deletes `.krit/types.json` before each run to force a cold JVM invocation.
Use this script for all future oracle performance comparisons.
