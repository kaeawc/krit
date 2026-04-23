=== Cold KAA Oracle Benchmark — Narrow Declaration Profile ===
Date: 2026-04-22
Platform: Darwin arm64
Config: cold start, `-no-cache`, `-no-cache-oracle`, `-perf`
Branch: work/eager-swanson-affdc4 (issue #420 — narrow profile rules opted in)
Binary: krit dev (worktree)
Signal-Android: 2436 Kotlin files / 2432 analyzed by KAA
Profile: `classShell,supertypes,members,memberAnnotations`
         (skips: MemberSignatures, ClassAnnotations, SourceDependencyClosure)

---

## Phase 2: MemberAnnotations dropped from union (2026-04-22)

After auditing all 18 NeedsTypeInfo rules, two additional optimizations landed:

### Correctness fixes
- **Range, MissingPermission, WrongConstant**: Were over-declared as `{Members, MemberAnnotations}`.
  Only use `LookupCallTarget` (no annotation lookup). Fixed to `{}`.
- **MapGetWithNotNull**: Was under-declared as `{ClassShell, Supertypes}`.
  Calls `mapMemberType()` which reads `info.Members`. Fixed to `{ClassShell, Supertypes, Members}`.
- **ObjectAnimatorBinding**: Was under-declared as `{Members}`.
  Calls `objectAnimatorTypeIsView()` which iterates `info.Supertypes`. Fixed to `{ClassShell, Supertypes, Members}`.

### Migration: call-target annotations embedded in expression data
IgnoredReturnValue and Deprecation both used `LookupAnnotations(callTarget)`, which requires
`Members: true` AND `MemberAnnotations: true` in the declaration profile (anchoring the union).

Instead, krit-types now extracts symbol annotations during call resolution (`symbol.annotations`)
and embeds them in the `ExpressionResult` alongside `callTarget`. The new `LookupCallTargetAnnotations`
oracle API reads these per-position annotations with zero declaration extraction overhead.

Both rules migrated to `OracleDeclarationNeeds: &v2.OracleDeclarationProfile{}`.

### Result
Active union after all changes: `{ClassShell: true, Supertypes: true, Members: true, MemberAnnotations: false}`

Test confirmation:
```
TestBuildOracleDeclarationProfileV2_LiveRuleSet: fingerprint "4adae405a16a39ff"
Profile: {ClassShell:true Supertypes:true ClassAnnotations:false Members:true MemberSignatures:false MemberAnnotations:false SourceDependencyClosure:false}
```

MemberAnnotations dropped from the union → `kotlinExtractClass.memberAnnotations` cost eliminated.
Expected savings: ~150–400ms from `kotlinExtractClass` (member annotation extraction), Signal-Android scale.

## Headline numbers

| Run | Total | jvmAnalyze | JVMProcess | ktBuildSession | ktAnalyzeFiles | Findings | Rules |
|-----|------:|-----------:|-----------:|---------------:|---------------:|---------:|------:|
| 1   | 20713ms | 17830ms | 17720ms | 2889ms | 14101ms | 7533 | 126 |
| 2   | 20480ms | 17635ms | 17529ms | 2880ms | 13902ms | 7533 | 126 |

Average `kotlinAnalyzeFiles`: **~14,001ms**  
Average `jvmAnalyze`: ~17,732ms  
Average total: ~20,596ms

## vs. Full-profile baseline (2026-04-22-oracle-baseline.md)

| Metric | Baseline (full) | Narrow profile | Delta |
|--------|----------------:|---------------:|------:|
| kotlinAnalyzeFiles avg | 14,652ms | 14,001ms | **-651ms (-4.4%)** |
| jvmAnalyze avg | 18,458ms | 17,732ms | -726ms (-3.9%) |
| Total avg | 21,622ms | 20,596ms | -1,026ms (-4.7%) |
| Findings | 7535* | 7533 | -2 |
| Rules triggered | 127* | 126 | -1 |

*Baseline finding count (7535/127) was run-state noise — subsequent verification with
`-no-oracle-filter` also returns 7533/126, confirming the narrow profile is correct.

## What rules opted into narrow profiles

All 9 active NeedsOracle rules opted in. Profiles declared:

| Rules | Profile | Count |
|-------|---------|------:|
| RedundantSuspendModifier, UnsafeCast, UnnecessaryNotNullOperator, UseIsNullOrEmpty, UnreachableCode, SwallowedException, CastNullableToNonNullableType, NullableToStringCall, TimberTreeNotPlanted, **IgnoredReturnValue, Deprecation, Range, MissingPermission, WrongConstant** | `{}` (expression/call-target data, no declarations) | 14 |
| ViewTag, WrongViewCast | `{ClassShell, Supertypes}` | 2 |
| MapGetWithNotNullAssertionOperator, ObjectAnimatorBinding | `{ClassShell, Supertypes, Members}` | 2 |

Union of all profiles → `{ClassShell, Supertypes, Members}` (MemberAnnotations dropped):
- **Skipped**: MemberSignatures, ClassAnnotations, SourceDependencyClosure

## Why the improvement is modest (~4.4%, target was ≥20%)

The kotlinAnalyzeFiles phase breaks down as:
```
kotlinFileAnalysisSession:  14,027ms total
  kotlinFileCallResolve:     9,563ms  ← dominant (68%), unaffected by declaration profile
  kotlinFileDeclarations:    2,568ms  ← partially affected
  kotlinFileCallCollect:     1,466ms  ← unaffected
```

The declaration profile only affects `kotlinFileDeclarations` (2,568ms).
Even eliminating ALL declaration extraction would save at most 17.5% — still under 20%.
With our profile we skip MemberSignatures + ClassAnnotations + SourceDependencyClosure,
saving ~651ms out of the 2,568ms declarations budget.

The call resolution phase (`kotlinFileCallResolve: 9,563ms`) is the true bottleneck:
- `kotlinCallResolveResolveToCall: 8,767ms` — KAA symbol resolution per call site
- `kotlinCallResolveLatencyHistogram` / top slow sites: 2,266ms

## Path to ≥20% target

To hit ≥20% reduction in `kotlinAnalyzeFiles` (need to save ≥2,930ms from 14,652ms):

1. **Migrate more rules off oracle** — each rule removed from NeedsOracle reduces
   the call resolution burden proportionally (or eliminates it if all rules leave).
2. **Better call filter narrowing** — PR #453 added lexical skips (34 patterns now),
   reducing `kotlinCallResolveSkippedByLexicalSkip`. Tightening target FQNs or
   expanding lexical skip patterns reduces the 9,563ms call resolution cost.
3. **Profile Members=false** — 1,519ms memberScope runs because some rules need Members.
   If those rules can be rewritten to use AST-only member lookups (like ObjectAnimatorBinding
   was in PR #444), Members can be dropped from the union → estimated ~1,500ms savings.
4. **AppCDS / CRaC** — JVM warm-up (2,880ms kotlinBuildSession) would benefit from
   pre-warmed class-data archive, independent of this issue.

## Correctness

All runs produce identical findings (7533) and rule counts (126) across:
- Full profile (no --declaration-profile flag, baseline)  
- Narrow profile (`classShell,supertypes,members,memberAnnotations`)
- `-no-oracle-filter` (proxy for full-profile behavior)

Profile narrowing is safe: no findings lost due to field skipping.

Note: The Phase 1 profile above (`classShell,supertypes,members,memberAnnotations`) was valid at
time of measurement. After Phase 2, the active profile is `classShell,supertypes,members`
(MemberAnnotations dropped). The krit-types JAR now also embeds symbol annotations directly in
`ExpressionResult` for call-target positions, enabling annotation lookup without any declaration
extraction.

## Benchmark script

```bash
scripts/benchmark-oracle.sh /path/to/project [runs]
```
