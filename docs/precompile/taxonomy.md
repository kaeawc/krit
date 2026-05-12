# Pre-Compile Diagnostic Taxonomy

Stable catalog of compiler-class diagnostics emitted under `Category: "precompile"`.

Each diagnostic is identified by a plain rule name (PascalCase, matching every
other Krit rule). Rules in this category carry structured metadata on
`api.Rule` so tooling, dashboards, and CLI filters can group them without
parsing the ID:

- `Level` (`api.RuleLevel`) — analytical scope: `function`, `file`, `module`,
  `external`, `generated`, `meta`.
- `KotlincAnalog` (`string`) — informational name of the closest standard
  kotlinc diagnostic. Krit is not bug-for-bug compatible with kotlinc.

Renames go through the normal Krit deprecation path: add the new rule, mark
the old one with `Maturity: MaturityDeprecated`, keep both around until the
deprecation window closes.

## Severity

Every non-meta rule in this catalog has `Sev: api.SeverityError`. Style-class
rules use `warning` or `info` and stay outside this taxonomy. Meta diagnostics
(`Level: LevelMeta`) may use `Sev: api.SeverityWarning` for infrastructure
signals. The `precompile` profile floor is enforced by
`internal/rules/precompile_conventions_test.go`.

## Default activation

Precompile rules ship with `DefaultActive: false`. Users opt in via:

```yaml
# krit.yml
profile: precompile
```

...or by enabling the `precompile` category explicitly. The profile activation
plumbing lands in Phase 1 alongside the first runnable rule.

## Diagnostics

Columns: `name`, `level`, `needs`, `kotlinc analog`, `phase`.

`needs` lists the krit `Capabilities` bits the rule consumes. `kotlinc analog`
names the closest standard kotlinc diagnostic (informational only; krit is
not bug-for-bug compatible).

### Function level

| Name | Needs | kotlinc analog | Phase |
|---|---|---|---|
| UnreachableCode | - | UNREACHABLE_CODE | 1 |
| NonExhaustiveWhen | NeedsResolver | NO_ELSE_IN_WHEN | 1 |
| UselessElvisOnNonNull | NeedsResolver | USELESS_ELVIS | 1 |
| SmartCastImpossible | NeedsResolver | SMARTCAST_IMPOSSIBLE | 1 |
| TypeMismatchInReturn | NeedsResolver | TYPE_MISMATCH | 1 |
| UnusedParameter | - | UNUSED_PARAMETER | 1 |
| InvalidConstExpression | NeedsResolver | CONST_VAL_NOT_TOP_LEVEL_OR_OBJECT | 1 |
| DuplicateLabel | - | DUPLICATE_LABEL_IN_WHEN | 1 |
| ConditionAlwaysTrue | NeedsResolver | SENSELESS_COMPARISON | 1 |
| ImplicitNothingReturn | NeedsResolver | IMPLICIT_NOTHING_AS_TYPE_PARAMETER | 1 |

### File level

| Name | Needs | kotlinc analog | Phase |
|---|---|---|---|
| UnresolvedReference | NeedsResolver | UNRESOLVED_REFERENCE | 2 |
| OverrideSignatureMismatch | NeedsResolver | RETURN_TYPE_MISMATCH_ON_OVERRIDE | 2 |
| AbstractMemberNotImplemented | NeedsResolver | ABSTRACT_MEMBER_NOT_IMPLEMENTED | 2 |
| ReturnTypeMismatchOverride | NeedsResolver | RETURN_TYPE_MISMATCH_ON_OVERRIDE | 2 |
| VisibilityViolation | NeedsResolver | INVISIBLE_REFERENCE | 2 |
| DuplicateDeclaration | NeedsResolver | CONFLICTING_OVERLOADS | 2 |
| InvalidImport | NeedsResolver | UNRESOLVED_IMPORT | 2 |
| ConflictingOverloads | NeedsResolver | CONFLICTING_OVERLOADS | 2 |
| SealedSubclassOutsideHierarchy | NeedsResolver | SEALED_INHERITOR_IN_DIFFERENT_FILE | 2 |
| InitializerTypeMismatch | NeedsResolver | INITIALIZER_TYPE_MISMATCH | 2 |

### Module level

Cross-file, source-only. Use `NeedsCrossFile` and the `scanner.CodeIndex`.

| Name | Needs | kotlinc analog | Phase |
|---|---|---|---|
| UnresolvedReferenceCrossFile | NeedsResolver \| NeedsCrossFile | UNRESOLVED_REFERENCE | 3 |
| OverrideSignatureMismatchCrossFile | NeedsResolver \| NeedsCrossFile | RETURN_TYPE_MISMATCH_ON_OVERRIDE | 3 |
| AbstractMemberNotImplementedCrossFile | NeedsResolver \| NeedsCrossFile | ABSTRACT_MEMBER_NOT_IMPLEMENTED | 3 |
| InternalLeakCrossFile | NeedsResolver \| NeedsCrossFile | INVISIBLE_REFERENCE | 3 |
| NonExhaustiveWhenSealedCrossFile | NeedsResolver \| NeedsCrossFile | NO_ELSE_IN_WHEN | 3 |
| TypeAliasResolutionFailure | NeedsResolver \| NeedsCrossFile | UNRESOLVED_REFERENCE | 3 |
| PrivateExposedInPublicApi | NeedsResolver \| NeedsCrossFile | EXPOSED_PROPERTY_TYPE | 3 |

### External level

Backed by the JVM Oracle daemon. Rules declare narrow `NeedsOracle*` bits to
keep the extraction profile tight.

| Name | Needs | kotlinc analog | Phase |
|---|---|---|---|
| UnresolvedReferenceExternal | NeedsOracleCallTargets \| NeedsOracleLibraryClasses | UNRESOLVED_REFERENCE | 4 |
| OverrideSignatureMismatchExternal | NeedsOracleSupertypes \| NeedsOracleMemberSignatures | RETURN_TYPE_MISMATCH_ON_OVERRIDE | 4 |
| DeprecatedSymbolUsedError | NeedsOracleMemberAnnotations | DEPRECATION_ERROR | 4 |
| RequiresApiViolation | NeedsOracleMemberAnnotations \| NeedsGradle | NewApi (Android lint analog) | 4 |
| RemovedOverloadCalled | NeedsOracleCallTargets | NONE_APPLICABLE | 4 |
| InvalidNullabilityFromJava | NeedsOracleExprType \| NeedsOracleMemberAnnotations | NULL_FOR_NONNULL_TYPE | 4 |

### Generated level

| Name | Needs | kotlinc analog | Phase |
|---|---|---|---|
| GeneratedSourceStale | NeedsCrossFile | (no direct analog; pre-compile heuristic) | 5 |

### Meta level

| Name | Severity | When |
|---|---|---|
| OracleBudgetExceeded | warning | Oracle calls per file exceeded soft cap (200). |
| OracleUnavailable | warning | Oracle daemon could not be reached; external-resolution rules skipped. |

Meta diagnostics intentionally use `warning` (not `error`): they are
infrastructure signals, not user defects. They are exempt from the profile
severity floor; the convention test allows `Sev: SeverityWarning` only for
rules with `Level: LevelMeta`.

## Coverage targets

- **Function level:** >=10 (achieved: 10).
- **File level:** >=10 (achieved: 10).
- **Module level:** >=7 (achieved: 7).
- **Project / external level:** >=3 (achieved: 7 across external + generated).
- **Total:** >=30 (achieved: 36).

## Adding a new diagnostic

1. Add a row to the matching level table above.
2. Implement `internal/rules/precompile_<name>.go` + `registry_precompile_<name>.go`.
   Set `Level` and `KotlincAnalog` on the `api.Rule` literal.
3. Add fixtures: `tests/fixtures/precompile/{positive,negative,fixable}/<RuleID>/*.kt`
   plus `*.diag` companions.
4. Wire registration from `registry_bootstrap.go`.
5. `make lint-rules && make test && make integration` must stay green.
