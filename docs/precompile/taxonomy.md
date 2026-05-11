# Pre-Compile Diagnostic Taxonomy

Stable catalog of compiler-class diagnostics emitted under `Category: "precompile"`.

Each diagnostic is identified by a stable code embedded in the rule ID (`K####-Name`). The code is the public contract: agents and tooling match on the `K####` prefix; humans read the trailing name. Codes never change once shipped: superseded rules get a new code and the old code is retired with `Maturity: MaturityDeprecated`.

## ID format

```
^K\d{4}-[A-Z][A-Za-z0-9]+$
```

- `K0001`-`K0099`: reserved (cross-cutting)
- `K0100`-`K0199`: function scope
- `K0200`-`K0299`: file scope
- `K0300`-`K0399`: module scope (cross-file, source-only)
- `K0400`-`K0499`: external / binary resolution (Oracle-backed)
- `K0500`-`K0599`: generated sources / build-system surface
- `K9000`-`K9999`: meta diagnostics (budget exceeded, oracle unavailable, etc.)

## Severity

Every non-meta rule in this catalog has `Sev: api.SeverityError`. Style-class rules use `warning` or `info` and stay outside this taxonomy. Meta diagnostics in the `K9###` range may use `Sev: api.SeverityWarning` for infrastructure signals. The `precompile` profile floor is enforced by `internal/rules/precompile_conventions_test.go`.

## Default activation

Precompile rules ship with `DefaultActive: false`. Users opt in via:

```yaml
# krit.yml
profile: precompile
```

...or by enabling the `precompile` category explicitly. The profile activation plumbing lands in Phase 1 alongside the first runnable rule.

## Diagnostics

Columns: `code`, `name`, `scope`, `needs`, `kotlinc analog`, `phase`.

`needs` lists the krit `Capabilities` bits the rule consumes. `kotlinc analog` names the closest standard kotlinc diagnostic (informational only; krit is not bug-for-bug compatible).

### Function scope (K0100-K0199)

| Code | Name | Needs | kotlinc analog | Phase |
|---|---|---|---|---|
| K0101 | UnreachableCode | - | UNREACHABLE_CODE | 1 |
| K0102 | NonExhaustiveWhen | NeedsResolver | NO_ELSE_IN_WHEN | 1 |
| K0103 | UselessElvisOnNonNull | NeedsResolver | USELESS_ELVIS | 1 |
| K0104 | SmartCastImpossible | NeedsResolver | SMARTCAST_IMPOSSIBLE | 1 |
| K0105 | TypeMismatchInReturn | NeedsResolver | TYPE_MISMATCH | 1 |
| K0106 | UnusedParameter | - | UNUSED_PARAMETER | 1 |
| K0107 | InvalidConstExpression | NeedsResolver | CONST_VAL_NOT_TOP_LEVEL_OR_OBJECT | 1 |
| K0108 | DuplicateLabel | - | DUPLICATE_LABEL_IN_WHEN | 1 |
| K0109 | ConditionAlwaysTrue | NeedsResolver | SENSELESS_COMPARISON | 1 |
| K0110 | ImplicitNothingReturn | NeedsResolver | IMPLICIT_NOTHING_AS_TYPE_PARAMETER | 1 |

### File scope (K0200-K0299)

| Code | Name | Needs | kotlinc analog | Phase |
|---|---|---|---|---|
| K0201 | UnresolvedReference | NeedsResolver | UNRESOLVED_REFERENCE | 2 |
| K0202 | OverrideSignatureMismatch | NeedsResolver | RETURN_TYPE_MISMATCH_ON_OVERRIDE | 2 |
| K0203 | AbstractMemberNotImplemented | NeedsResolver | ABSTRACT_MEMBER_NOT_IMPLEMENTED | 2 |
| K0204 | ReturnTypeMismatchOverride | NeedsResolver | RETURN_TYPE_MISMATCH_ON_OVERRIDE | 2 |
| K0205 | VisibilityViolation | NeedsResolver | INVISIBLE_REFERENCE | 2 |
| K0206 | DuplicateDeclaration | NeedsResolver | CONFLICTING_OVERLOADS | 2 |
| K0207 | InvalidImport | NeedsResolver | UNRESOLVED_IMPORT | 2 |
| K0208 | ConflictingOverloads | NeedsResolver | CONFLICTING_OVERLOADS | 2 |
| K0209 | SealedSubclassOutsideHierarchy | NeedsResolver | SEALED_INHERITOR_IN_DIFFERENT_FILE | 2 |
| K0210 | InitializerTypeMismatch | NeedsResolver | INITIALIZER_TYPE_MISMATCH | 2 |

### Module scope (K0300-K0399)

Cross-file, source-only. Use `NeedsCrossFile` and the `scanner.CodeIndex`.

| Code | Name | Needs | kotlinc analog | Phase |
|---|---|---|---|---|
| K0301 | UnresolvedReferenceCrossFile | NeedsResolver \| NeedsCrossFile | UNRESOLVED_REFERENCE | 3 |
| K0302 | OverrideSignatureMismatchCrossFile | NeedsResolver \| NeedsCrossFile | RETURN_TYPE_MISMATCH_ON_OVERRIDE | 3 |
| K0303 | AbstractMemberNotImplementedCrossFile | NeedsResolver \| NeedsCrossFile | ABSTRACT_MEMBER_NOT_IMPLEMENTED | 3 |
| K0304 | InternalLeakCrossFile | NeedsResolver \| NeedsCrossFile | INVISIBLE_REFERENCE | 3 |
| K0305 | NonExhaustiveWhenSealedCrossFile | NeedsResolver \| NeedsCrossFile | NO_ELSE_IN_WHEN | 3 |
| K0306 | TypeAliasResolutionFailure | NeedsResolver \| NeedsCrossFile | UNRESOLVED_REFERENCE | 3 |
| K0307 | PrivateExposedInPublicApi | NeedsResolver \| NeedsCrossFile | EXPOSED_PROPERTY_TYPE | 3 |

### External / binary resolution (K0400-K0499)

Backed by the JVM Oracle daemon. Rules declare narrow `NeedsOracle*` bits to keep the extraction profile tight.

| Code | Name | Needs | kotlinc analog | Phase |
|---|---|---|---|---|
| K0401 | UnresolvedReferenceExternal | NeedsOracleCallTargets \| NeedsOracleLibraryClasses | UNRESOLVED_REFERENCE | 4 |
| K0402 | OverrideSignatureMismatchExternal | NeedsOracleSupertypes \| NeedsOracleMemberSignatures | RETURN_TYPE_MISMATCH_ON_OVERRIDE | 4 |
| K0403 | DeprecatedSymbolUsedError | NeedsOracleMemberAnnotations | DEPRECATION_ERROR | 4 |
| K0404 | RequiresApiViolation | NeedsOracleMemberAnnotations \| NeedsGradle | NewApi (Android lint analog) | 4 |
| K0405 | RemovedOverloadCalled | NeedsOracleCallTargets | NONE_APPLICABLE | 4 |
| K0406 | InvalidNullabilityFromJava | NeedsOracleExprType \| NeedsOracleMemberAnnotations | NULL_FOR_NONNULL_TYPE | 4 |

### Generated sources (K0500-K0599)

| Code | Name | Needs | kotlinc analog | Phase |
|---|---|---|---|---|
| K0501 | GeneratedSourceStale | NeedsCrossFile | (no direct analog; pre-compile heuristic) | 5 |

### Meta diagnostics (K9000-K9999)

| Code | Name | Severity | When |
|---|---|---|---|
| K9001 | OracleBudgetExceeded | warning | Oracle calls per file exceeded soft cap (200). |
| K9002 | OracleUnavailable | warning | Oracle daemon could not be reached; external-resolution rules skipped. |

Meta diagnostics intentionally use `warning` (not `error`): they are infrastructure signals, not user defects. They are exempt from the profile severity floor; the convention test allows `Sev: SeverityWarning` only for IDs in the `K9###` range.

## Coverage targets

- **Function scope:** >=10 (achieved: 10).
- **File scope:** >=10 (achieved: 10).
- **Module scope:** >=7 (achieved: 7).
- **Project / external scope:** >=3 (achieved: 7 across K0400 + K0500).
- **Total:** >=30 (achieved: 36).

## Adding a new diagnostic

1. Pick the lowest unused code in the right band.
2. Add a row here.
3. Implement `internal/rules/precompile_<name>.go` + `registry_precompile_<name>.go`.
4. Add fixtures: `tests/fixtures/precompile/{positive,negative,fixable}/<RuleID>/*.kt` plus `*.diag` companions.
5. Wire registration from `registry_bootstrap.go`.
6. `make lint-rules && make test && make integration` must stay green.
