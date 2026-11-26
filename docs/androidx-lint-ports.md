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
