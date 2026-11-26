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

## Opt-in markers

`androidx.annotation:annotation-experimental` 1.6.0, on annotation declarations in production sources:

- `ExperimentalAnnotationRetention` (Kotlin and Java) reports an opt-in marker (an annotation class meta-annotated
  with `kotlin.RequiresOptIn`, `kotlin.Experimental`, `androidx.annotation.RequiresOptIn` or
  `androidx.annotation.experimental.Experimental`) whose retention is not the expected one, at its name: Kotlin
  markers need an explicit `kotlin.annotation.Retention(AnnotationRetention.BINARY)` (the default is `RUNTIME`), Java
  markers the default `CLASS`. As in Lint, a Kotlin `java.lang.annotation.Retention` does not count, and the message
  calls a retention equal to the default "default (`…`)".
- `WrongRequiresOptIn` (Kotlin) reports a Kotlin annotation class marked with `androidx.annotation.RequiresOptIn`,
  whose opt-in policy the Kotlin compiler does not enforce; use `kotlin.RequiresOptIn`.

Annotation names resolve through imports (aliases and star imports), the file's package and Kotlin's default
`kotlin.*` / `kotlin.annotation.*` imports.

- `UnsafeOptInUsageError` and `UnsafeOptInUsageWarning` (Kotlin and Java) report a use of an API marked with an
  opt-in marker declared in the project, by the marker's level, without opting in: a call, constructor call or member
  call (including through a Kotlin file facade such as `ApiKt.fn()` from Java) whose target carries the marker,
  directly or through its class. A member call's receiver type comes from a same-file declaration (a Kotlin or Java
  local, parameter or field) or a class name. An unqualified Kotlin call that binds to a local function, a lambda or
  function parameter, a local or class property, or an enclosing class's companion object is not the marked top-level
  function. Lint also reports a Java type reference to a marked class (`MarkedClass field;`); krit does not yet (#1826).
  The use is accepted when an enclosing declaration (or the file, `@file:OptIn`) is
  annotated with the marker itself or opts in with an annotation the marker accepts: androidx markers need
  `androidx.annotation.OptIn` (a `kotlin.OptIn` does not count, as in Lint), Kotlin markers accept either. As in Lint,
  Kotlin code is only checked against androidx markers, since the Kotlin compiler enforces `kotlin.RequiresOptIn`
  itself; Java code is checked against both. The message is the marker's `message`, or Lint's default. Both run
  once per scan over every source file (markers, marked APIs and uses live in different files). Markers declared in
  libraries (CameraX, Media3, …) are not known, because their class files are not read, and the lint.xml `opt-in`
  option is not read either.

## Lifecycle

`androidx.lifecycle` 2.11.1 (`lifecycle-runtime`, `lifecycle-runtime-compose`, `lifecycle-runtime-testing`,
`lifecycle-viewmodel-compose`), Kotlin:

- `LifecycleCurrentStateInComposition` reports `….currentState` read in composition on a `Lifecycle`: a value
  declared or initialized as a `Lifecycle` or `LifecycleRegistry`, or `owner.lifecycle` where the owner is a known
  `LifecycleOwner` (Activity, Fragment, `LocalLifecycleOwner.current`, …) or a same-file class whose `lifecycle` is a
  `Lifecycle`. An owner whose type is not in the file falls back to the property name, which `LifecycleOwner`
  defines; a same-file class whose `lifecycle` is something else is not reported. Lint's quick fix
  (`currentStateAsState().value` plus an import) is not ported.
- `ViewModelConstructorInComposable` reports a constructor call of an `androidx.lifecycle.ViewModel` subclass declared
  in the same file, in composition, at the class name. The per-file findings cache is keyed by the file's own content
  (#1778), so a ViewModel declared in another file is not checked yet.
- `TestLifecycleOwnerInCoroutine` reports a `kotlinx.coroutines.test.runTest { }` whose body's last assignment of a
  constant is `….currentState = Lifecycle.State.X` (the blocking setter hangs inside a coroutine; use
  `setCurrentState`), at `runTest`. It follows the detector source and its tests: the Lint CLI 9.4 reported nothing
  for this ID against the released `lifecycle-runtime-testing` and `kotlinx-coroutines-test` classes. As in the
  detector, the owner's type is not checked, and a later assignment of another constant clears the report.
- `RepeatOnLifecycleWrongUsage` reports `repeatOnLifecycle` reached from `onStart` or `onResume` of a Kotlin class
  extending `Fragment` or `androidx.core.app.ComponentActivity` (so also `ComponentActivity`, `FragmentActivity` and
  `AppCompatActivity`), proven from the file alone (an imported androidx supertype or a same-file base class, for the
  same cache reason), in production sources, with Lint's message naming the class and method. Unqualified and
  `this.` calls are followed into functions declared in the same file, each once per class, as Lint follows each
  method once; Lint also follows calls into other files. `super` calls are not followed.

## Lifecycle runtime and LiveData

`androidx.lifecycle:lifecycle-livedata-core` and `lifecycle-runtime` 2.11.1 (Kotlin, production sources):

- `NullSafeMutableLiveData` reports `live.value = x`, `live.setValue(x)` and `live.postValue(x)` on a LiveData whose
  type argument is non-nullable and not a type parameter: `null` ("Cannot set non-nullable LiveData value to
  `null`"), or a nullable value ("Expected non-nullable value"), at the value. The type argument comes from the
  receiver's same-file declaration (its declared `LiveData`/`MutableLiveData`/`MediatorLiveData` type, a
  `MutableLiveData<T>()` initializer, or the supertype of a same-file subclass). A nullable value is a name declared
  with a nullable type, unless an enclosing `if`/`when` condition null-checks it, it is inside the lambda of a safe
  call on it (`x?.let { }`), or an earlier statement proves it non-null (`!!`, `?: return`, a `?: run { }` block
  whose last statement exits (not `return@run`), `requireNotNull`, `require`/`check(x != null)`, `if (x == null || …) return`, an `isNullOrEmpty`/`isNullOrBlank` exit, or a
  subject-less `when` whose entries up to `x == null -> return` all exit), or a call of a same-file function with a
  nullable return type.
- `UnsafeLifecycleWhenUsage` reports View access in the `finally` block of a `try` that makes a suspend call, inside
  `lifecycle.whenStarted { }`, `whenCreated` or `whenResumed` (aliases resolved), at the access. Only the `try`
  clause is searched for suspend calls: same-file `suspend` functions, kotlinx.coroutines top-level suspend
  functions, and suspend members such as `first`/`await`/`send` on a value declared as a kotlinx.coroutines `Flow`,
  `Channel`, `Deferred` or `Job`. View access is a call or property on a value declared as an `android.view.View`
  subclass, followed into same-file functions three levels deep. Like the detector's visitor, both searches skip
  suspend lambdas (`launch { }`, `coroutineScope { }`, …) and declarations, local variables included. Only
  `finally` is checked and a `lifecycle.currentState.isAtLeast(…)` guard does not count (the detector looks for an
  `isAtLeast` on `Lifecycle` itself, which `Lifecycle.State` declares instead).
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
