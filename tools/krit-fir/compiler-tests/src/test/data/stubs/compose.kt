// Compiler-test source stubs; never packaged in the production artifact.
package androidx.compose.runtime

import kotlin.coroutines.CoroutineContext
import kotlin.coroutines.EmptyCoroutineContext
import kotlin.reflect.KProperty
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.StateFlow

@MustBeDocumented
@Target(
    AnnotationTarget.FUNCTION,
    AnnotationTarget.TYPE,
    AnnotationTarget.TYPE_PARAMETER,
    AnnotationTarget.PROPERTY_GETTER,
)
@Retention(AnnotationRetention.BINARY)
annotation class Composable

@MustBeDocumented
@Target(AnnotationTarget.TYPE)
@Retention(AnnotationRetention.BINARY)
annotation class DisallowComposableCalls

@MustBeDocumented
@Target(AnnotationTarget.FUNCTION, AnnotationTarget.PROPERTY_GETTER)
@Retention(AnnotationRetention.BINARY)
annotation class ReadOnlyComposable

@MustBeDocumented
@Target(AnnotationTarget.FUNCTION, AnnotationTarget.PROPERTY_GETTER)
@Retention(AnnotationRetention.BINARY)
annotation class NonRestartableComposable

@MustBeDocumented
@Target(AnnotationTarget.ANNOTATION_CLASS)
@Retention(AnnotationRetention.BINARY)
annotation class StableMarker

@MustBeDocumented
@Target(
    AnnotationTarget.CLASS,
    AnnotationTarget.FUNCTION,
    AnnotationTarget.PROPERTY_GETTER,
    AnnotationTarget.PROPERTY,
)
@Retention(AnnotationRetention.BINARY)
@StableMarker
annotation class Stable

@MustBeDocumented
@Target(AnnotationTarget.CLASS)
@Retention(AnnotationRetention.BINARY)
@StableMarker
annotation class Immutable

// ---- remember ----

@Composable
inline fun <T> remember(crossinline calculation: @DisallowComposableCalls () -> T): T = TODO()

@Composable
inline fun <T> remember(key1: Any?, crossinline calculation: @DisallowComposableCalls () -> T): T = TODO()

@Composable
inline fun <T> remember(key1: Any?, key2: Any?, crossinline calculation: @DisallowComposableCalls () -> T): T =
    TODO()

@Composable
inline fun <T> remember(
    key1: Any?,
    key2: Any?,
    key3: Any?,
    crossinline calculation: @DisallowComposableCalls () -> T,
): T = TODO()

@Composable
inline fun <T> remember(vararg keys: Any?, crossinline calculation: @DisallowComposableCalls () -> T): T = TODO()

@Composable
fun <T> rememberUpdatedState(newValue: T): State<T> = TODO()

@Composable
inline fun rememberCoroutineScope(
    crossinline getContext: @DisallowComposableCalls () -> CoroutineContext = { EmptyCoroutineContext },
): CoroutineScope = TODO()

// ---- state ----

@Stable
interface State<out T> {
    val value: T
}

@Stable
interface MutableState<T> : State<T> {
    override var value: T

    operator fun component1(): T

    operator fun component2(): (T) -> Unit
}

@Stable
interface IntState : State<Int> {
    override val value: Int
        get() = intValue

    val intValue: Int
}

@Stable
interface MutableIntState : IntState, MutableState<Int> {
    override var value: Int
        get() = intValue
        set(value) {
            intValue = value
        }

    override var intValue: Int
}

@Stable
interface FloatState : State<Float> {
    override val value: Float
        get() = floatValue

    val floatValue: Float
}

@Stable
interface MutableFloatState : FloatState, MutableState<Float> {
    override var value: Float
        get() = floatValue
        set(value) {
            floatValue = value
        }

    override var floatValue: Float
}

interface SnapshotMutationPolicy<T> {
    fun equivalent(a: T, b: T): Boolean
}

fun <T> structuralEqualityPolicy(): SnapshotMutationPolicy<T> = TODO()

fun <T> referentialEqualityPolicy(): SnapshotMutationPolicy<T> = TODO()

fun <T> mutableStateOf(value: T, policy: SnapshotMutationPolicy<T> = structuralEqualityPolicy()): MutableState<T> =
    TODO()

fun mutableIntStateOf(value: Int): MutableIntState = TODO()

fun mutableFloatStateOf(value: Float): MutableFloatState = TODO()

fun <T> mutableStateListOf(vararg elements: T): MutableList<T> = TODO()

fun <T> derivedStateOf(calculation: () -> T): State<T> = TODO()

fun <T> derivedStateOf(policy: SnapshotMutationPolicy<T>, calculation: () -> T): State<T> = TODO()

fun <T> snapshotFlow(block: () -> T): Flow<T> = TODO()

// Property delegation: `val x by state` / `var x by mutableState`.
@Suppress("NOTHING_TO_INLINE")
inline operator fun <T> State<T>.getValue(thisObj: Any?, property: KProperty<*>): T = TODO()

@Suppress("NOTHING_TO_INLINE")
inline operator fun <T> MutableState<T>.setValue(thisObj: Any?, property: KProperty<*>, value: T) {
    TODO()
}

@Suppress("NOTHING_TO_INLINE")
inline operator fun IntState.getValue(thisObj: Any?, property: KProperty<*>): Int = TODO()

@Suppress("NOTHING_TO_INLINE")
inline operator fun MutableIntState.setValue(thisObj: Any?, property: KProperty<*>, value: Int) {
    TODO()
}

@Suppress("NOTHING_TO_INLINE")
inline operator fun FloatState.getValue(thisObj: Any?, property: KProperty<*>): Float = TODO()

@Suppress("NOTHING_TO_INLINE")
inline operator fun MutableFloatState.setValue(thisObj: Any?, property: KProperty<*>, value: Float) {
    TODO()
}

// collectAsState lives in runtime and takes the flow as its receiver.
@Composable
fun <T> StateFlow<T>.collectAsState(context: CoroutineContext = EmptyCoroutineContext): State<T> = TODO()

@Composable
fun <T : R, R> Flow<T>.collectAsState(initial: R, context: CoroutineContext = EmptyCoroutineContext): State<R> =
    TODO()

// ---- effects ----

class DisposableEffectScope {
    inline fun onDispose(crossinline onDisposeEffect: () -> Unit): DisposableEffectResult = TODO()
}

interface DisposableEffectResult {
    fun dispose()
}

@Composable
@NonRestartableComposable
fun DisposableEffect(key1: Any?, effect: DisposableEffectScope.() -> DisposableEffectResult) {
    TODO()
}

@Composable
@NonRestartableComposable
fun DisposableEffect(key1: Any?, key2: Any?, effect: DisposableEffectScope.() -> DisposableEffectResult) {
    TODO()
}

@Composable
@NonRestartableComposable
fun DisposableEffect(vararg keys: Any?, effect: DisposableEffectScope.() -> DisposableEffectResult) {
    TODO()
}

// The keyless overload exists only to turn `LaunchedEffect { }` into a
// compile error instead of resolving to the vararg overload with no keys.
private const val LaunchedEffectNoParamError =
    "LaunchedEffect must provide one or more 'key' parameters that define the identity of " +
        "the LaunchedEffect and determine when its previous effect coroutine should be cancelled " +
        "and a new effect launched for the new key."

@Deprecated(LaunchedEffectNoParamError, level = DeprecationLevel.ERROR)
@Composable
fun LaunchedEffect(block: suspend CoroutineScope.() -> Unit): Unit = error(LaunchedEffectNoParamError)

@Composable
@NonRestartableComposable
fun LaunchedEffect(key1: Any?, block: suspend CoroutineScope.() -> Unit) {
    TODO()
}

@Composable
@NonRestartableComposable
fun LaunchedEffect(key1: Any?, key2: Any?, block: suspend CoroutineScope.() -> Unit) {
    TODO()
}

@Composable
@NonRestartableComposable
fun LaunchedEffect(key1: Any?, key2: Any?, key3: Any?, block: suspend CoroutineScope.() -> Unit) {
    TODO()
}

@Composable
@NonRestartableComposable
fun LaunchedEffect(vararg keys: Any?, block: suspend CoroutineScope.() -> Unit) {
    TODO()
}

@Composable
@NonRestartableComposable
fun SideEffect(effect: () -> Unit) {
    TODO()
}

// ---- composition locals ----

abstract class CompositionContext

sealed class CompositionLocal<T>(defaultFactory: () -> T) {
    val current: T
        @Composable
        @ReadOnlyComposable
        get() = TODO()
}

abstract class ProvidableCompositionLocal<T> internal constructor(defaultFactory: () -> T) :
    CompositionLocal<T>(defaultFactory) {
    infix fun provides(value: T): ProvidedValue<T> = TODO()

    infix fun providesDefault(value: T): ProvidedValue<T> = TODO()
}

class ProvidedValue<T> internal constructor()

fun <T> compositionLocalOf(
    policy: SnapshotMutationPolicy<T> = structuralEqualityPolicy(),
    defaultFactory: () -> T,
): ProvidableCompositionLocal<T> = TODO()

fun <T> staticCompositionLocalOf(defaultFactory: () -> T): ProvidableCompositionLocal<T> = TODO()

@Composable
fun CompositionLocalProvider(vararg values: ProvidedValue<*>, content: @Composable () -> Unit) {
    TODO()
}

@Composable
fun <T> key(vararg keys: Any?, block: @Composable () -> T): T = TODO()
