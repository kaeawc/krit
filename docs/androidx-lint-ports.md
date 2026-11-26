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

## Lifecycle runtime and LiveData

`androidx.lifecycle:lifecycle-livedata-core` and `lifecycle-runtime` 2.11.1 (Kotlin, production sources):

- `NullSafeMutableLiveData` reports `live.value = x`, `live.setValue(x)` and `live.postValue(x)` on a LiveData whose
  type argument is non-nullable and not a type parameter: `null` ("Cannot set non-nullable LiveData value to
  `null`"), or a nullable value ("Expected non-nullable value"), at the value. The type argument comes from the
  receiver's same-file declaration (its declared `LiveData`/`MutableLiveData`/`MediatorLiveData` type, a
  `MutableLiveData<T>()` initializer, or the supertype of a same-file subclass). A nullable value is a name declared
  with a nullable type, unless an enclosing `if`/`when` condition null-checks it or an earlier statement proves it
  non-null (`!!`, `?: return`, `requireNotNull`, `if (x == null) return`), or a call of a same-file function with a
  nullable return type.
- `UnsafeLifecycleWhenUsage` reports View access in the `finally` block of a `try` that makes a suspend call, inside
  `lifecycle.whenStarted { }`, `whenCreated` or `whenResumed`, at the access. Suspend calls are same-file `suspend`
  functions and kotlinx.coroutines suspend functions; View access is a call or property on a value declared as an
  `android.view.View` subclass, followed into same-file functions three levels deep. As in the detector, only
  `finally` is checked and a `lifecycle.currentState.isAtLeast(…)` guard does not count (the detector looks for an
  `isAtLeast` on `Lifecycle` itself, which `Lifecycle.State` declares instead).
