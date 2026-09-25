# Compiler test source stubs

`KritFirProbe` adds two stub layers to each in-memory Kotlin compilation:

- **Kotlin stubs**: every `*.kt` file in this directory, compiled as Kotlin
  sources next to the test snippet.
- **Java stubs**: every `java/**/*.java` file, copied with its package
  directories and passed as a Java source root (`javaSourceRoots`). K2
  resolves them from source without javac.

Diagnostic tests and checker property tests both go through that probe, so new
declarations are available automatically and are never added to the
production artifact. The compilation has kotlin-stdlib and the JDK on its
classpath and nothing else: every Android, AndroidX, Compose, coroutines,
DI, logging, test, and networking type a checker needs comes from here.

A checker is only as trustworthy as these shapes. A stub that is looser than
the real API lets a checker pass its tests and then misfire (or never fire) on
real code, so the real SDK/library signature is always the ground truth.

## Which layer a type belongs in

Match the language the real library is written in, because FIR symbols differ
by origin and no harness check can detect the mismatch:

- **Java libraries go in `java/`.** The Android platform (`android.*`) and
  the Apache HTTP client it bundles (`org.apache.http.*`) are Java stubs.
  Only Java gives the real symbol shapes: a static member has no dispatch
  receiver and a callable id without a `Companion` segment
  (`android/view/View.VISIBLE`); getter/setter pairs become synthetic
  properties (`view.visibility` is a `FirSyntheticPropertySymbol` over
  `getVisibility`/`setVisibility`); and unannotated types are platform types.
  A Kotlin `object` or `companion object` imitation gets every one of these
  wrong.
- **Kotlin libraries stay in `*.kt`**: Compose, coroutines, Timber
  (`Timber.d` really is the companion `Forest`), OkHttp 4, Room 2.7 (companion
  constants), and the other Kotlin-first libraries.
- **Java-origin AndroidX types** (`RecyclerView.NO_POSITION`, `ViewCompat`,
  `ContextCompat`, `DiffUtil`, `Fragment`, `DialogFragment`, and so on) are
  still Kotlin stubs today. Convert a type to a Java stub when a rule first
  needs its Java-static or synthetic-property behavior; the smoke files for
  its package must keep compiling unchanged.

## Matching FIR symbols in checkers

Match on the callable id (`android/util/Log.d`) or on the owning class id plus
member name. Never decide on `dispatchReceiver == null`, `isStatic`, symbol
origin, or whether a property is synthetic: those differ between the Java
stubs, Kotlin stubs of Java-origin AndroidX types, and the real binaries, so a
checker keyed on them passes its tests and dies in production (or the
reverse).

## Layout rules

- **Kotlin: one package per file**, named from the package with dots replaced
  by underscores (`androidx.compose.ui.unit` -> `androidx_compose_ui_unit.kt`).
  `compose.kt` (`androidx.compose.runtime`), `coroutines.kt`
  (`kotlinx.coroutines`), `flow.kt` (`kotlinx.coroutines.flow`), and
  `lifecycle.kt` (`androidx.lifecycle`) predate the naming rule and keep their
  names.
- **Java: one public top-level class per file**, at the path matching its
  package (`java/android/view/View.java`); nested types stay nested as in the
  SDK (`View.OnClickListener`, `Build.VERSION`). Java stubs never reference
  Kotlin stubs: the Java layer compiles on its own with the JDK.
- **Never redeclare.** Search both layers and extend the existing declaration;
  a duplicate symbol breaks every compiler test.
- **One declaration or member per line**, in normal formatting: every class
  member, top-level function, and constant on its own line(s), bodies on their
  own lines. Many rule lanes add members to these files in parallel; dense
  one-line class bodies turn every addition into a merge conflict.
- **Never stub a JDK package.** The compilation already sees the real
  `java.*`/`javax.*` classes (for example `javax.net.ssl`); a stub in such a
  package shadows the JDK class, so real idioms stop type-checking and checkers
  that match JDK symbols see stub symbols instead. Packages that are not in the
  JDK (such as `javax.inject`) are fine.

`StubLibraryTest` enforces the smoke pairing, the Java package directories,
the no-JDK-package rule, a clean compile of the Kotlin stubs, and a javac
compile (`-proc:none`) of the Java layer, since kotlinc does not report Java
parse or type errors.

## Every stub has a smoke file

Each Kotlin stub `X.kt` pairs with `src/test/data/diagnostic/stubs/XSmoke.kt`;
each Java package pairs with `<package_with_underscores>Smoke.kt`
(`android.view` -> `android_viewSmoke.kt`). A smoke file compiles against the
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
  annotations with their real arguments on the declaration kinds their
  `@Target` allows;
- pin the Java shapes: `view.visibility = View.VISIBLE`, `Log.d(TAG, "x")`,
  `Toast.makeText(ctx, "x", Toast.LENGTH_SHORT).show()`,
  `if (Build.VERSION.SDK_INT >= 26)`, `startActivity(Intent(Intent.ACTION_VIEW))`,
  `setResult(Activity.RESULT_OK)`.

A smoke file that only declares `val x: T? = null` proves the type exists and
nothing about its shape. Keep smoke code away from existing checker triggers
(for example, keep hardcoded dispatchers out of class members so
InjectDispatcher stays silent). Smoke data is picked up by
`:compiler-tests:generateTests`; regenerate rather than editing generated
output.

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
- Modifiers: `abstract`/`final`/`protected` exactly as in the SDK or library,
  `suspend` and lambda receivers (`suspend CoroutineScope.() -> T`,
  `@Composable ColumnScope.() -> Unit`).
- Nullability: Java stubs leave types unannotated (platform types, the SDK
  norm). Kotlin stubs use the library's declared nullability.
- Annotations: real `@Target` and `@Retention`. In Kotlin stubs of Java
  annotations, map `METHOD` -> `FUNCTION`, `PROPERTY_GETTER`,
  `PROPERTY_SETTER`; `PARAMETER` -> `VALUE_PARAMETER`; `TYPE` -> `CLASS`;
  `ANNOTATION_TYPE` -> `ANNOTATION_CLASS`, with no extra targets (no
  `PROPERTY` unless the real annotation has it).
- Constants: only real compile-time constants may be constant. **Trap:**
  `Build.VERSION.SDK_INT` (and the other `Build` fields) are read at runtime in
  the SDK. The Java stub initializes them with non-constant expressions
  (`Integer.parseInt("0")`, `null`) so FIR does not constant-fold
  `SDK_INT >= 26`; never write `= 0`.
- Bodies are `throw new RuntimeException("Stub!")` (Java) or `TODO()`
  (Kotlin); stubs are never executed.

## Known approximations

- `android.os.Parcelable.describeContents`/`writeToParcel` are default
  methods because the kotlin-parcelize plugin (which generates them for
  `@Parcelize` classes) is not loaded here.
- Compose `Alignment`, `Arrangement.Horizontal/Vertical`, `ContentScale`,
  `Painter`, and `PaddingValues` omit layout members whose parameter types
  (`IntSize`, `LayoutDirection`, `DrawScope`) are not modeled.
- Kotlin stubs of Java-origin AndroidX classes model statics as `object`
  members (callable id matches Java, but FIR reports a dispatch receiver) or
  `companion object` members (callable id gains `Companion`, for example
  `Counter.Companion.builder` for Micrometer's Java interface static). See the
  matching rule above.
- Some SDK supertypes are not modeled (`CursorLoader`'s loader chain,
  `Settings.NameValueTable`, ICU `UFormat`); each is noted in its stub.
