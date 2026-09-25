# Compiler test source stubs

`KritFirProbe` copies every `*.kt` file in this directory into each in-memory
Kotlin compilation. Diagnostic tests and checker property tests both use that
probe, so new declarations are available automatically and are never added to
the production artifact. The compilation has kotlin-stdlib and the JDK on its
classpath and nothing else: every Android, AndroidX, Compose, coroutines,
DI, logging, test, and networking type a checker needs comes from here.

A checker is only as trustworthy as these shapes. A stub that is looser than
the real API lets a checker pass its tests and then misfire (or never fire) on
real code, so the real SDK/library signature is always the ground truth.

## Layout rules

- **One package per file**, named from the package with dots replaced by
  underscores (`androidx.compose.ui.unit` -> `androidx_compose_ui_unit.kt`).
  `compose.kt` (`androidx.compose.runtime`), `coroutines.kt`
  (`kotlinx.coroutines`), `flow.kt` (`kotlinx.coroutines.flow`), and
  `lifecycle.kt` (`androidx.lifecycle`) predate the naming rule and keep their
  names.
- **Never redeclare.** Search the directory and extend the existing
  declaration; a duplicate symbol breaks every compiler test.
- **One declaration or member per line**, in normal Kotlin formatting: every
  class member, top-level function, and constant on its own line(s), bodies on
  their own lines. Many rule lanes add members to these files in parallel;
  dense one-line class bodies turn every addition into a merge conflict.
- **Never stub a JDK package.** The compilation already sees the real
  `java.*`/`javax.*` classes (for example `javax.net.ssl`); a stub in such a
  package shadows the JDK class, so real idioms stop type-checking and checkers
  that match JDK symbols see stub symbols instead. Packages that are not in the
  JDK (such as `javax.inject`) are fine. `StubLibraryTest` enforces this.

## Every stub file has a smoke file

For each `X.kt` here there is a `src/test/data/diagnostic/stubs/XSmoke.kt`
(`StubLibraryTest` enforces the pairing). A smoke file compiles against the
stubs and must produce zero krit diagnostics and zero compiler errors. It must
exercise the API the way real code does, not just name a type:

- subclass the component and override its real callbacks
  (`class A : AppCompatActivity() { override fun onCreate(b: Bundle?) { super.onCreate(b) } }`);
- assign instances to their real supertypes (`val owner: LifecycleOwner = activity`);
- call the canonical idioms: `launch(Dispatchers.IO) { withContext(...) { } }`,
  `flow.collect { }` inside a suspend function,
  `var x by remember { mutableStateOf(0) }`,
  `LazyColumn { items(list, key = { it.id }) { } }`,
  `@Query("SELECT ...") fun q(): List<X>` in a `@Dao interface`,
  `Timber.tag("t").d("x")`, `Color(0xFF000000)`, `16.dp`,
  `Build.VERSION.SDK_INT >= 26`, annotations with their real arguments on the
  declaration kinds their `@Target` allows.

A smoke file that only declares `val x: T? = null` proves the type exists and
nothing about its shape. Keep smoke code away from existing checker triggers
(for example, keep hardcoded dispatchers out of class members so
InjectDispatcher stays silent). Smoke data is picked up by
`:compiler-tests:generateTests`; regenerate rather than editing generated
output. `StubLibraryTest` also compiles the stub files as requested sources, so
an error inside a stub fails loudly instead of hiding behind the smoke files.

## Fidelity expectations

- Real supertype chains and nested generic types (`AppCompatActivity :
  FragmentActivity : androidx.activity.ComponentActivity :
  androidx.core.app.ComponentActivity : Activity : ContextThemeWrapper :
  ContextWrapper : Context`; `RecyclerView.Adapter<VH>`; a dispatcher is a
  `CoroutineContext.Element`).
- Declaration kind: interface vs abstract/open/final class vs object vs
  annotation vs value class vs factory function. `Color(0xFF...)` and
  `RoundedCornerShape(8.dp)` are top-level factory functions, not
  constructors; `Channel` and `Flow` are interfaces; `Role` and `Dp` are value
  classes.
- Modifiers: `open`/`abstract` where subclasses override, `protected` where
  the SDK is protected, `suspend` and lambda receivers
  (`suspend CoroutineScope.() -> T`, `@Composable ColumnScope.() -> Unit`).
- Nullability from the SDK annotations. Kotlin source cannot express Java
  platform types; where the SDK's annotation only warns in real Kotlin and real
  call sites dereference the result (`findViewById`, `Activity.intent`), the
  stub uses the shape real code writes and says so in a comment.
- Annotations: Java `@Target` mapped to Kotlin (`METHOD` -> `FUNCTION`,
  `PROPERTY_GETTER`, `PROPERTY_SETTER`; `PARAMETER` -> `VALUE_PARAMETER`;
  `TYPE` -> `CLASS`; `ANNOTATION_TYPE` -> `ANNOTATION_CLASS`) with no extra
  targets (no `PROPERTY` unless the real annotation has it, so an unqualified
  annotation on a Kotlin property lands where it really lands), real
  `@Retention`, `@Repeatable`, real parameters and defaults.
- Constants: `const val` only for real compile-time constants.
  `Build.VERSION.SDK_INT` is a runtime `@JvmField val`, not `const`.
- Java getter/setter pairs that app code uses as properties
  (`view.visibility = View.GONE`) are Kotlin properties; Java getters that app
  code *overrides* (`Drawable.getOpacity`, `Adapter.getItemCount`) stay
  functions, so test code calls them as `getItemCount()`.
- Bodies are `TODO()` (or `error(...)`); stubs are never executed.

## `object` vs Java statics

FIR gives a Java static member no dispatch receiver, and its callable id is
`package/Class.member` (for example `android/widget/Toast.makeText`). Kotlin
has no statics, so each stub picks one of two approximations:

- **`object`** for Java classes that app code never constructs or subclasses
  (`Log`, `Toast`, `Uri`, `Build`, `PackageManager`, `LayoutInflater`).
  The callable id matches the Java one (`android/util/Log.d`), but FIR reports
  the object as the dispatch receiver. Checkers must match on the callable id,
  not on "no dispatch receiver".
- **`companion object`** for classes that app code does construct or subclass
  (`Context.MODE_PRIVATE`, `View.VISIBLE`, `Intent.ACTION_VIEW`,
  `Service.START_STICKY`, Java-interface statics such as `Span.current()`).
  Here the callable id gains a `Companion` segment
  (`android/view/View.Companion.VISIBLE`) that the real Java static does not
  have. A checker keyed on such a static must accept both forms, or match on
  the owning class plus member name.

Kotlin libraries are modeled exactly: `Timber.d(...)` really is the
companion `Forest` (`timber/log/Timber.Forest.d`), and Room 2.7's
`OnConflictStrategy.REPLACE` really is a companion constant.

## Known approximations

- `android.os.Parcelable.describeContents`/`writeToParcel` carry default
  bodies because the kotlin-parcelize plugin (which generates them for
  `@Parcelize` classes) is not loaded here.
- Compose `Alignment`, `Arrangement.Horizontal/Vertical`, `ContentScale`,
  `Painter`, and `PaddingValues` omit layout members whose parameter types
  (`IntSize`, `LayoutDirection`, `DrawScope`) are not modeled.
- Platform types: `TextView.text` is a non-null `CharSequence` (reads such as
  `text.toString()` dominate), so assigning a nullable value needs `?: ""` in
  test code even though real Kotlin accepts it.
- Java static inheritance (`ActivityCompat.checkSelfPermission` resolving to
  `ContextCompat`) cannot be expressed; only members declared on the class
  itself exist.
