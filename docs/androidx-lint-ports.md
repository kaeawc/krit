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
