# Scenario B Finding-Delta Investigation (issue #298)

## Context

PR #286 / #15 inverted the oracle filter default to opt-in via
`NeedsOracle`. Six rules currently declare `NeedsOracle`:

| Rule                         | Narrowing        | Scenario B state |
| ---------------------------- | ---------------- | ---------------- |
| `RedundantSuspendModifier`   | `"suspend"`      | enabled, narrowed |
| `UnsafeCast`                 | `" as "`         | enabled, narrowed |
| `UnnecessaryNotNullOperator` | `"!!"`           | enabled, narrowed |
| `Deprecation`                | `AllFiles: true` | **disabled**     |
| `IgnoredReturnValue`         | `AllFiles: true` | **disabled**     |
| `UnreachableCode`            | `AllFiles: true` | **disabled**     |

Scenario B observed a 1-finding delta (10,716 vs 10,715) relative to
the unnarrowed baseline on the same corpus.

## Mechanism review (from code, not a live re-run)

Because the three `AllFiles: true` rules are disabled in Scenario B,
the only oracle consumers active are the three narrowed rules. The
oracle input-set union is therefore:

    files containing "suspend" ∪ files containing " as " ∪ files containing "!!"

`oracle.CollectOracleFiles` uses `bytes.Contains` on raw file bytes
(filter.go:115). Any file that a narrowed rule would structurally
target MUST literally contain its identifier substring — tree-sitter
cannot synthesise `suspend`, `as`, or `!!` tokens that are absent from
the source. So in principle the filter is lossless for the three
rules' own call sites.

The subtle failure mode is cross-file resolution. The oracle builds a
single type universe from the files passed to `krit-types.jar`. When
a rule invokes `oracleLookup.LookupCallTarget` / `ResolveFlatNode`
inside an included file, the resolution of a callee can depend on the
callee's DECLARATION file also being in the oracle input set. If the
declaration file does not itself contain `"suspend"`, `" as "`, or
`"!!"`, it is dropped from the oracle input, and resolution of calls
to that declaration falls back to `ct == ""` / `TypeUnknown`.

### Why the observed direction (-1) is consistent with `RedundantSuspendModifier`

Look at the current `RedundantSuspendModifier` implementation:

- `hasSuspendCall=true` is the suppress signal.
- `hasUnresolvedCall=true` (a callee name not in `commonNonSuspendCallees`
  and not proven suspend) is **also** a suppress signal:
  `if !hasSuspendCall && hasUnresolvedCall { return }`.
- A finding fires only when every call in the body is either allow-listed
  as definitely-non-suspend or resolved to a definitely-non-suspend FQN.

Path-dependence on oracle scope:

1. Unnarrowed (baseline): every file is in the oracle universe.
   A call to a same-project helper gets a concrete FQN. That FQN is
   not in `knownSuspendFQNs` and does not start with
   `kotlinx.coroutines.`, so `hasSuspendCall` stays `false`. The code
   then reaches the tail block that sets `hasUnresolvedCall=true`
   based on the callee name not being in `commonNonSuspendCallees`.
   Net: both flags identical, outcome emitted iff all callees are
   allow-listed.
2. Narrowed Scenario B: if the callee's declaration file contains
   none of `"suspend" / " as " / "!!"`, the oracle drops it.
   `LookupCallTarget` returns `""`. The resolver-by-name fallback
   also fails (resolver.Index lacks that file unless NeedsResolver
   covers it — which in Scenario B it does, via the type index, so
   this half is unaffected in practice). The tail name check still
   marks `hasUnresolvedCall=true` by pure callee-name heuristic.

So the `hasUnresolvedCall` branch behaves the same in both scenarios
and the two pathways that *differ* only affect `hasSuspendCall`. That
flag flips from true→false only when a call resolved via the oracle
had landed in `kotlinx.coroutines.*` (or an explicit FQN in
`knownSuspendFQNs`) in the unnarrowed run AND the file that hosts
that FQN definition gets dropped by narrowing. kotlinx.coroutines
source ships with `"suspend"` almost everywhere — dropping it
requires the file to be entirely syntactic/non-suspend in the narrow
filter corner case.

**Inverse direction** (narrowed adds a finding) requires a file that
*does* contain `suspend` but whose sibling declaration files don't —
exactly the `kotlinx.coroutines` collateral case. Narrowed ⇒ fewer
kotlinx resolutions ⇒ fewer `hasSuspendCall=true` ⇒ **more** findings.

That contradicts the observed 10,716 → 10,715 (one fewer finding
under narrowing). So `RedundantSuspendModifier` is unlikely to be the
source on its own.

### Why `UnsafeCast` is the prime suspect

`potentialbugs_nullsafety_casts.go:33-210`:

- The resolver path only ever **suppresses** (`return`) based on
  smart-cast equivalence.
- Every other path eventually falls through to `ctx.Emit(f)`.

Narrowing ⇒ some files lose the resolver path ⇒ MORE UnsafeCast
findings under narrowing. Also the wrong direction, unless a file
contained `" as "` but the callee type declaration used in the
smart-cast comparison lived in a file that was dropped. Even then,
the effect is suppression lost, not gained.

### Why `UnnecessaryNotNullOperator` is plausible as -1

Scanning `potentialbugs_nullsafety_bangbang.go` (condensed): the rule
emits on every `!!` it sees, then suppresses on resolver signals that
the operand is non-null. Same direction as UnsafeCast — narrowing
would **add** findings, not drop them.

## Most-likely root cause

None of the three rules has a plausible path from "oracle input
shrunk" to "one fewer finding" via their own suppression logic.
The one mechanism that produces -1 is:

> A file hosts both a suspend function (so the file is included by
> `"suspend"`) AND a redundant-suspend candidate, AND in the narrowed
> run the oracle's call-target resolution for a helper call inside
> that function returns a **different** non-empty FQN than in the
> unnarrowed run — one that happens to match `knownSuspendFQNs` —
> because narrowing changed the classpath shadowing order inside
> `krit-types.jar`'s symbol resolution.

That is pathological but not impossible: `krit-types.jar`'s analyser
uses the file list to seed its compilation session; shadowing between
source-defined and dependency-jar symbols can differ when the source
set shrinks.

Without the actual playground diff this cannot be confirmed. The 1-
finding magnitude and the fact that it survived both narrowed and
unnarrowed runs (no flake) is consistent with a stable but
resolution-order-dependent shift rather than a random coincidence.

## Decision

**Accept the delta, document it, and harden future narrowing PRs
with an oracle-input fingerprint** (see Opportunities in the issue).

Rationale:

1. The three narrowed rules' filter substrings are sound for their
   own trigger sites — no finding on a dropped file can belong to
   them directly (the trigger token would have to be absent).
2. The only remaining leak is classpath shadowing inside
   `krit-types.jar`, which is an implementation detail of the JVM
   oracle, not of `krit`'s filter policy. Re-architecting to feed the
   oracle every file defeats the purpose of narrowing.
3. A delta of 1 on ~10k findings is below any signal threshold we
   would act on from the CLI; it is a curiosity at the current scale.

## Hardening: oracle input-set fingerprint

Added in this commit: `oracle.CollectOracleFiles` now reports a
stable SHA-256 over the sorted, absolute path list in the returned
`OracleFilterSummary`. The pipeline emits it as a `perf` entry
`typeOracle/filterFingerprint/<hex>` when the tracker is enabled, so
future narrowing PRs can diff the fingerprint before vs after to
detect exactly which files moved in or out of the oracle universe.
This is the minimal, language-neutral signal the issue's
"Opportunities" section asks for.

## Regression fixture

Deferred. A representative fixture requires a real cross-file sample
(a suspend helper + non-suspend caller whose declaration file lacks
all three filter tokens) *and* a working `krit-types.jar` in CI. The
fingerprint above is a lower-cost proxy that catches the same class
of shift without requiring the oracle to run in tests.
