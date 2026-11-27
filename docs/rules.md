# Rules

Krit ships rules spanning Kotlin and Java source, style, complexity, null safety,
performance, Android manifests, resources, icons, and Gradle. Many are
detekt-compatible (same names and config keys) or Android Lint-compatible. Many
are auto-fixable.

## LogConditional Semantics

The opt-in Go and FIR checkers report only `android.util.Log.v/d/i/println` when the message
performs work; `w/e/wtf` are silent. FIR follows Android Lint 32.4.1 message semantics. Go uses a
deliberately narrower structural rule: after unwrapping parentheses, the message itself must be a
`call_expression`, or be a string template or `+` concatenation with a direct entry/operand that
unwraps to a `call_expression`. Elvis messages also report when the left operand, after unwrapping
parentheses, is a call or a navigation whose final selector is a call. For example,
`"x" + compute()`, `"${user.name()}"`, `obj.toString()`, and
`prefs.getString("key", null) ?: ""` report. Calls on the right side of Elvis
or nested below another expression do not count: property reads such as
`getPackageInfo(...).versionName`, other Elvis/conditional expressions, and `${compute().length}`
are silent in Go. It does not resolve message bindings, property declarations, or constness. For a
`Log.x` receiver, Go reports only when every `Log` identifier token in the file is part of the
canonical `import android.util.Log` header or is the receiver of a `Log.<name>` navigation. This
purely lexical, file-wide check ignores comments and literal string content, but counts identifiers
inside string-template interpolations. It does not distinguish declaration kinds, scopes, or order;
any other `Log` occurrence makes the whole file silent, including aliases and qualified non-Android
receivers. Because it only sees one file's tokens, it can report a call shadowed by an implicit
receiver member declared in another file (for example, a base-class `Log` or a `Log` member visible
inside a scope function). FIR continues to use resolved call targets.

FIR follows Android Lint 32.4.1's resolved `LogDetector` message semantics: non-constant
concatenations/templates, calls, Kotlin qualified property accesses, and other expressions
perform work. Literals, simple references, and constant concatenations/templates are silent.
For example, with a non-const `val name`, `"load $name"` reports in FIR but is silent in Go;
resolved const references in templates/concatenations stay silent in FIR.
`println` uses argument index 2; the other methods use index 1. Go also checks positional values
inside named argument wrappers, but Kotlin rejects named arguments to the real Java `Log` methods;
the receiver-shape compile test records that boundary rather than using invalid code as FIR proof.

The nearest `if` decides guarding, regardless of flag spelling; only literal `true` does not guard.
A `when` entry guards if any of its case values is not literal `true`; `else` does not guard.
Enclosing source calls, classes/objects, member/top-level functions and properties, constructors,
accessors, and initializers stop the search. Local functions, standalone lambdas/anonymous functions,
declarations local to init blocks and secondary constructors, loops, and try/catch continue it.
A log used as a receiver, such as `if (flag) Log.d("T", compute()).also { }`, is guarded:
the selector call is not an enclosing call. A call taking that expression as an argument still
stops the search. Thus `if (flag) { run { Log.d("T", compute()) } }` reports,
but `if (flag) { val callback = { Log.d("T", compute()) } }` does not.

All Go verdicts are pinned by `// go-lines:` headers in the `LogConditional*.kt` goldens. Remaining
Go differences are documented inline: alias/static/star-import resolution and the conservative
file-wide lexical check for `Log` can cause missed log calls, while unavailable cross-file implicit
receiver facts can cause false positives; call-free
non-constant templates/concatenations, qualified Kotlin properties and Java getters, Elvis
expressions without a direct call on the left, not-null assertions, and conditional expressions
that FIR treats as work are silent in Go. Calls nested in those latter expressions are also silent
unless they are the message itself, a direct template/concatenation entry, or a direct call on the
left side of Elvis. FIR treats Java field reads
such as `getPackageInfo(...).versionName` as no work; Go also
stays silent because the outer message is not a call. FIR/Lint can fold literal local `val`s in
concatenations, but not templates; Go does not resolve constness in either case.
The historical corpus row in `docs/fir-validation.md` predates this semantic change and is not a fresh qualification.

Follow-ups, not implemented here: Java-source coverage and the scanner's `/benchmark/` test-path
substring classification. The existing Go-provided test-file skip remains unchanged in FIR.

## Categories

| Category           | What it catches |
|--------------------|-----------------|
| Style              | Magic numbers, wildcard imports, unused code, redundant syntax |
| Naming             | Class, function, variable, enum, package naming |
| Complexity         | Cyclomatic/cognitive complexity, long methods, nested depth |
| Potential Bugs     | Unsafe casts, null safety, unreachable code, swallowed exceptions |
| Empty Blocks       | Empty catch/if/else/for/while/when/try/finally/init |
| Exceptions         | Too-generic catches, printStackTrace, return from finally |
| Coroutines         | Hardcoded dispatchers, redundant suspend, GlobalScope |
| Performance        | Array primitives, spread operator, unnecessary instantiation |
| Dead Code          | Cross-file unused public API |
| Libraries          | Library-specific patterns |
| Android Manifest   | Security, permissions, exported components, backup config |
| Android Resources  | Missing translations, unused resources, density checks |
| Android Usability  | Accessibility, typography, button styles, text fields |
| Android Icons      | Launcher icon density, WebP conversion, adaptive icons |
| Android Gradle     | Dependency management, SDK versions, build configuration |
| Android Source     | Android-specific Kotlin/Java code patterns |

`krit --list-rules` prints the full list with IDs and default thresholds.
`krit --list-rules --format json` prints it as JSON (`{"rules": [...]}`, one
object per rule with `id`, `category`, `severity`, `defaultActive`,
`fixable`, `fixLevel`, `precision`, `maturity`, `description`, and
`docsUrl`) for editor
integrations and scripts; `--cwe` and `--maturity` filter it the same way.

## Rule reference

Every rule has a page with its options, defaults, detekt/Android Lint
mapping, and examples: see the [rule reference](rules/categories.md). Pages
live at `https://kaeawc.github.io/krit/rules/<category>/<rule-id-kebab>/`
(for example `rules/style/magic-number/`) and are generated by
`go run ./cmd/krit-docgen` (run it before `mkdocs build`; the docs workflow
does). The same URL is the `docsUrl` field of `krit --list-rules --format json`
and the SARIF `helpUri`.

## Third-party Lint checks

The `slack-lint` rule set ports checks from [slack-lints](https://github.com/slackhq/slack-lints) 0.11.1. Each rule
uses its Lint issue ID, message and severity, so `@SuppressLint("MoshiUsageEnumCasing")`, `tools:ignore` and
`lint.xml` entries for the Lint ID apply to the krit rule. The rules are experimental and off by default; turn them
on by ID (`--enable-rules MoshiUsageEnumCasing`) or with `--experimental`.

The `MoshiUsage*` rules port `MoshiUsageDetector`. As in Lint, one pass over the project serves all of them, and
they need Kotlin or Java sources that import Moshi (`com.squareup.moshi`) or moshix (`dev.zacsweers.moshix`).
They report only what the source proves:

- an annotation counts only when its import, full name, wildcard import or package proves it is Moshi's;
- an annotation argument that is not a literal (a `const val` reference, an escaped string) makes the check
  abstain. Lint evaluates constants and so reports, for example, `MoshiUsageEnumMissingUnknown` on an enum whose
  `generator` is a constant `""`. On a non-literal `generateAdapter` Lint itself fails with an internal error.

Not ported: `MoshiUsageNonMoshiClassInternal` and `MoshiUsageNonMoshiClassExternal`, which split on whether a type
lives in Slack's own `slack.*` packages.

Three checks read annotations:

- `TestParameterSiteTarget` reports `@TestParameter` on a Kotlin constructor property (`val`/`var`) without the
  `param:` site target. As in Lint, any annotation named `TestParameter` counts.
- `MustUseNamedParams` reports a Kotlin call with a positional argument to a Kotlin function or constructor
  annotated `@slack.lint.annotations.MustUseNamedParams`.
- `RestrictCallsTo` reports a Kotlin call, from another file, to a function annotated
  `@slack.lint.annotations.RestrictCallsTo`, including calls through an override.

Both call checks match only what the source shows: a top-level function through its import, package or a star
import; a constructor through its class; a member through the receiver's resolved or declared type (walking project
supertypes as each declaring file names them), or through the enclosing class for an unqualified call. A local
function or a member of an enclosing class (or of its project supertypes) with the same name wins over a top-level
function, as in Kotlin. When the annotated declaration has same-named overloads (in its file, other files of its package,
or classes between the receiver's type and the owner, including overrides for `MustUseNamedParams`), or its class
has other constructors, the call must fit its argument count and no overload's; otherwise krit abstains. Annotated
declarations in libraries are not seen, and as in Lint 0.11.1, Java call sites are not checked. Known false
negatives: a call through an import alias (`import com.example.f as g`), an unqualified member call from a nested,
inner or anonymous class inside a subclass, and annotated extension functions.

`JavaOnlyDetector` ports `JavaOnlyDetector` for slack-lint-annotations' `@JavaOnly` and `@KotlinOnly`. As in Lint,
it checks Kotlin sources only (Lint leaves Java to an ErrorProne check) and reads Java declarations for them. It
reports a class or function annotated with both; a class whose direct superclass or interface, or a function whose
overridden function, is annotated but which does not repeat the annotation (`@KotlinOnly` is checked before
`@JavaOnly`); an object expression or a SAM-constructor lambda of a `@JavaOnly` Java type, unless returned from a
`@JavaOnly` function; and a call or callable reference to a `@JavaOnly` function or to any member of a `@JavaOnly`
class, with the annotation's `reason` in the message. Calls are matched like `MustUseNamedParams` above. A finding on
a class sits at the declaration, where Lint starts it at the KDoc above. Not checked: annotated library
declarations, a lambda passed where a `@JavaOnly` interface is expected without its SAM constructor, and Lint's
package-level `@KotlinOnly` lookup.

The R class import-alias checks (Kotlin only, as import aliases are a Kotlin feature) read the `importAliases` option,
Lint's `import-aliases`: entries such as `slack.l10n.R as L10nR`.

```yaml
slack-lint:
  FullyQualifiedResource:
    active: true
    importAliases: ["slack.l10n.R as L10nR"]
```

- `FullyQualifiedResource` reports a fully qualified R class (`slack.l10n.R.string.app_name`) other than `android.R`,
  naming the configured alias when there is one. Every segment before `R` must be a package segment (lower-case), so
  unlike Lint `Outer.R.x` (a nested `object R`) and `Build.VERSION_CODES.R` are not reported.
- `MissingResourceImportAlias` reports `import other.module.R` without an alias. Lint treats an import from the
  module's own package as local; krit does not read the module's namespace and uses Lint's fallback, so an import
  whose package is a prefix of the file's package is local. This differs from Lint in both directions: the module's
  own R is reported when its namespace is not a prefix of the file's package (`namespace = "slack.features.home"`
  with `package com.app.home` and `import slack.features.home.R`), and another module's R is not reported when its
  package happens to be such a prefix. As in Lint, of the imports that have a configured alias only the file's last
  one is reported.
- `WrongResourceImportAlias` reports the file's first R class import whose alias differs from the configured one, so
  it reports nothing until `importAliases` is set.

Three Android source checks prove their receiver before reporting:

- `CastingViewContextToActivity` reports `view.getContext() as Activity` (also `as?` and a Java cast) when the
  receiver's type is a `View` subtype, or the call is an implicit `getContext()` in a Kotlin or Java `View` subclass.
  As in Lint, the Kotlin property form `view.context` and a parenthesized call are not reported. Unlike Lint, an
  `is Activity` / `instanceof Activity` check is not reported: it is not a cast.
- `ArgInFormattedQuantityStringRes` reports format arguments of `Resources.getQuantityString` that are not a
  `getFormattedCount()` call (in Java, also a local initialized by one). The receiver must be typed `Resources` (or a
  project subclass), or be Android's accessor (`context.resources`, `getResources()`, an unqualified `resources`)
  on a Context, View or Fragment, so a project's own `getQuantityString` wrapper is not reported.
- `SpanMarkPointMissingMask` reports `==`/`!=` against a `Spanned` mark-point flag (`SPAN_EXCLUSIVE_EXCLUSIVE`, ...)
  whose other side does not mask with `SPAN_POINT_MARK_MASK` (a bitwise `and`/`&`; `or`/`|` does not mask). A
  parameter or local named like the flag shadows its static import. Unlike Lint, krit looks inside parentheses, so
  the correctly masked `(flags & Spanned.SPAN_POINT_MARK_MASK) == Spanned.SPAN_EXCLUSIVE_EXCLUSIVE` is not
  reported. Lint's quick fix, which inserts the mask, is not ported.

`SubscribeOnMain` reports `subscribeOn(AndroidSchedulers.mainThread())`, or `immediateMainThread()`, on RxJava 3's
`Observable`, `Flowable`, `Single`, `Maybe` and `Completable`. Lint resolves the receiver's type; krit requires the file
to import from RxJava 3 and not from RxJava 1/2 or Reactor, which also declare `subscribeOn`. As in Lint, a scheduler
variable counts when its initializer ends in `mainThread()`; a parameter does not, nor (in Java) a statically imported
`mainThread()`.

`RxObservableDoesNotEmit` and `RxFlowableDoesNotEmit` report a kotlinx-coroutines-rx `rxObservable`/`rxFlowable`
whose lambda never calls `send()` or `trySend()` on its `ProducerScope`. A nested factory is checked on its own. They
follow slack-lints main (c0e92cb): the 0.11.1 release looks for `kotlin.coroutines.ProducerScope`, which does not exist,
so it never reports on real code.

Five single-node checks resolve the names they need through the file's imports and stay silent when a same-file
declaration could shadow them:

- `AvoidUsingNotNullOperator` reports every Kotlin `!!`, at the operator. It differs from detekt's
  `UnsafeCallOnNullableType`, which needs type information and skips some receivers, so it is a separate rule.
- `ExceptionMessage` reports kotlin's `check`, `checkNotNull`, `require` and `requireNotNull` without a `lazyMessage`
  (trailing, positional or named), outside test sources. A call through an import alias (`import kotlin.require as req`)
  is not checked.
- `NullableConcurrentHashMap` reports a nullable key or value type argument of a `java.util.concurrent.ConcurrentHashMap`
  variable type or constructor call; in Java, a type argument annotated `@…Nullable`.
- `GuavaPreconditionsUsedInKotlin` reports Kotlin calls into `com.google.common.base.Preconditions`. Lint's quick fix,
  which renames `checkState` to `check` and leaves the message argument in a position `check` does not accept, is not
  ported.
- `ParcelizeFunctionProperty` reports function-typed primary constructor parameters of a `@Parcelize` class, including
  `kotlin.FunctionN` and same-file typealiases. As in Lint, `@IgnoredOnParcel` does not exempt them: it targets the
  property, and the detector reads the parameter.

The Dagger rules (`MustBeInModule`, `BindsMustBeAbstract`, `ProvidesMustNotBeAbstract`, `BindingReturnType`,
`BindingReceiverParameter`) port `DaggerIssuesDetector` for Kotlin and Java `@Binds`/`@Provides` methods. As in
Lint, a method gets at most one of the detector's issues, in the detector's order. Two of its IDs map onto existing
rules instead: `BindsWrongParameterCount` is a Lint alias of `BindsMismatchedArity`, and `RedundantBinds` of
`BindsReturnTypeMatchesParam`. Differences from Lint:

- a finding on a whole method is anchored at its first annotation or modifier. Lint starts the location at a KDoc,
  Javadoc or comment directly above it, which would put the finding outside the range a `@SuppressLint` on the
  method covers;
- `BindingReturnType` abstains on a Kotlin expression body without a declared type, unless the body is `Unit`.

`BindsTypeMismatch` and `DoNotCallProviders` complete the Dagger checks and look across the project:

- `BindsTypeMismatch` is the detector's last check: a `@Binds` method none of the earlier checks reported whose
  parameter type is not assignable to its return type. krit reports it only when the return type is a project class
  and the parameter is either a project class whose project supertypes never reach it, or a library type, which
  cannot implement a project type. Names resolve in the enclosing classes first, then through imports and the
  package. It abstains on a parameterized return type, a library return type, and a parameter or supertype it
  cannot place (a typealias, a generated class of a project package).
- `DoNotCallProviders` reports a call from main sources to a project method annotated `@Binds`, `@Provides` or
  `@Produces`, unless the innermost class is annotated `@Generated`. The call is matched through the class name
  (`Module.INSTANCE.m()` and `Module.Companion.m()` from Java too), the receiver's resolved or declared type, a
  `with`/`run`/`apply` receiver, or the enclosing class. Provider methods in libraries are not seen.

The `DoNotMock*` rules port `MockDetector` and, as in Lint, look only at test sources (`src/test`, `src/androidTest`
and the other test source sets). A mock site is a call to one of the detector's default factories
(`org.mockito.Mockito.mock`/`spy`, `org.mockito.kotlin.mock`/`spy`, `slack.test.mockito.mock`), also under an import
alias, or a field annotated `@org.mockito.Mock` or `@org.mockito.Spy`; Lint's `mock-factories` and `mock-annotations`
options are not read yet. The mocked type comes from a type argument, a class literal, the declared type of the field
or of the variable a `mock()` initializes, or a local variable those initialize. The detector's checkers run in its
order (platform, data, `@DoNotMock`, sealed, AutoValue, object, record), so a platform data class such as `kotlin.Pair`
is reported as a platform type, and a type annotated `@DoNotMock` is left to that check. A class reached only through an
on-demand import is resolved to that package when it is a well-known platform class (`android.content.Context`,
`java.util.Date`, ...); any other unindexed name might be a same-package class krit does not see, so it abstains.

`DoNotMockAnything` reports every mock. Lint keeps it disabled unless it is named, and so does krit: it is not
experimental, so `--experimental` does not turn it on, and it is marked noisy, so `--strict` leaves it off. Enable it
with `--enable-rules DoNotMockAnything` or in `krit.yml`.

Differences from Lint:

- a `mock()` passed as an argument, whose type Lint infers from the parameter, is not reported;
- `DoNotMockAnything` reports alongside the other `DoNotMock*` rules. Lint runs it first, so when it is enabled the
  others stay silent;
- `DoNotMockRecordClass` also reports Java records, which Lint 0.11.1's own tests leave out pending Lint support.

`InclusiveNaming` ports `InclusiveNamingSourceCodeScanner` for Kotlin and Java. It reports nothing until its
`blockList` option (Lint's `block-list`) lists words, for example:

```yaml
slack-lint:
  InclusiveNaming:
    active: true
    blockList: [master, slave, whitelist, blacklist]
```

As in Lint, a name matches when its lower-cased text contains an entry as written, and the names checked are those of
the file, its package and the declarations Lint's light classes have: classes, functions and methods, fields and
Kotlin properties (as the backing field and the `getX` getter, so a `val` constructor parameter is reported twice),
parameters, local variables and labels. A local function, a `for` loop variable and a destructured lambda parameter
are not checked, matching Lint. Differences from Lint:

- an `internal` Kotlin member is reported under its source name. Lint reports the JVM name, which carries a
  module-dependent suffix;
- the resource half of the detector (XML attribute names) is not ported.

`Deprecation` also answers to slack-lints' `DeprecatedCall` ID, so `@SuppressLint("DeprecatedCall")` and `lint.xml`
entries for it apply. `DeprecatedCall` reports calls to deprecated methods and constructors; `Deprecation` covers
those and deprecated property and type references too, and uses the compiler's verdict when the type oracle runs.

## Compose lint checks (compose-lints)

Rules named `Compose…` with a Lint issue ID port [compose-lints](https://github.com/slackhq/compose-lints)
(`com.slack.lint.compose:compose-lint-checks`) 1.6.1. They live in the `compose` rule set, analyze Kotlin only (as
compose-lints does), and use the Lint issue ID, message and severity, so `@SuppressLint`, `tools:ignore` and
`lint.xml` entries for the Lint ID apply. They are experimental and off by default: they encode the compose-lints
style guide, and some overlap AndroidX Compose ports that are on. Enable them by ID
(`--enable-rules ComposeCompositionLocalUsage`) or with `--experimental`. Their findings are checked against the Lint
CLI running compose-lint-checks 1.6.1 on the inputs under `internal/rules/testdata/compose_lints/`.

| Lint issue | krit rule | Notes |
|------------|-----------|-------|
| `ComposeNamingUppercase`, `ComposeNamingLowercase` | `ComposableNaming` | The AndroidX port (on by default) reports both casings. `@Suppress`/`@SuppressLint` of a compose-lints ID silences its direction only (Uppercase: Unit composables, Lowercase: value-returning ones); a `lint.xml` entry for them leaves `ComposableNaming` on, as in Lint. Its `allowedComposableFunctionNames` option is compose-lints' `allowed-composable-function-names`. Unlike compose-lints it skips overrides, and it also reports lowercase Unit composables with a receiver and expression bodies with a provable return type. |
| `ComposeCompositionLocalUsage` | same ID | `allowedCompositionLocals` is compose-lints' `allowed-composition-locals`. The factory must resolve to `androidx.compose.runtime`; Lint matches the name alone. |
| `ComposeCompositionLocalGetter` | same ID | The factory must resolve to `androidx.compose.runtime`, as for `ComposeCompositionLocalUsage`. It has no allowlist. |
| `ComposePreviewPublic` | same ID | Runs once per analysis over the cross-file index, so multipreview annotations declared in other files count. Multipreviews resolve to top-level annotation classes in the analyzed sources and to the AndroidX (`PreviewLightDark`, `PreviewFontScale`, `PreviewScreenSizes`, `PreviewDynamicColors`) and Wear tooling multipreviews. compose-lints' autofix (make the function private) is not ported. |
| `ComposePreviewNaming` | same ID | Cross-file like `ComposePreviewPublic`; top-level annotation classes only. |
| `ComposeM2Api` | same ID | Off by default in compose-lints too. Resolves through imports, not types: a call or reference to an `androidx.compose.material` top-level declaration (explicit import, or the wildcard import for a name Material 2 declares), `M2Class.member`, `MaterialTheme.colors/typography/shapes.member`, and imported M2 extensions. A member read on a local of an M2 type (`colors.primary`) is a known false negative. `allowedM2Apis` (compose-lints `allowed-m2-apis`) matches source names, as Lint does with `enable-mangling-workaround`. |
| `ComposeItemKeyHashCode` | same ID | A zero-argument `hashCode()` inside a `key =` argument of a call in a `@Composable` function. |
| `ComposeModifierComposed` | same ID | The receiver must be provably a `Modifier` (`Modifier`, `this` or the implicit receiver of a `Modifier` extension, a parameter or local declared as `Modifier`, or a call chain rooted in one). |
| `ComposeModifierMissing` | same ID | Cross-file (previews are exempt, including multipreviews from other files). Options: `contentEmitters` (compose-lints `content-emitters`) and `visibilityThreshold` (`only_public`, `public_and_internal` or `all`; compose-lints `visibility-threshold`). An extension receiver that resolves to `Modifier` or `androidx.glance.GlanceModifier` counts as the modifier parameter, as in Lint's UAST. |
| `ComposeModifierWithoutDefault` | same ID | Autofix (idiomatic): appends a default written as the parameter's type (` = Modifier`, ` = GlanceModifier`, the qualified name or the import alias). A `vararg` parameter, or a lookalike type named `Modifier` or `GlanceModifier` that resolves elsewhere, is reported without a fix. |
| `ComposeParameterOrder` | same ID | Cross-file, so a trailing-function type alias declared in another file is expanded. As in Lint, a trailing function is a function type, a type alias of one, `kotlin.Function`/`FunctionN` or `kotlin.jvm.functions.*`, or a Java `@FunctionalInterface` type such as `Runnable` or `Comparator`; a Kotlin `fun interface` is not, and neither is a project class that shadows one of those names. The message prints each parameter's source text with whitespace runs collapsed, so a multi-line default reads as one line (Lint keeps the line breaks). compose-lints' reordering autofix is not ported, because moving parameters breaks positional callers. |

## Compose-aware defaults

Krit recognizes common Compose patterns without extra config:

- `@Composable` functions skip PascalCase naming checks
- `@Preview` functions skip magic number checks
- Color hex literals and `.dp` / `.sp` values are recognized
- Ignore-by-annotation lists (`ignoreAnnotated`) take short names (`Composable`, not the FQN)

## Production-only security rules

`HardcodedSecretKey` and `StaticIv` are enabled by default and report only in
production source. Both Go and FIR skip files classified as tests, including
Gradle `src/test` and `src/androidTest` source sets. Hardcoded keys and static
IVs in tests are therefore left silent without changing production findings.

## Auto-fix safety levels

| Level     | What it changes | Example |
|-----------|-----------------|---------|
| Cosmetic  | Whitespace, formatting, redundant keywords | Remove `public`, fix trailing whitespace, reorder modifiers |
| Idiomatic | Equivalent Kotlin conventions | `check(x != null)` → `checkNotNull(x)`, `.filter{}.first()` → `.first{}` |
| Semantic  | Changes that can affect edge-case behavior | Remove unused code, `var` → `val`, `as` → `as?` |

`krit --fix .` applies cosmetic + idiomatic. `krit --fix --fix-level=semantic .` applies everything. Use `--dry-run` to preview.

## Suggested fixes

A rule that cannot reduce to a single safe rewrite may instead emit
**suggested fixes** — an ordered, user-selectable list. Autofix and
suggested fixes are mutually exclusive per rule, and the order is
rule-recommended. See [Suggested fixes](suggested-fixes.md) for the
full contract, an authoring example, JSON shape, and integration
guidance.

## Rule types (internals)

- **Node-dispatched rules** — register `NodeTypes` and receive matching flat AST nodes.
- **Line-pass rules** — declare `NeedsLinePass` and scan file lines in a single pass.
- **Project-scope rules** — declare capabilities such as cross-file, module,
  manifest, resource, Gradle, resolver, or oracle data and run after the needed
  indexes are available. A project-scope rule sees every file at once, so it
  builds its indexes once per run and answers each query with a lookup; a
  helper that scans or copies every project class per query makes the rule
  quadratic (#1658). A rule that only reads the phase inputs declares
  `NeedsConcurrent`, and the cross-file and manifest-source phases then run it
  beside the other rules.
- **JVM-backed rules** — use source inference first where possible and can opt
  into FIR helper facts when source analysis is not enough.

See `internal/rules/` for implementations and `tests/fixtures/` for positive/negative examples.

## Android Lint Wear Checks

Ports of Android Lint 32.4.1's Wear OS detectors under their Lint IDs, severities and messages. Each has project
fixtures under `tests/fixtures/android-lint/<RuleID>/` with `lint-32.4.1.txt`, the findings Lint reported for the same
Gradle module (recorded with `scripts/lint-parity-record.sh`, AGP 9.4.1); `TestAndroidLintParityFixtures` in
`cmd/krit` replays krit on each and requires the same file, line, column and message.

- `WearableConfigurationAction` (warning) reads each `AndroidManifest.xml` of a project that may depend on
  `androidx.wear.watchface:watchface` and is not a Watch Face Format project. With the
  `wearableConfigurationAction` metadata set to `WATCH_FACE_EDITOR` and no intent filter for that action it reports
  "Watch face configuration activity is missing" at the metadata name. Below minSdk 30 it also reports an editor
  intent filter without the `WEARABLE_CONFIGURATION` category, and an editor intent filter without the metadata.
- `WearableActionDuplicate` (warning) reports a second intent filter for the editor action. Like Lint, the finding
  (and the `tools:ignore` that silences it) sits on the first such `<action>`.
- `WatchFaceForAndroidX` (warning) reports `wearableConfigurationAction` metadata whose value is not
  `WATCH_FACE_EDITOR`, at the value (or the element when the value is missing), only when the build scripts declare
  `androidx.wear.watchface:watchface`. Lint 32.4.1 does not skip Watch Face Format projects here, so krit does not.
- `WearBackNavigation` (warning) reports `<item name="android:windowSwipeToDismiss">false</item>` in any values
  folder of a module whose manifest declares the `android.hardware.type.watch` feature.
- `WearMaterialTheme` (error, Kotlin and Java) reports an import of `androidx.compose.material.MaterialTheme` in such a
  module; a file belongs to the nearest module root above it, so a phone module nested in a Wear project is not
  checked. Lint compares the import text, so an aliased import and `MaterialTheme.*` count; Material 3 imports do not.

- `SquareAndRoundTilePreviews` (warning) reports a tile service (a `<service>` with the
  `androidx.wear.tiles.action.BIND_TILE_PROVIDER` action) without an `androidx.wear.tiles.PREVIEW` metadata ("Tiles
  need preview assets", in any project). In a Wear module it also reads the binary drawables named by each
  `@drawable/` preview in every source set's `drawable*` folders and reports, at the metadata's resource value, a
  preview found only in folders whose name contains `round` or only in the others. This is Lint 32.4.1's behavior;
  later Lint renamed the check `TilePreviews` and dropped the round/square requirement.
- `TilePreviewImageFormat` (error) reports, at the same value, a preview image that is not square or is smaller than
  384px in either dimension. PNG, JPEG and GIF headers are decoded; a WebP preview is 0x0, as it is for Lint, whose
  `ImageIO` has no WebP reader; other formats are skipped.
- `StartActivityAndCollapseDeprecated` (Kotlin and Java) reports the argument of `startActivityAndCollapse(Intent)` on
  a `TileService`: an error from target SDK 34, a warning below. The receiver must be proven a `TileService` (an
  implicit or `this` receiver in a class whose same-file supertype chain reaches it, or a receiver declared as one)
  and the argument an `Intent` (a constructor call, a variable declared or initialized as one, or an `Intent` builder
  call on one); the `PendingIntent` overload and same-named methods of other classes are not reported.

Lint checks the merged manifest; krit checks each manifest file, so components declared only in a variant or library
manifest are not seen, and the watch face dependency is read from build scripts and version catalogs.

## Android Lint Source Text Checks

Ports of Android Lint 32.4.1 source checks, with project fixtures and recorded Lint output under
`tests/fixtures/android-lint/<RuleID>/`:

- `TrimLambda` (info, Kotlin, autofix) reports the lambda of `s.trim { it <= ' ' }` or `s.trim { it.isWhitespace() }`
  (implicit or single named parameter, parentheses allowed) on a `String` or `CharSequence`, with or without
  parentheses around it. The fix removes the lambda as Lint's quick fix does; it is `FixSemantic` because
  `<= ' '` and `isWhitespace()` disagree on some characters. Without type information the receiver must be a string
  literal or a variable declared as `String`/`CharSequence`, and a file declaring its own `trim` or `isWhitespace` is
  skipped.
- `TextConcatSpace` (warning, Kotlin and Java) reports a `+` of two string literals on different lines where the left
  one ends with a word after a space, has at least two words, and the right one starts with a letter, at the right
  literal (Kotlin: its text; Java: its opening quote). Like Lint, a Kotlin escape or template at the joint ends the
  match.
- `WrongCommentType` (warning, Kotlin and Java, autofix `/*` → `/**`) reports a block comment on a non-private class,
  object, function, property with a backing field, `val`/`var` constructor parameter, accessor or enum entry that
  holds a doc tag: a lowercase `@tag` at a line start (in Kotlin, not when the tag's text contains a period), a
  `[text](url)` link in a KDoc description, or `{@link}`/`{@linkplain}`/`{@inheritDoc}` in a Javadoc description.
  Kotlin binds the run of comments before a declaration to it and Lint reads the first, so a line comment first
  hides a tagged block comment after it; Java uses the nearest comment.

## AndroidX Library Lint Checks

Lint loads the checks bundled in an AndroidX AAR only when that AAR is on the module's classpath. Krit's ports of
them run only when the project declares the library (or one that brings it, such as Material for appcompat, or
appcompat or Material for fragment); a library declared through buildSrc constants or a convention plugin, or that
arrives only transitively, is not seen, so those checks stay silent there.

The ports of the Compose material, animation, foundation and ConstraintLayout checks and of the activity, lifecycle,
annotation, startup, webkit and UiAutomator checks are described in [AndroidX Lint ports](androidx-lint-ports.md).

`androidx.appcompat:appcompat` 1.8.0:

- `UseAndroidAlpha` reports `app:alpha` on a `<selector>` `<item>` that has `android:color` but no `android:alpha`,
  in any resource folder. Lint reports an item of a nested `<selector>` once per enclosing selector at the same
  position; krit reports it once.
- `UseAppTint`, `UseSwitchCompatOrMaterialXml` and `UseCompatTextViewDrawableXml` match the exact layout tags
  `ImageView`/`ImageButton`, `Switch` and `TextView`, as Lint does, so `android.widget.Switch` or
  `AppCompatTextView` tags are not reported. Attributes are matched by namespace URI, not by prefix.
- `UsingOnClickInXml` reports an `onClick` attribute in any namespace whose value is not a data binding expression,
  when minSdk is below 23. When the build does not show minSdk (for example, a convention plugin sets it), Lint
  assumes API 1 and reports; krit abstains.

`androidx.fragment` 1.9.1 and `androidx.activity:activity` 1.13.0:

- `FragmentGradleConfiguration` reports a bare string dependency on `androidx.fragment:fragment-testing` or
  `fragment-testing-manifest` declared on any configuration but `debugImplementation`, at the statement. Both testing
  artifacts register this ID, so Lint reports a `fragment-testing-manifest` line twice; krit reports it once. Lint's
  detector also reports any other assignment of such a string (an `ext` constant, a local variable, a
  `testInstrumentationRunner` value with the same prefix); krit only reads the `dependencies` closure.
- `FragmentTagUsage` reports the tag name of each `<fragment>` in a layout. Navigation graph destinations are not
  layouts and are not reported.
- `InvalidFragmentVersionForActivityResult` reports each `registerForActivityResult(...)` call (Lint matches the name
  only) in a module whose `androidx.fragment:fragment` dependency is older than 1.3.0. Krit reads the module's build
  file and its version catalog; a fragment version that only arrives transitively is a known false negative until
  dependency artifacts are analyzed (#1120). Lint compares versions as strings, so it reports `1.10.0` as older than
  `1.3.0`; krit compares the numbers. Lint reports a call once per old declaration; krit once.
- `InvalidUseOfOnBackPressed` reports `onBackPressed()` inside a method named `handledOnBackPressed`, exactly as Lint
  does. `OnBackPressedCallback`'s method is `handleOnBackPressed`, so the shipped detector does not fire inside a real
  callback; krit keeps Lint's verdicts rather than reporting what Lint does not.

`androidx.appcompat:appcompat` 1.8.0 source checks (Kotlin and Java):

- `UseSupportActionBar` reports `setActionBar(toolbar)` whose receiver (explicit, `this`, or the enclosing class)
  extends `AppCompatActivity`.
- `UseCompatLoadingForColorStateLists` reports `Resources.getColorStateList(int)`. The message names
  `ContextCompat.getColorStateList()` from minSdk 24 and `AppCompatResources.getColorStateList()` below it, or when
  minSdk is unknown, as Lint does. `Context.getColorStateList(int)` is not reported.
- `UseCompatLoadingForDrawables` reports `Context.getDrawable(int)`, `Resources.getDrawable(int)` and
  `Resources.getDrawable(int, Theme)`.
- `UseCompatTextViewDrawableApis` reports `setCompoundDrawableTintList` / `setCompoundDrawableTintMode` on a `TextView`
  and the Kotlin property assignments `compoundDrawableTintList = …` / `compoundDrawableTintMode = …` that call them.
- `UseSwitchCompatOrMaterialCode` reports properties, locals, fields and parameters of type `android.widget.Switch`
  (declared, constructed, cast or from `findViewById<Switch>`) and classes that extend it.

Receivers are proven from declarations, imports, constructor calls, the resolver's class hierarchy, Java facts and the
type oracle when it runs. Where Lint's compiler resolution sees further, krit abstains: an implicit receiver inside
`apply`/`run`/`with` whose subject type is not proven (for example `holder.label.apply { … }`, or a call that Kotlin
resolves past the subject to the enclosing class) is not reported.
`androidx.fragment` 1.9.1 lifecycle-owner checks (Kotlin and Java):

- `FragmentLiveDataObserve`, `FragmentBackPressedCallback` and `FragmentAddMenuProvider` report the Fragment itself
  passed as the `LifecycleOwner` of `LiveData.observe`, `OnBackPressedDispatcher.addCallback(owner, callback)` or
  `MenuHost.addMenuProvider` from `onCreateView`, `onViewCreated`, `onActivityCreated` or `onViewStateRestored` of a
  Fragment that is not a `DialogFragment`. As in Lint, calls are followed into same-file helpers (Lint follows them
  into other files too), an owner that is the enclosing class is reported at the argument, and one that is a Fragment
  of another class at the call ("Unsafe call to …"). The Kotlin `addCallback(owner) { }` extension is not reported,
  as Lint's owner check does not match it. An `observe` receiver must be proven to be `LiveData` from this file or by
  the type oracle, so a ViewModel property declared in another file is a known false negative of the Go path.
- `UnsafeRepeatOnLifecycleDetector` reports `repeatOnLifecycle` in those methods of such a Fragment unless the
  enclosing launch call has a receiver and `viewLifecycleOwner` appears, case-insensitively, in that receiver or any
  enclosing expression up to the method body. As in Lint, a mention anywhere in the method body (a comment included)
  passes.

`androidx.compose.ui` 1.12.1 (`ui-graphics-android`, `ui-text-android`), Kotlin and Java:

These Compose checks run on any file that imports the Compose API they check, since the import already proves the
library is present; Compose Multiplatform code is checked too, although Lint does not run there. Their IDs are the Lint issue
IDs, so `@SuppressLint`, `tools:ignore`, `lint.xml` and Gradle `lint { }` severities apply to them.

- `MissingColorAlphaChannel` and `InvalidColorHexValue` report the literal passed to a one-argument Compose
  `Color(...)` (imported, star-imported, aliased or fully qualified, and not shadowed by a same-file `Color`). Lint
  counts the digits after `0x` with `_` and `L` removed: 6 digits misses the alpha channel (the fix prepends `FF`, or
  `ff` when the first letter digit is lowercase), 8 is valid, and any other count is invalid. Parenthesized,
  negated or computed arguments are not literals and are skipped, as in Lint. Lint cannot resolve `Color(0x...u)`
  (there is no `UInt` overload), so neither check reports it.
- `InvalidLanguageTagDelimiter` reports a Compose `Locale` / `LocaleList` language tag containing `_`, at the
  argument, in Kotlin and Java. Tags are evaluated from literals (raw strings included), `+` concatenation, parentheses
  and same-file `val`s and constants; string templates are not constant, as in Lint. A constant declared in another
  file is a known false negative. The fix rewrites a literal tag only.

`androidx.compose.ui:ui-test-manifest` 1.12.1 (Gradle):

- `TestManifestGradleConfiguration` reports a statement whose value is a bare string starting with
  `androidx.compose.ui:ui-test-manifest` and whose name, lowercased, contains `implementation`, `api`, `compileonly`,
  `runtimeonly`, `annotationprocessor`, `lintchecks` or `lintpublish` but not `debugimplementation`, and starts with one
  of them or with `test` or `android`. As in Lint, the enclosing block does not matter, so an `ext` property named like a
  configuration is reported too; catalog accessors, `platform(...)`, map notation and declarations with a closure are
  not read. The fix renames the configuration to `debugImplementation`. The Lint CLI cannot run Gradle checks on Kotlin
  scripts, so the `.kts` behavior follows the detector's source.
`androidx.compose.ui:ui-android` 1.12.1 pointer input (Kotlin):

- `awaitPointerEventScope` is a `PointerInputScope` member, so it needs no import; the checks require the file to import
  from `androidx.compose.ui.input.pointer` and not to declare a function of that name.
- `MultipleAwaitPointerEventScopes` counts the calls within the nearest enclosing `pointerInput` call (Lint's first
  Modifier-typed ancestor; only its own arguments and trailing lambda, not the receiver chain) or, failing that, the
  containing declaration (a local `val` counts as one, as in Lint), and reports each call when there is more than one.
- `ReturnFromAwaitPointerEventScope` reports an unqualified call that initializes a local variable, or that is returned
  (`return`, an expression body, or the last expression of any lambda, which UAST treats as an implicit return) with no
  enclosing call named `pointerInput`, `PointerInputEventHandler` or `coroutineScope` and no enclosing lambda declared as
  `suspend PointerInputScope.() -> ...` on its property or parameter. Lint looks at the call's direct parent, so a
  qualified `scope.awaitPointerEventScope { }` is never reported; krit keeps that verdict.
`androidx.compose.ui:ui-android` 1.12.1 Modifier declarations (Kotlin):

- `ModifierFactoryExtensionFunction`, `ModifierFactoryReturnType` and `ModifierFactoryUnreferencedReceiver` check
  functions (members, interface functions and overrides included, local functions excluded) and top-level `val`
  getters that return a Modifier other than a `ParentDataModifier`. The return type must be declared as
  `androidx.compose.ui.Modifier`, `Modifier.Element`, a public Compose Modifier interface (`LayoutModifier`,
  `DrawModifier`, `ModifierNodeElement`, ...) or a same-file class implementing one, or be shown by an expression body
  (`Modifier`, a `Modifier.padding(...)...` chain of known factories, an object literal or a same-file constructor).
  Other inferred return types need type resolution and are not checked.
- `ModifierFactoryUnreferencedReceiver` treats any `this` and any unqualified call that may bind the receiver
  implicitly as a use; calls to constructors, Kotlin top-level functions such as `listOf` and `TODO`, function-typed
  locals and same-file top-level functions are not. Lint reports every extension-property getter, because its check
  only matches named functions; krit reports only getters that do not use the receiver.
- `ModifierParameter` checks the first Modifier-typed parameter of a `@Composable` function returning `Unit`: it should
  be named `modifier`, typed `Modifier`, default to the bare `Modifier` and be the first optional parameter. Lint's
  rename and retype quick fixes are not ported, because they would leave call sites and the body unchanged.

`androidx.compose.ui:ui-android` 1.12.1 Modifier usage and nodes:

- `UnnecessaryComposedModifier` reports `composed` (imported from `androidx.compose.ui`) whose `factory` argument, nested
  lambdas included, calls nothing that may be `@Composable`. A call proves nothing when it names a same-file function
  that is not `@Composable`, a known Compose Modifier factory, a Kotlin stdlib function or a same-file class; any other
  call, a `.current` read or a property read on an object other than `Modifier`, `Color`, `Alignment` or `Arrangement`
  makes krit abstain, so factories that call functions declared in other files are not reported.
- `SuspiciousModifierThen` reports, inside `then(arg)` on a Modifier receiver, each call in `arg` without an explicit
  receiver to a same-file `Modifier.` extension or a known Compose Modifier factory. Calls in lambdas inside `arg` have
  their own receivers and are skipped. As in Lint, a call nested in two `then`s is reported once per `then`.
- `ModifierNodeInspectableProperties` (informational) reports Kotlin classes, objects and object literals (named `null`,
  as Lint does) that reach `ModifierNodeElement` through same-file superclasses without declaring
  `fun InspectorInfo.inspectableProperties()`, and Java classes extending `ModifierNodeElement`, which cannot declare it.
  A superclass from another file is a known false negative.
- `SuspiciousCompositionLocalModifierRead` reports `currentValueOf(...)` in `onAttach`/`onDetach` and constructors
  (`init` blocks and secondary constructors) of a class that itself implements `CompositionLocalConsumerModifierNode`
  (stopping at lambdas and local functions, as Lint does), and inside any `lazy { }`.

`androidx.compose.ui:ui-android` 1.12.1 LocalContext, Configuration and Locale (Kotlin):

- `LocalContextConfigurationRead` reports `….configuration` read from the `resources` of `LocalContext.current`,
  directly or through same-file variables (`val context = LocalContext.current`, `val res = context.resources`).
  `LocalContextResourcesRead` reports `LocalContext.current.resources` (or `context.resources`) unless that read is part
  of such a Configuration read, so each chain is reported once, as in Lint. `LocalContextGetResourceValueCall` reports
  `getString`, `getText`, `getColor`, `getDrawable` and `getColorStateList` on `LocalContext.current` or a variable
  holding it. Lint's quick fixes are not ported, because they add imports.
- `ConfigurationScreenWidthHeight` reports `screenWidthDp`/`screenHeightDp` on a value shown to be a `Configuration`
  (`LocalConfiguration.current`, `….resources.configuration`, or a parameter or val declared or initialized as one) in
  composition, or on a variable initialized to `LocalConfiguration.current` anywhere. It joins source with manifests:
  files under a module whose `AndroidManifest.xml` declares `android.hardware.type.watch` are skipped, matching Lint's
  merged-manifest filter, and so are files importing `androidx.wear`. It only runs in projects with a manifest.
- `NonObservableLocale` reports `java.util.Locale.getDefault()`, `android.os.LocaleList.getAdjustedDefault()` and
  `androidx.core.os.LocaleListCompat.getAdjustedDefault()` in composition, using the same composition walk as the Compose
  runtime ports: reads inside `remember { }` and other non-inline lambdas are not in composition, while reads inside
  stdlib inline lambdas such as `let` and `forEach` are. Reads inside `map`, `filter` and the other collection
  operations count only when the receiver is proven to be an `Iterable`, `Array` or `Map`.

## Android Lint Performance Checks

Ports of Android Lint 32.4.1's built-in Performance detectors, under their Lint IDs:

- `UnnecessaryArrayInit` (Kotlin) reports the initializer lambda of a `kotlin.*` primitive or unsigned array
  constructor (`IntArray(n) { 0 }`) when its single expression folds to a numeric zero (`0`, `0L`, `0f`, `0.0`,
  `1 - 1`, a same-file constant). `false`, chars and unsigned literals are not numbers to Lint, so those initializers
  are left alone. The fix removes the lambda.
- `ExpensiveAssertion` (Kotlin, opt-in) reports `assert(...)` conditions that do work Kotlin performs even with
  assertions off. As in Lint, literals, names, `is` checks and qualified references or calls (`list.isEmpty()`,
  `a.b()`) are cheap; unqualified library calls other than `isEmpty`/`isNotEmpty`/`contains`, constructors, `if`,
  `when` and `?:` are expensive, and so is nesting four levels deep. An unqualified call to a function of the same
  file is judged by its single-return body. A call to a function declared in another file is not followed and not
  reported. Asserts inside an `if` (or `when` branch) whose condition mentions `desiredAssertionStatus` are skipped.
- `AssertionSideEffect` (Kotlin and Java) reports increments, decrements and assignments in an assertion condition,
  and in Kotlin, calls into same-file functions (up to three levels) that perform one. Mutating library calls (`add`,
  `remove`, `put`, `delete`, `mkdir`, `mkdirs`, `setX`) are reported when the receiver is a JDK or Android class,
  including the `kotlin.collections` typealiases `ArrayList`/`HashMap`/`HashSet` and `StringBuilder`. As in Lint,
  members of `kotlin.*` types (`MutableList.add`) are not side effects, and Java calls are not followed. Lint
  resolves receiver types the compiler knows; krit reports only receivers whose type is declared or constructed in
  the file.

## Android Lint Performance Checks (Gradle)

Ports of Android Lint 32.4.1's Gradle-scope Performance detectors, under their Lint IDs. Lint reads these from its
Gradle model; krit reads the build script, the version catalog, and the project's AGP version.

- `ProguardAndroidTxtUsage` reports `getDefaultProguardFile(...)` whose argument names `proguard-android.txt`, in an
  app module. It is a warning before AGP 9 and an error from AGP 9 (or when the AGP version is unknown), as the
  Lint command line reports it. The fix switches to `proguard-android-optimize.txt`.
- `NotShrinkingResources` reports, in an app module, `shrinkResources false` (`isShrinkResources = false` in Kotlin
  scripts) in a block whose name matches a block with `minifyEnabled true`. When the script never sets
  `shrinkResources`, it also reports the last `minifyEnabled true`, until AGP 10. Like Lint, blocks are matched by name, so
  `getByName("debug")` and `getByName("release")` count as one block. The fixes set the value to `true` or add the
  missing line.
- `KaptUsageInsteadOfKsp` reports `kapt` dependencies (the `kapt` configuration only, not `kaptTest`) on the
  processors Lint lists as having a KSP version: Glide, Room, Moshi codegen, RxHttp, Kotshi, LINE savedstate,
  EasyAdapter, DeepLinkDispatch, Epoxy and Paris. Version catalog references are resolved. krit's broader `KaptUsage`
  rule stays separate: it also reports the kapt plugin and every other processor.
- `LifecycleAnnotationProcessorWithJava8` reports a dependency on `androidx.lifecycle:lifecycle-compiler` (or
  `android.arch.lifecycle:compiler`) in any configuration when the module compiles Java 8 or later. That comes from
  `sourceCompatibility`, `jvmToolchain`/`JavaLanguageVersion`, or else AGP 4.2 or later, whose default is Java 8. With
  no declaration and no known AGP version, krit does not report, because Lint treats an unknown level as Java 7.

- `NotificationTrampoline` (error) and `LaunchActivityFromNotification` (warning), Kotlin and Java, run once over all
  parsed files. A `setContentIntent`/`setFullScreenIntent`/`addAction` call on a proven
  `androidx.core.app.NotificationCompat.Builder` is followed to its `PendingIntent.getBroadcast`/`getService`. That
  can be direct, the last assignment of a local, or the return of a same-class Java helper. The trace then goes to
  the `Intent(context, X::class.java)` assigned to the local passed in. When `X` is a Java source class whose own
  `onReceive`/`onStartCommand` calls `startActivity`/`startActivities` (lambdas included), the notification call is a
  trampoline. When `X` cannot be found, and the call is not `addAction`, it is `LaunchActivityFromNotification`. Lint's class
  lookup does not find Kotlin classes, so a Kotlin receiver gets `LaunchActivityFromNotification`. Kotlin helper
  functions and qualified calls into other classes are not followed, as in Lint. `android.app.Notification.Builder`
  and the support-library `NotificationCompat` are not checked.

## Android Lint Performance Checks (manifest and resources)

- `WearableBindListener` (fatal) reports `com.google.android.gms.wearable.BIND_LISTENER` in an intent filter of a
  `<service>`, only in Gradle projects, as Lint does. With `com.google.android.gms:play-services-wearable` 8.2.0 or
  later declared (or declared without a readable version), the message is "The ... action is deprecated". Otherwise,
  from compileSdk 24, it asks to upgrade to 8.2.0 or later. Lint's variant naming the newest version installed in
  the SDK's Maven repository is not reproduced. Activities and other components are not checked.
- `DuplicateStrings` (opt-in, as in Lint) compares the `<string>` values of every values folder. Folders are grouped
  by locale (language and region); qualifiers such as `-night` share the default locale. Values are compared
  case-insensitively after Lint's quote stripping, so leading or trailing whitespace keeps two strings apart.
  `translatable="false"` strings, strings with child elements (markup or `xliff:g`) and empty strings are skipped.
  Each group is reported once, at the first definition in Lint's folder order. The message lists the names sorted as
  Lint sorts them and adds the `android:inputType`/`android:capitalize` note when the case varies. The resource index
  spans every module krit scans, which matches Lint on an app module that checks its dependencies.

- `DuplicateDivider` (Kotlin and Java) reports a class named `DividerItemDecoration` that extends
  `android.support.v7.widget.RecyclerView.ItemDecoration` (directly, through a class of the file, or through the
  resolver's hierarchy) and declares `HORIZONTAL_LIST` and `VERTICAL_LIST`. Fields, Kotlin properties and companion
  properties all count. Lint registers only the support-library supertype, so an androidx `ItemDecoration` subclass is
  not reported.
- `SyntheticAccessor` (Kotlin and Java, opt-in as in Lint) reports a private method, constructor or Java field used
  from a different class of the same file. That covers inner, nested, local and anonymous classes, the outer class,
  and the Kotlin file class for a top-level `private fun` called from a class. References resolve lexically:
  - an unqualified name binds to the innermost enclosing class declaring it (locals and local functions shadow);
  - `this`/`this@Outer`/`Outer.this`, a class name, a constructor call or a declared variable gives the receiver's
    class;
  - private extension functions resolve from the receiver's declared type.

  As in Lint, Kotlin properties, compile-time constants, lambdas (same class), calls from a companion object,
  sealed-class constructors, and constructors taking a value class are not reported. krit abstains in three cases:
  - for receivers whose type the file does not show;
  - in `apply`/`run`/`with` lambdas;
  - past a superclass declared elsewhere, for names a framework class could declare (single words and `get…`,
    `set…`, `on…`, `add…`, `start…`-style names), which Lint resolves with the compiler.

## Android Lint Performance Checks (AnimatorKeep)

`AnimatorKeep` (opt-in) follows an `ObjectAnimator` factory call to the property setter it reaches by reflection:

- **Target class:** the first argument's class, from `this`, a constructor call, or a variable declared or
  constructed as a class of the project.
- **Property name:** a string literal or constant.
- **Expected type:** the factory's value type (`ofInt`/`ofArgb` int, `ofFloat` float, `ofMultiInt`/`ofMultiFloat`
  arrays, `ofObject` by its evaluator).
- **`ofPropertyValuesHolder`:** each `PropertyValuesHolder.ofX` holder is followed the same way.

The setter is `setX` with one parameter of the expected type, in the class or a source superclass. A Kotlin `var x`
compiles to that setter.

It reports the setter's name once when the setter:

- is in the project's sources (library setters such as `View.setAlpha` are kept);
- is not static;
- has no `@Keep` (androidx or support) on itself or an enclosing class.

As in Lint, a Kotlin property needs `@set:Keep`: a plain `@Keep` annotates the property, not its setter.

Lint checks only modules that shrink, so krit stays quiet in a Gradle project whose build scripts never set
`minifyEnabled true`. ProGuard/R8 keep rules, which Lint also accepts, and the MotionLayout `CustomAttribute` half of
the detector (it needs source and resources together) are not covered.

`androidx.fragment` 1.9.1 API misuse checks (Kotlin and Java):

- `UseRequireInsteadOfGet` reports a Fragment's `arguments`, `context`, `activity`, `fragmentManager`, `host`,
  `parentFragment` or `view` (or the `getX()` call) asserted with `!!`, or passed alone to `checkNotNull` or
  `requireNonNull`, and names the `requireX()` rewrite as Lint does (`this.context!!` becomes
  `this.requireContext()`). Kotlin's `requireNotNull` is not one of Lint's null checks, and a check with a message is
  not rewritten. A local variable that shadows the getter is skipped, as in Lint; krit also skips a shadowing
  parameter, which Lint reports.
- `UseGetLayoutInflater` reports `LayoutInflater.from(x)` in a class whose direct superclass is `DialogFragment`. Lint's
  Kotlin path also asks whether the class extends the unqualified name `DialogFragment`, which never matches, so
  subclasses such as `AppCompatDialogFragment` are not reported in either language.
- `DialogFragmentCallbacksDetector` reports `setOnCancelListener` and `setOnDismissListener` calls inside
  `onCreateDialog` of a class whose first supertype is named `DialogFragment`, matching names as Lint does.
- `DetachAndAttachSameFragment` reports a `beginTransaction()` on a FragmentManager whose transaction, followed through
  call chains and local variables of the function, ends with `detach(f)` and `attach(f)` of the same fragment, at the
  `beginTransaction` name. Fragments are compared by their argument text; Lint compares the resolved declaration.
`androidx.work:work-runtime` 2.12.0 call-site checks (Kotlin and Java), checked against the Lint CLI running that
release's `lint.jar` (`internal/rules/testdata/androidx_work/`):

- `IdleBatteryChargingConstraints` reports `setRequiresCharging(true)` in the same block as `setRequiresDeviceIdle(true)`
  on a `Constraints.Builder` reached from the same variable (or both from a bare constructor chain), once per idle call.
  As in Lint, the walk does not enter call arguments or lambdas passed to calls, and an expression-bodied function has
  no block, so a one-expression builder chain there is not reported.
- `InvalidPeriodicWorkRequestInterval` reports a `PeriodicWorkRequest.Builder` constructor whose repeat interval folds
  below 15 minutes (`TimeUnit` constants convert with truncation; `Duration.ofX`). It follows the detector source: the
  Lint CLI reported no finding at all for this ID against the compiled 2.12.0 classes. As in the detector, a Java
  `int` argument is not evaluated. `PeriodicWorkRequestBuilder<W>(...)` is covered by `PeriodicWorkRequestLessThan15Min`.
- `BadPeriodicWorkRequestEnqueue` reports `WorkManager.enqueue(...)` with a proven `PeriodicWorkRequest` argument or a
  Kotlin `List<out PeriodicWorkRequest>`, at the call. Lint reports the location of `WorkManager.enqueue` in the
  library jar, and its `List<? extends WorkRequest>` match also reports lists of one-time requests; krit does not.
- `UseRxSetProgress2` reports `setProgress(...)` resolved to `androidx.work.RxWorker` (RxJava 2), not the RxJava 3
  `androidx.work.rxjava3.RxWorker` or `CoroutineWorker.setProgress`. When the call is a statement of its own, the fix
  renames it to `setCompletableProgress` (Lint's quick fix); a call whose `Single` result is used is not rewritten.

`androidx.work:work-runtime` 2.12.0 project checks (Kotlin and Java), which, like Lint's `afterCheckRootProject`,
look at every main-source file of the scan:

- `BadConfigurationProvider` reports when a concrete class implements `androidx.work.Configuration.Provider` but no
  concrete `Application` subclass does. Lint reports the whole file of the last `Application` it visited; krit anchors
  that finding at the file's first line.
- `SpecifyJobSchedulerIdRange` reports the first `android.app.job.JobService` subclass (other than WorkManager's
  `SystemJobService`) when nothing calls `setJobSchedulerJobIdRange`. A JobService alone does not mean the project uses
  WorkManager, so krit also needs an `androidx.work` dependency or import, where Lint needs the work `lint.jar` on the
  classpath.
- `WorkerHasAPublicModifier` reports each concrete `ListenableWorker` subclass that does not compile to a public class
  (Kotlin `private` or `protected`, Java without `public`; Kotlin `internal` is public in bytecode, as Lint sees it),
  unless the project calls `setWorkerFactory`.

Class hierarchies are followed through the project's own classes in any file and the androidx.work worker classes
(`Worker`, `CoroutineWorker`, `RxWorker`, `RemoteListenableWorker`), `MultiDexApplication` and `SystemJobService`.
Krit counts any `setJobSchedulerJobIdRange` / `setWorkerFactory` call without proving that its receiver is a
`Configuration.Builder`, so a builder it cannot trace never leads to a report.

`androidx.work:work-runtime` 2.12.0 source-and-manifest checks (Kotlin and Java), run once per scan with every parsed
manifest and source file:

- `RemoveWorkManagerInitializer` reports when a concrete `Application` implements `Configuration.Provider` (on-demand
  initialization) and no manifest removes `androidx.startup.InitializationProvider` or the
  `androidx.work.WorkManagerInitializer` meta-data with `tools:node="remove"`. As in Lint, it reports at the
  `WorkManagerInitializer` meta-data, else the `InitializationProvider`, else `<application>`, here in the
  Application's own module, and `@SuppressLint` on the Application class or `tools:ignore` in the manifest silences it.
- `SpecifyForegroundServiceType` reports a `ForegroundInfo(id, notification, type)` constructor once for each of
  `dataSync`, `mediaPlayback`, `phoneCall`, `location`, `connectedDevice` and `mediaProjection` in the folded `type`
  that no `SystemForegroundService` `<service>` lists in `android:foregroundServiceType`. Types are collected from every
  manifest of the scan.
