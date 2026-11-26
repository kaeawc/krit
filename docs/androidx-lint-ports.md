# AndroidX Lint ports

Krit ports the Lint checks that AndroidX libraries bundle in their AARs (`lint.jar`) under the Lint issue IDs, so
`lint.xml`, Gradle `lint { }` severities and `@SuppressLint` reach them. This page covers the ports of the
Compose material, animation, foundation and ConstraintLayout checks, and of the activity, lifecycle, annotation,
startup, webkit and UiAutomator checks; the older AndroidX ports (appcompat, fragment, work, recyclerview, Compose
runtime and UI) are described in [Rules](rules.md#androidx-library-lint-checks).

Like Lint, these checks also run on test sources. Each port is checked against the real detector: the inputs under
`internal/rules/testdata/<group>/` were run through the Android Lint CLI 9.4 with the release `lint.jar` on the rule
path and the library jars on the classpath, and `TestAndroidxLintParity` requires krit to report exactly the recorded
findings, apart from the divergences listed next to them.

Lint resolves every call; krit proves the callee from imports, fully qualified names and same-file declarations. A
function of the same name declared in the calling class, the enclosing function or the file itself wins over the
import, as it does in Kotlin.

## Activity Compose

`androidx.activity:activity-compose` 1.13.0 (Kotlin). Callees resolve through import aliases as in Lint:

- `LaunchDuringComposition` reports `launcher.launch(input, options)` within composition (see the Compose runtime
  ports in [Rules](rules.md#androidx-library-lint-checks) for what counts), where `launcher` is proven to be a
  `ManagedActivityResultLauncher`: declared as one or initialized by `rememberLauncherForActivityResult`. As in Lint,
  only the two-argument `launch` is `ManagedActivityResultLauncher`'s own method; `launch(input)` resolves to
  `ActivityResultLauncher` in `androidx.activity.result` and is not reported.
- `NoCollectCallFound` reports a `PredictiveBackHandler` `onBack` lambda (trailing, named or the second positional
  argument) whose progress parameter is unused (at the
  parameter, or at the lambda for an unused `it`), or in which no name is `collect`, `collectLatest` or
  `collectIndexed`. Like Lint, the second case is reported at the lambda's first name reference with the message
  "You must call collect() on Flow null".
- `ContextCastToActivity` reports `LocalContext.current as Activity` (or `as?`), with `LocalContext` from
  `androidx.compose.ui.platform`, at the cast. The type counts as an Activity when it is `android.app.Activity`,
  `ComponentActivity`, `FragmentActivity`, `AppCompatActivity`, a same-file subclass of one, or any other class whose
  name ends in `Activity`. As in Lint, `(LocalContext.current) as Activity` and a variable holding
  `LocalContext.current` are not reported.

## Compose material and material3

`androidx.compose.material:material-android` 1.13.0 and `androidx.compose.material3:material3-android` 1.5.0
(Kotlin, and Java for the import check):

- `UnusedMaterialScaffoldPaddingParameter` and `UnusedMaterial3ScaffoldPaddingParameter` report each parameter of a
  `Scaffold` content lambda that the lambda never refers to, at the parameter, or at the lambda for an unused implicit
  `it`: `Scaffold { Text("x") }` and `Scaffold { padding -> Text("x") }`, also through an import alias. A name inside
  an inner lambda that declares a parameter of the same name does not count, nor does `it` inside an inner lambda with
  its own `it`. Krit knows an inner lambda has its own `it` when it is passed to a one-parameter lambda function such
  as `forEach`, `let`, `map` or `LazyListScope.items`, or to a same-file function whose parameter type takes one
  value; any other inner lambda is assumed not to rebind `it`, so krit stays quiet where Lint may report. Destructured
  parameters are skipped, as in Lint. The content must be the trailing lambda or `content = { }`.
- `ConflictingOnColor` reports the 'on' colors of a `Colors(...)`, `lightColors(...)` or `darkColors(...)` call when
  background colors with the same value (`primary`, `primaryVariant`, `secondary`, `secondaryVariant`,
  `background`, `surface`, `error`) do not share one 'on' color, at each explicitly passed 'on' color. Values are
  compared by source text after following a same-file `val`/`var` to its initializer (a local `var` to its last
  assignment before the call), as Lint does; `Color(0xFF000000)` and `Color( 0xFF000000 )` differ. A background
  whose 'on' color is left to the library default conflicts with every explicit 'on' color in its group.
- `UsingMaterialAndMaterial3Libraries` reports an `androidx.compose.material` import (a class, member, star or static
  import) outside the `icons`, `ripple` and `pullrefresh` packages and the `ExperimentalMaterialApi` import, once the
  code uses material3: the same file imports from `androidx.compose.material3`, or the build file of the file's own
  Gradle module (the nearest directory with a `build.gradle(.kts)`) declares `androidx.compose.material3:material3`.
  As in Lint, which runs the check only in modules that depend on material3, a material-only module of a build whose
  other modules use material3 is not reported, however the other modules declare their dependencies. A material3
  dependency added by a convention plugin or arriving through another project module is not seen; one read from a
  Bazel `MODULE.bazel`/`WORKSPACE` counts for the whole build.
  Fully qualified references such as `androidx.compose.material.Button()` are not reported, as in Lint.

## ConstraintLayout Compose

`androidx.constraintlayout:constraintlayout-compose-android` 1.1.2 (Kotlin, production sources). Like the detector,
the checks compare source text. In UAST a lambda's last statement is an implicit return, which the detector's
statement scans skip, so the port skips it too: a `linkTo` (or a `constrain { }` block) written last in its lambda is
not looked at.

- `IncorrectReferencesDeclaration` reports `val (a, b) = createRefsFor("a", "b", "c")` when the number of string ids
  differs from the destructured variables, at `createRefsFor`, when its implicit receiver is a `ConstraintSetScope` or
  `MotionSceneScope` (inside `ConstraintSet { }` / `MotionScene { }`, or an extension on the scope), as Lint
  requires. A spread array or a non-string id is not counted.
- `IncorrectMatchParentUsage` reports `width = Dimension.matchParent` (or `height`) in a `constrain(ref) { }` block
  that also links the same axis to something other than the matching parent anchor (`start.linkTo(button.end)`,
  `centerHorizontallyTo(button)`), at `Dimension.matchParent`.
- `IncorrectChainMarginsUsage` reports an element of `createHorizontalChain(...)` / `createVerticalChain(...)`
  without `withChainParams` whose `constrain` block links it, with a margin, to its neighbour's opposite anchor (or
  the parent's same anchor at the ends), at the element.
## Startup

`androidx.startup:startup-runtime` 1.2.0 (Kotlin and Java). Both checks see every main-source class at once, so an
`Initializer` that extends a base class declared in another file is found; they run in the project scopes, whose
results are cached on the whole source set.

- `EnsureInitializerNoArgConstr` reports an `Initializer` that cannot be created without arguments, at the class: a
  Kotlin class whose primary and secondary constructors all take parameters (a primary constructor whose parameters
  all have defaults counts as a no-argument one, as it compiles to one, and so does an `@JvmOverloads` secondary
  constructor whose parameters all have defaults), or a Java class whose constructors all take parameters. Comments
  in a parameter list are not parameters. Interfaces are skipped.
- `EnsureInitializerMetadata` reports an `Initializer` (abstract ones too, as in Lint; interfaces are skipped) that no
  manifest `<meta-data android:name="…" android:value="androidx.startup">` (or `@string/androidx_startup`) names and no
  class literal in an `Initializer`'s `dependencies()` reaches, at the class. As in Lint, the `<meta-data>` name must
  be the fully qualified class name, and test-artifact (`androidTest`, `test`) and generated `build/` manifests are not
  read.
## Compose animation

`androidx.compose.animation:animation-android` and `animation-core-android` 1.13.0 (Kotlin). The unused-parameter
checks share the Scaffold checks' reference and shadowing rules above, and resolve import aliases
(`import androidx.compose.animation.Crossfade as Fade`) as Lint does.

- `UnusedCrossfadeTargetStateParameter` reports an unused parameter of the first lambda passed to `Crossfade`, as
  Lint does: the content lambda, or the `contentKey` lambda of `Transition.Crossfade` when it comes first.
- `UnusedContentLambdaTargetStateParameter` and `UnusedTargetStateInContentKeyLambda` report an unused parameter of
  `AnimatedContent`'s `content` lambda (trailing or `content = { }`) and of its `contentKey = { }` lambda.
- `UnusedTransitionTargetStateParameter` reports an unused state parameter of a `Transition.animate*` call
  (`animateFloat`, `animateDp`, `animateColor`, `animateValue` and the other library extensions) in the trailing or
  `targetValueByState = { }` lambda, and of any one-parameter lambda of a same-file `animate*` extension. The
  receiver must be proven to be an `androidx.compose.animation.core.Transition`: a variable declared as one or
  initialized by `updateTransition`/`rememberTransition`, the receiver of a `Transition` extension (unless a
  `with`/`apply`/`run` lambda in between brings a closer implicit receiver), or
  `AnimatedVisibilityScope.transition` inside an `AnimatedVisibility` or `AnimatedContent` lambda. Lint checks
  every `animate*` call on a resolved `Transition`.
- `UnusedSharedTransitionModifierParameter` reports an unused `Modifier` parameter of a `SharedTransitionScope { }`
  content lambda (trailing, `content = { }`, or the positional `SharedTransitionScope({ })`). Lint's quick fix, which moves the Modifier onto the single top-level layout composable, is not
  ported.
- `ConstantContentStateKeyInItemsCall` reports `rememberSharedContentState(key)` with a constant key (literals, `+`,
  same-file constants and local `val`s, as UAST's `evaluate()` folds them; a `var` or a string template with a
  variable is not constant) passed to `sharedElement` or
  `sharedBounds`, inside a composable call, inside a LazyList `items` or `itemsIndexed` call from an androidx
  foundation package (Compose, TV or Wear). As in Lint, each step is the nearest call that has the previous one in
  its arguments or lambda, so a call further along the Modifier chain (`Modifier.sharedElement(…).then(…)`) is not
  a step, and `Box(Modifier.sharedElement(…)) { }` counts as one `Box` call with or without its trailing lambda. The
  standalone Lint CLI reports no call of material's `Card`, which krit treats as the composable it is. The composable must be a same-file `@Composable` function or an upper-case function imported from
  `androidx.compose`.
- `DisallowLookaheadAnimationVisualDebug` reports `LookaheadAnimationVisualDebugging { }` and
  `CustomizedLookaheadAnimationVisualDebugging { }` in production sources (not tests, as in Lint). The quick fix that
  unwraps the content is not ported.
- `ArcAnimationSpecTypeIssue` reports `ArcAnimationSpec<T>()` with an explicit type argument other than `Offset`,
  `IntOffset` or `DpOffset`, at the constructor name, in production sources. A same-file `typealias` is expanded, as
  Lint's `canonicalText` does (`typealias Pos = Offset` is not reported). An inferred type argument is not
  checked, as in Lint.

## WebKit and UiAutomator

`androidx.webkit:webkit` 1.18.0 (Kotlin and Java) and `androidx.test.uiautomator:uiautomator` 2.4.1 (Kotlin),
production sources only, as Lint's `JAVA_FILE_SCOPE`. The checks ship in those AARs, so, as in Lint, each rule runs
only when the project declares its artifact:

- `MissingOnRenderProcessGone` reports a concrete class or object literal extending `WebViewClient` or
  `WebViewClientCompat` (abstract and sealed classes excepted) with no two-parameter `onRenderProcessGone` in itself
  or a same-file superclass, at its name,
  and a direct `WebViewClient()` / `WebViewClientCompat()` construction, at the call. A superclass declared in
  another file is not followed (the per-file cache is keyed by the file's content, #1778), so such a class is not
  reported. Lint reports a construction inside a Kotlin property assignment twice; krit reports it once.
- `CrossOriginIsolatedAllowlist` reports `setCrossOriginIsolatedAllowlist(…)` (or the Kotlin property assignment) on
  an `androidx.webkit.Profile`, proven by its declaration, a `with(profile)`/`apply`/`run` block, or a
  `getOrCreateProfile`/`getProfile` call.
- `DontUseAccessibilityNodeInfoGetText` reports `text` read on the `AccessibilityNodeInfo` of a UiAutomator element
  predicate (`onElement { text == "Ok" }`, implicitly or through `this`), or on a value declared as one, at `text`;
  a named-argument label such as `matches(text = "Ok")` is not a read.
  The shipped detector matches only the pre-release names `onView`, `onViews` and `onViewOrNull`, which the released
  2.4 API renamed to `onElement`, `onElements` and `onElementOrNull`, so it no longer fires; krit checks both sets.
