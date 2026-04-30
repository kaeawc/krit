---
name: krit-kaa-benchmarking
description: Use when benchmarking Krit's Kotlin Analysis API oracle on a large Kotlin/Android repo, comparing cold vs warm KAA behavior, or investigating KAA regressions, oracle filters, call-target filters, and rule-driven KAA workload.
---

# Krit KAA Benchmarking

Use this workflow for cold/warm Kotlin Analysis API benchmarking, especially on `~/github/Signal-Android`.

## Setup

Build a fresh binary from the Krit repo:

```bash
go build -o krit ./cmd/krit/
```

Verify the oracle JAR exists:

```bash
ls -lh tools/krit-types/build/libs/krit-types.jar
```

Record revisions when reporting numbers:

```bash
git log --oneline -1
git -C ~/github/Signal-Android log --oneline -1
```

## Cold KAA

Use the checked-in script. It deletes `.krit/types.json` before each run and uses `-no-cache -no-cache-oracle`.

```bash
KRIT="$PWD/krit" scripts/benchmark-oracle.sh ~/github/Signal-Android 2
```

Report:

- total `durationMs`
- `typeOracle`
- `jvmAnalyze`
- `kritTypesProcess`
- `kotlinBuildSession`
- `kotlinAnalyzeFiles`
- oracle filter file count
- call filter callee/hint/skip counts
- KAA files analyzed
- findings and triggered rules

Treat `kotlinAnalyzeFiles` as the main KAA extraction/resolve metric. Treat total as user-visible cold runtime.

## Warm KAA

After cold KAA has produced `.krit/types.json`, measure warm oracle loading while still disabling the incremental findings cache:

```bash
for i in 1 2 3; do
  ./krit -no-cache -perf -f json -q \
    -o "/tmp/krit_signal_kaa_warm${i}.json" \
    ~/github/Signal-Android || true
done
```

Summarize phases:

```bash
python3 - <<'PY'
import json
paths=[(f"warm-{i}",f"/tmp/krit_signal_kaa_warm{i}.json") for i in range(1,4)]
phases=["total","typeOracle","jvmAnalyze","parse","typeIndex","ruleExecution","crossFileAnalysis","indexBuild","androidProjectAnalysis"]
def flat(nodes,out=None):
    out={} if out is None else out
    for n in nodes:
        out[n["name"]]=n.get("durationMs",0)
        flat(n.get("children",[]),out)
    return out
print(f"{'phase':<24}"+''.join(f"{name:>12}" for name,_ in paths))
for ph in phases:
    row=f"{ph:<24}"
    for _,p in paths:
        d=json.load(open(p)); f=flat(d.get("perfTiming",[]))
        v=d.get("durationMs",0) if ph=="total" else f.get(ph,0)
        row+=f"{v:>9}ms" if v else f"{'-':>12}"
    print(row)
PY
```

Warm KAA should have `typeOracle` in tens of milliseconds and no `jvmAnalyze` phase. If warm total is high, investigate rule execution, cross-file analysis, or config, not KAA.

## Regression Search

When cold KAA slows down:

1. Compare active oracle rules with `KotlinOracleRulesV2`.
2. Check `OracleCallTargets`: any `AllCalls` or missing filters can explode `resolveToCall`.
3. Check `OracleDeclarationNeeds`: any nil declaration profile forces full declaration extraction.
4. Inspect call-filter stats: callee names, lexical hints, lexical skips, disabled rules.
5. Use no-oracle perf rules to separate KAA costs from Go-side dispatch costs:

```bash
./krit -no-cache -no-type-oracle -perf -perf-rules -f json -q \
  -o /tmp/krit_signal_perf_rules_no_oracle.json \
  ~/github/Signal-Android || true
jq -r '.perfRuleStats[:30][] | [.rule,.invocations,.durationMs,.avgNs,.sharePct] | @tsv' \
  /tmp/krit_signal_perf_rules_no_oracle.json
```

## Reporting Standard

Always say whether Signal's `krit.yml` was applied. If running from outside the target repo, confirm config discovery or pass `--config ~/github/Signal-Android/krit.yml`.

Do not report cached warm findings as truth after rule/config changes. Use `-no-cache` for truthful finding counts.
