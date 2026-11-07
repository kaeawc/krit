# Migrating from detekt

Krit reads an existing detekt configuration and applies it under `krit.yml`, so
a project that already tunes or disables rules in `detekt.yml` gets the same
thresholds and switches without rewriting its config.

## How the import works

Settings are layered, lowest to highest priority:

1. krit's shipped defaults
2. detekt's own default config (only when a detekt config is imported)
3. the imported detekt config
4. `krit.yml`

A value in `krit.yml` always wins over the same setting in `detekt.yml`.

Layer 2 is detekt 1.23.8's `default-detekt-config.yml` (embedded as
`config/detekt-defaults.yml`). It makes a rule or option your `detekt.yml` leaves
out behave as it does in detekt, including `active`, even where krit's shipped
default is deliberately different (for example `ReturnCount` allows 3 returns and
skips guard clauses in krit, but 2 and no guard-clause exemption in detekt). Without
a detekt config, krit's own defaults apply. List entries written as
`{value: ..., reason: ...}` are read as their `value`.

### Discovery

Krit looks for these files, in order, in the scanned directory and each parent
up to the repository root (it never goes past `.git`, `.hg`, `.jj`, or your home
directory):

1. `config/detekt/detekt.yml`
2. `detekt.yml`
3. `.detekt.yml`

To pick a file yourself:

```yaml
# krit.yml
detekt:
  configFile: tools/lint/detekt.yml   # relative to this krit.yml
  baseline: tools/lint/baseline.xml   # optional, see "Baseline"
```

or pass `--detekt-config path/to/detekt.yml` on the command line. An explicit
file that does not exist is an error.

### Opting out

```yaml
# krit.yml
detekt:
  enabled: false
```

or run with `--no-detekt-config`. `--detekt-config` wins over
`detekt.enabled: false` when both are present.

Run with `-v` to see which file was imported and which detekt keys were skipped.

## What is mapped

| detekt | krit |
| --- | --- |
| `<ruleset>: active` | same key; disables every rule in the set |
| `<ruleset>: excludes` / `includes` | added to each rule's own globs |
| `<Rule>: active` | same key |
| `<Rule>: excludes` | same key (file globs, e.g. `**/test/**`) |
| `<Rule>: includes` | the rule runs only on files matching one of the globs |
| `<Rule>: ignoreAnnotated` | declarations annotated with a listed annotation are exempt from the rule (simple names, `@` optional) |
| other rule options | same name when the krit rule has that option or an alias for it; otherwise skipped |

Rulesets that krit has no equivalent for (`build`, `config`, `processors`,
`console-reports`, `output-reports`, `formatting`, ...) are skipped.

### Renamed rules

| detekt rule | krit rule |
| --- | --- |
| `ComplexMethod` | `CyclomaticComplexMethod` |
| `FunctionName` | `FunctionNaming` |
| `ClassName` | `ClassNaming` |
| `VariableName` | `VariableNaming` |
| `PackageName` | `PackageNaming` |
| `FunctionMaxLength` | `FunctionNameMaxLength` |
| `FunctionMinLength` | `FunctionNameMinLength` |
| `PackageDirectoryMismatch` | `InvalidPackageDeclaration` |
| `MandatoryBracesIfStatements` | `BracesOnIfStatements` |
| `EmptyKtFile` | `EmptyKotlinFile` |
| `CommentOverPrivateFunction` | `DocumentationOverPrivateFunction` |
| `CommentOverPrivateProperty` | `DocumentationOverPrivateProperty` |
| `UnnecessaryAbstractClass` | `AbstractClassCanBeConcreteClass` and `AbstractClassCanBeInterface` |
| `MayBeConst` | `MayBeConstant` |
| `UnusedImports` | `UnusedImport` |
| `SpacingBetweenPackageAndImports` | `SpacingAfterPackageAndImports` |
| `RedundantVisibilityModifierRule` | `RedundantVisibilityModifier` |
| `OptionalWhenBraces` | `BracesOnWhenStatements` |
| `UnusedPrivateMember` | `UnusedPrivateFunction` (krit splits detekt's rule per member kind) |

If a file configures both the old and the new name, the new name wins. A rule
that krit files under a different ruleset than detekt does is moved to the
ruleset krit uses.

### Thresholds

detekt 1.x thresholds fire when the measured value reaches the number
(`threshold: 120` reports a method of 120 lines). Krit's options name the
largest allowed value and fire above it. The importer subtracts one so the two
agree:

| detekt option | krit option | imported value |
| --- | --- | --- |
| `LongMethod: threshold` | `allowedLines` | threshold - 1 |
| `LargeClass: threshold` | `allowedLines` | threshold - 1 |
| `NestedBlockDepth: threshold` | `allowedDepth` | threshold - 1 |
| `ComplexMethod` / `CyclomaticComplexMethod: threshold` | `allowedComplexity` | threshold - 1 |
| `CognitiveComplexMethod: threshold` | `allowedComplexity` | threshold - 1 |
| `ComplexCondition: threshold` | `allowedConditions` | threshold - 1 |
| `ComplexInterface: threshold` | `allowedDefinitions` | threshold - 1 |
| `CouldBeSequence: threshold` | `allowedOperations` | threshold - 1 |
| `MethodOverloading: threshold` | `allowedOverloads` | threshold - 1 |
| `StringLiteralDuplication: threshold` | `allowedDuplications` | threshold - 1 |
| `LongParameterList: functionThreshold` | `allowedFunctionParameters` | threshold - 1 |
| `LongParameterList: constructorThreshold` | `allowedConstructorParameters` | threshold - 1 |
| `TooManyFunctions: thresholdInFiles` | `allowedFunctionsPerFile` | threshold - 1 |
| `TooManyFunctions: thresholdInClasses` | `allowedFunctionsPerClass` | threshold - 1 |
| `TooManyFunctions: thresholdInInterfaces` | `allowedFunctionsPerInterface` | threshold - 1 |
| `TooManyFunctions: thresholdInObjects` | `allowedFunctionsPerObject` | threshold - 1 |
| `TooManyFunctions: thresholdInEnums` | `allowedFunctionsPerEnum` | threshold - 1 |

`NestedScopeFunctions: threshold` and `NamedArguments: threshold` fire above the
number in detekt too, so they are copied unchanged.

Options that already state a maximum (`MaxLineLength: maxLineLength`,
`ReturnCount: max`, `ThrowsCount: max`,
`DestructuringDeclarationWithTooManyEntries: maxDestructuringEntries`,
`allowed*` names from detekt 2.x) are copied unchanged. Pattern and list
options such as `TopLevelPropertyNaming: constantPattern`,
`MagicNumber: ignoreNumbers`, and `ReturnCount: excludeGuardClauses` are copied
unchanged too.

### Example

```yaml
# detekt.yml
complexity:
  LongMethod:
    threshold: 120
  NestedBlockDepth:
    threshold: 6
  TooManyFunctions:
    thresholdInFiles: 40
    thresholdInClasses: 40
  LongParameterList:
    functionThreshold: 8
    constructorThreshold: 10
    ignoreAnnotated: ['Composable']
style:
  MaxLineLength:
    active: false
  ReturnCount:
    max: 3
    excludeGuardClauses: true
```

With this file, an 80-line function is no longer reported by `LongMethod`
(krit's default flags anything over 60 lines), `MaxLineLength` does not run,
and `@Composable` functions are exempt from `LongParameterList`.

### Aliases in `krit.yml`

Rule options that have an alias (for example `threshold` for `allowedLines`)
are rewritten to the primary name before layers merge, so an alias in
`krit.yml` or `detekt.yml` is not shadowed by krit's shipped default for the
primary name. In `krit.yml` an alias keeps krit's meaning: the value is the
largest allowed value, with no subtraction.

## Baseline

If `baseline.xml` sits next to the imported detekt config (the usual
`config/detekt/baseline.xml`), or `detekt.baseline` names a file, krit applies
it when you do not pass `--baseline`. `--create-baseline`, `--baseline-audit`,
and `--delta` do not use it.

detekt baseline entries look like `Rule:File.kt$Class$fun name()` and carry no
line numbers. Krit matches each entry to one finding of that rule (or the krit
rule that replaced it) in a file with that name, so a baseline silences as many
findings per file as it has entries. Entries in krit's own baseline format keep
their exact matching.

## Suppressions

`@Suppress("detekt.RuleName")`, `@Suppress("RuleName")`, and the detekt names
above (every name in the rename table) keep working in source.

### Style rule parity

The `style` rules were compared with detekt 1.23.8 on synthetic samples and
aligned on conditions, messages, anchors and defaults. Intentional differences:

- Krit-only options default to on in krit and off in the detekt-defaults layer
  (`ignoreIdioms` on FunctionOnlyReturningConstant, ForbiddenVoid and
  MagicNumber, `ignoreUnwrappableLines`, `reportMutableCollections`), so an
  imported detekt config behaves like detekt.
- SerialVersionUIDInSerializableClass does not report enum classes; detekt does.
  Like detekt, it matches only a declared supertype entry written exactly
  `Serializable`.
- UnnecessaryFilter, UseOrEmpty and other rules that detekt runs only with type
  resolution keep krit's type-aware behaviour when a resolver is available.
- `UntilInsteadOfRangeTo` is implemented as a new rule.

## Differences to check

- Rule behavior is not identical in every edge case. Spot-check a few files.
- Rules detekt has and krit lacks are reported under `-v` and skipped.
- Only the importer-mapped renames above are translated; a different detekt
  version may name a rule or option differently.
- `ignoreAnnotated` and `includes` are applied to every rule, including those
  where detekt itself ignores them.
- The daemon applies auto-discovered detekt config but not `--detekt-config` or
  `--no-detekt-config`; those flags run the scan in-process.

### Known deliberate differences from detekt 1.23.8

Checked against a detekt 1.23.8 run on synthetic sources. Everything else in the
potential-bugs, coroutines, empty-blocks and `UseIfInsteadOfWhen` rules reports the same lines.

- Rules ported from detekt analyze Kotlin sources only (`.kt`, `.kts`), as detekt does; they skip `.java` files. Rules that mirror Android Lint, which does analyze Java, keep their Java support.
- `PropertyUsedBeforeDeclaration` reports a read during construction (property initializers, `init`
  blocks, anonymous objects), not a read in a function body or a lambda, which detekt also reports.
- `MapGetWithNotNullAssertionOperator` also covers `MutableMap`, `HashMap` and the other map types (detekt only
  sees `kotlin.collections.Map.get`), and skips a `get` guarded by `containsKey`.
- `SuspendFunWithFlowReturnType` skips overrides, `open` and `abstract` members, interface members, and
  functions that make a suspend call before the flow is built. detekt reports them.
- `SuspendFunSwallowedCancellation` reports `runCatching` around a suspend call like detekt, and also a
  `catch` of `Exception` or `Throwable` that does not rethrow `CancellationException`.
- `RedundantSuspendModifier` keeps the modifier when the project declares a `suspend` function of the same name,
  and does not decide calls on a receiver it cannot resolve.
- `ExplicitGarbageCollectionCall` also reports the fully qualified `java.lang.System.gc()`.
- `EmptyKotlinFile` also reports a file that holds only a package line and imports.
- `UnusedPrivateMember` leaves `@Preview` functions alone.
- `UnsafeCast` and `Deprecation` follow the Kotlin compiler krit runs (K2), so the few shapes the K1 compiler
  behind detekt judges differently, and deprecations of the JDK in use, differ.
- `DuplicateCaseInWhenExpression` is covered by the opt-in `DuplicateLabel` for repeated literal constants
  only (not enum entries, `is` tests, ranges or subject-less conditions); `MissingWhenCase` by `NoElseInWhenSealed` and `NonExhaustiveWhen`; `RedundantElseInWhen` by
  `ElseCaseInsteadOfExhaustiveWhen`.
