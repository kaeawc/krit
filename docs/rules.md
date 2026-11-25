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
  indexes are available.
- **JVM-backed rules** — use source inference first where possible and can opt
  into FIR helper facts when source analysis is not enough.

See `internal/rules/` for implementations and `tests/fixtures/` for positive/negative examples.

## AndroidX Library Lint Checks

Lint loads the checks bundled in an AndroidX AAR only when that AAR is on the module's classpath. Krit's ports of
them run only when the project declares the library (or one that brings it, such as Material for appcompat, or
appcompat or Material for fragment); a library declared through buildSrc constants or a convention plugin, or that
arrives only transitively, is not seen, so those checks stay silent there.

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
library is present; Compose Multiplatform code is checked too, although Lint does not run there.

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
