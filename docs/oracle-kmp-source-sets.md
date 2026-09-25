# JVM oracle facts in Kotlin Multiplatform projects

Krit's Go rules scan every Kotlin file in a project, whatever its target. The
two JVM type oracles (`krit-fir`, the K2 compiler front end, and `krit-types`,
the Kotlin Analysis API) are different: they run a JVM compilation, so in a
Kotlin Multiplatform (KMP) project they compile only the source sets that
build for the JVM.

## Which files get oracle facts

The oracles compile each `kotlin`/`java` directory found under the scanned
paths, except the directories of non-JVM target source sets. A directory
shaped `src/<sourceSet>/kotlin` (or `java`) is dropped when `<sourceSet>`,
with a trailing `Main` or `Test` removed, is one of these target families, or
starts with one followed by an upper-case letter or digit:

`js`, `wasm`, `native`, `apple`, `ios`, `macos`, `tvos`, `watchos`, `linux`,
`mingw`, `androidNative`

So `jsMain`, `wasmJsMain`, `nativeTest`, `iosArm64Main`, `linuxX64Main`,
`jsAndWasmSharedMain`, and `androidNativeArm64Main` are dropped. Everything
else is kept:

- plain JVM and Android source sets (`main`, `test`, `debug`, `androidTest`, ...);
- `commonMain`, `commonTest`, `jvm*`, and `android*` (other than `androidNative*`);
- custom intermediate source sets, which compile on the JVM
  (`jvmAndroidMain`, `jvmCommonMain`, `concurrentMain`, `nonJsMain`,
  `desktopMain`, `serverMain`, ...);
- any name krit does not recognize, and any directory outside the
  `src/<sourceSet>/` layout.

Files in dropped source sets get no oracle facts. Rules treat a missing fact
as unknown: a nullable declaration is still checked from source, and no
redundancy rule claims a value is non-null without a fact that says so.
Plain JVM and Android projects compile exactly the directories they did
before.

## How krit-fir compiles common code

When a `commonMain` or `commonTest` directory is present, krit-fir compiles
the kept directories the way kotlinc compiles a KMP project's JVM target:
`-Xmulti-platform -Xexpect-actual-classes` with the common files passed as
`-Xcommon-sources`. Calls through an `expect` declaration then resolve, and
their warnings are reported. In a single flat compilation, every such call is
ambiguous between the `expect` and its `actual`, and loses its type and
warnings.

K2's command-line compiler has two levels, common and platform. Each kept
directory goes to one of them. The rule below is what compiled without errors
under kotlinc for each shape:

| Directory | Level |
| --- | --- |
| `commonMain`, `commonTest` | common |
| Other `src/<sourceSet>/kotlin` declaring an `expect` and no `actual` (for example `expect` in `concurrentMain` with its `actual` in `jvmMain`) | common |
| Any directory declaring an `actual`, including one that also declares a new `expect` | platform |
| `main`, `test`, and directories outside the `src/<sourceSet>/` layout | platform |

Why each shape goes where it does:

- If a set declares an `expect` and is compiled as platform, kotlinc reports
  "expect and corresponding actual are declared in the same module", and
  every call through the `expect` becomes ambiguous. Compiled as common, it
  gets every warning. JVM-only APIs such as `java.io.File` still resolve at
  the common level.
- If a set holds an `actual` for `commonMain` and is compiled as common, the
  `actual` lands in the common module beside its `expect`, which breaks
  `commonMain` itself. Compiled as platform, it gets every warning.
- A set holding both an `actual` and a new `expect` can't be modeled with
  two levels. Compiling it as platform keeps the errors inside that set
  rather than spreading them to `commonMain`.

The `expect` and `actual` modifiers are read from source, ignoring comments
and string literals.

## Known limits

- **krit-types does not resolve `expect` calls.** It analyzes all the kept
  directories as one Analysis API source module. Dropping the non-JVM
  source sets removes the conflicting JS and Native `actual`s, but calls
  through an `expect` stay unresolved. On the regression fixture in
  `tests/parity/kmp_source_set_parity_test.go`, krit-types reports the three
  warnings that don't go through an `expect`. krit-fir and kotlinc report
  all six. Fixing this needs one Analysis API module per source set, with
  dependencies between them.
- **Two JVM-side actuals of one `expect`.** If both `jvmMain` and
  `androidMain` provide an `actual`, both are compiled in the single platform
  compilation. Calls from `commonMain` still resolve through the `expect`.
  Calls from platform code to that function are ambiguous and lose their
  facts. Android is not compiled as its own target.
- **Deeper hierarchies.** Only two levels are modeled. A common-level
  intermediate set that uses declarations from a platform-level set, such
  as another intermediate set holding `actual`s, doesn't resolve those
  declarations.
- **All modules share one compilation.** Every Gradle module's kept
  directories are compiled together, as before.
- **FIR checker rules (`--fir`) are unchanged.** They compile the scanned
  files directly rather than the oracle source directories, so none of the
  above applies to them.
