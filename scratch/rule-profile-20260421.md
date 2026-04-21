# Rule Profile 2026-04-21

## Command

Unprofiled ranking runs:

```bash
./krit --report json --perf --perf-rules --no-type-oracle --no-cache /Users/jason/github/Signal-Android
```

CPU profile capture:

```bash
./krit --cpuprofile=/tmp/krit-rule-20260421-runN.prof --report json --perf --perf-rules --no-type-oracle --no-cache /Users/jason/github/Signal-Android
go tool pprof -top -cum ./krit /tmp/krit-rule-20260421-run1.prof /tmp/krit-rule-20260421-run2.prof /tmp/krit-rule-20260421-run3.prof
go tool pprof -tags ./krit /tmp/krit-rule-20260421-run1.prof /tmp/krit-rule-20260421-run2.prof /tmp/krit-rule-20260421-run3.prof
```

## Run Summary

| Run | ruleExecution | Top rule | Top rule cumulative |
|---|---:|---|---:|
| 1 | 1,804 ms | NoNameShadowing | 840.0 ms |
| 2 | 1,797 ms | ComposeDerivedStateMisuse | 809.4 ms |
| 3 | 1,839 ms | NoNameShadowing | 841.0 ms |

Median ruleExecution: 1,804 ms.

Median-run ruleExecution children:

| Bucket | Cumulative time |
|---|---:|
| walkTraversal | 135 ms |
| ruleCallbacks | 19,728 ms |
| lineRules | 87 ms |
| suppressionFilter | 4 ms |

The rule rows below are cumulative callback CPU, not wall time. Because rule execution is parallel, cumulative callback time is expected to exceed the ruleExecution wall-clock bucket.

## Top Rule Execution Rows

| Rank | Rule | Family | Time | Share | Calls | Avg |
|---:|---|---|---:|---:|---:|---:|
| 1 | NoNameShadowing | dispatch | 840.0 ms | 4.1% | 2,468 | 340,358 ns |
| 2 | ComposeDerivedStateMisuse | dispatch | 773.0 ms | 3.8% | 103,172 | 7,492 ns |
| 3 | PrintlnInProduction | dispatch | 763.4 ms | 3.8% | 103,172 | 7,399 ns |
| 4 | PrintStackTraceInProduction | dispatch | 659.5 ms | 3.2% | 103,172 | 6,391 ns |
| 5 | DebugToastInProduction | dispatch | 638.1 ms | 3.1% | 103,172 | 6,185 ns |
| 6 | ComposeClickableWithoutMinTouchTarget | dispatch | 516.3 ms | 2.5% | 103,172 | 5,003 ns |
| 7 | UnreachableCode | dispatch | 438.9 ms | 2.2% | 37,048 | 11,847 ns |
| 8 | UnusedParameter | dispatch | 415.9 ms | 2.0% | 16,129 | 25,782 ns |
| 9 | JdbcPreparedStatementNotClosed | dispatch | 398.1 ms | 2.0% | 21,414 | 18,588 ns |
| 10 | InjectDispatcher | dispatch | 355.5 ms | 1.8% | 103,172 | 3,445 ns |
| 11 | UselessCallOnNotNull | dispatch | 336.0 ms | 1.7% | 103,172 | 3,256 ns |
| 12 | UseSparseArrays | dispatch | 328.2 ms | 1.6% | 103,172 | 3,181 ns |
| 13 | UnnecessarySafeCall | dispatch | 297.7 ms | 1.5% | 136,390 | 2,182 ns |
| 14 | IgnoredReturnValue | dispatch | 274.6 ms | 1.4% | 103,172 | 2,661 ns |
| 15 | ComposeRememberWithoutKey | dispatch | 265.1 ms | 1.3% | 103,172 | 2,569 ns |
| 16 | LongParameterList | dispatch | 249.2 ms | 1.2% | 20,021 | 12,444 ns |
| 17 | UseValueOf | dispatch | 241.1 ms | 1.2% | 103,172 | 2,337 ns |
| 18 | ComposeStringResourceInsideLambda | dispatch | 238.9 ms | 1.2% | 103,172 | 2,315 ns |
| 19 | OkHttpClientCreatedPerCall | dispatch | 236.9 ms | 1.2% | 103,172 | 2,296 ns |
| 20 | LoggerWithoutLoggerField | dispatch | 222.8 ms | 1.1% | 103,172 | 2,159 ns |

No individual rule exceeds 5% of cumulative measured rule callback CPU in this run, so this PR does not bundle a rule-specific optimization. The ranking points to shared helper costs (`flatCallExpressionName`, navigation-expression text extraction, and string interning) as the next leverage point.

## Pprof Top Cumulative

Merged CPU profile, three profiled runs, top cumulative entries:

| Function | Cum | Cum % |
|---|---:|---:|
| `pipeline.DispatchPhase.Run.func1` | 19.29 s | 43.86% |
| `rules.(*Dispatcher).RunWithStats` | 18.25 s | 41.50% |
| `rules.(*V2Dispatcher).RunColumnsWithStats` | 18.25 s | 41.50% |
| `rules.runWithRuleProfileLabel` | 16.82 s | 38.24% |
| `runtime/pprof.Do` | 16.61 s | 37.77% |
| `scanner.(*StringPool).Intern` | 11.58 s | 26.33% |
| `scanner.(*File).FlatNodeString` | 11.00 s | 25.01% |
| `scanner.internBytes` | 10.94 s | 24.87% |
| `rules.flatCallExpressionName` | 7.79 s | 17.71% |
| `rules.flatNavigationExpressionLastIdentifier` | 7.47 s | 16.98% |

Pprof rule labels, merged profiled runs:

| Label | Samples |
|---|---:|
| `dispatch` family | 16.51 s |
| `line` family | 0.65 s |
| `PrintlnInProduction` | 0.66 s |
| `ComposeDerivedStateMisuse` | 0.55 s |
| `PrintStackTraceInProduction` | 0.50 s |
| `DebugToastInProduction` | 0.48 s |
| `UnnecessarySafeCall` | 0.47 s |
| `UnusedParameter` | 0.36 s |
| `NoNameShadowing` | 0.33 s |
| `UseSparseArrays` | 0.33 s |

The profiled runs were used for call-stack and label attribution only; the pprof label wrapper has visible overhead, so the hotspot table uses the unprofiled `--perf-rules` runs.
