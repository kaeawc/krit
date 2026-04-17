# OracleFilterInversion

**Cluster:** [core-infra](README.md) · **Status:** planned ·
**Severity:** n/a (infra) · **Default:** n/a

## What it does

Inverts the oracle filter default from opt-out (`AllFiles: true` for
unaudited rules) to opt-in (`AllFiles: false` unless a rule explicitly
declares oracle need). Eliminates unnecessary JVM invocations for
the 220+ rules that have never been audited and almost certainly do
not need the oracle.

## Current cost

`OracleFilterProvider` is an optional interface rules can implement to
tell the oracle which files to analyse. Rules that do *not* implement
it receive a conservative default:

```go
// internal/rules/rule.go line ~138
var allFilesFilter = &OracleFilter{AllFiles: true}
```

220+ rules have never been audited for oracle usage. Each of them
silently feeds every file to the JVM oracle on every scan, even in
cases where the rule is a pure syntactic check that never calls any
type-resolution API. On a large Android project this causes
unnecessary JVM warm-up, extra memory usage, and slower scan times.

There is no linting or CI gate to catch new rules that forget to
implement `OracleFilterProvider`. The problem grows with every new
rule added.

Relevant files:
- `internal/rules/rule.go` — `OracleFilterProvider`, `allFilesFilter`
- `internal/oracle/` — oracle client, uses the filter to select files

## Proposed design

With [`unified-rule-interface.md`](unified-rule-interface.md) in
place, oracle need is declared via the `Capabilities` bitfield:

```go
const (
    NeedsResolver    Capabilities = 1 << iota
    NeedsModuleIndex
    NeedsCrossFile
    NeedsLinePass
    NeedsOracle      // new: rule may call oracle APIs
)
```

The oracle runner builds its file list from only those rules that
declare `NeedsOracle`. Rules that do not declare it are never fed to
the oracle, regardless of what `OracleFilterProvider` returns. The
`allFilesFilter` default and the `OracleFilterProvider` interface are
deleted.

For the subset of rules that declare `NeedsOracle`, they can further
narrow the file set by implementing a `FileFilter(file *ParsedFile)
bool` method on their `Check` closure (or via an optional interface)
to avoid the "all files" overhead when only specific patterns trigger
oracle use.

## Migration path

1. Add `NeedsOracle` capability bit.
2. Audit the 220+ unaudited rules: the vast majority are syntactic and
   can be confirmed as not needing the oracle in bulk. Mark each
   confirmed rule with neither `NeedsOracle` nor `NeedsResolver`.
3. For rules that genuinely use the oracle, add `NeedsOracle`.
4. Update the oracle runner to select files based on `NeedsOracle`
   declarations.
5. Delete `OracleFilterProvider` interface and `allFilesFilter`.

The bulk audit in step 2 can be scripted: any rule whose `Check`
function body contains no call to `ctx.Resolver` can be automatically
confirmed as oracle-free.

## Acceptance criteria

- `OracleFilterProvider` interface deleted.
- `allFilesFilter` deleted.
- On a scan of a large project with no oracle-needing rules active,
  the oracle is never invoked (verified by `--perf` output showing
  zero oracle calls).
- Rules that declare `NeedsOracle` continue to receive oracle data.
- `go vet` or a custom linter fails if a rule calls `ctx.Resolver`
  without declaring `NeedsResolver`.

## Links

- Depends on: [`unified-rule-interface.md`](unified-rule-interface.md)
  (Capabilities bitfield)
- Depends on: [`type-resolution-service.md`](type-resolution-service.md)
  (`NeedsOracle` and `NeedsResolver` are unified under the resolver
  service)
- Related: `internal/rules/rule.go`, `internal/oracle/`
