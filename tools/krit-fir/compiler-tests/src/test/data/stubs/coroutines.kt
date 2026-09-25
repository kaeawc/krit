// Compiler-test source stubs; never packaged in the production artifact.
package kotlinx.coroutines

import kotlin.coroutines.AbstractCoroutineContextElement
import kotlin.coroutines.Continuation
import kotlin.coroutines.ContinuationInterceptor
import kotlin.coroutines.CoroutineContext
import kotlin.coroutines.EmptyCoroutineContext
import kotlin.time.Duration

@RequiresOptIn(level = RequiresOptIn.Level.WARNING, message = "This is a delicate API and its use requires care.")
@Retention(AnnotationRetention.BINARY)
@Target(AnnotationTarget.CLASS, AnnotationTarget.PROPERTY, AnnotationTarget.FUNCTION, AnnotationTarget.TYPEALIAS)
annotation class DelicateCoroutinesApi

@RequiresOptIn(level = RequiresOptIn.Level.WARNING)
@Retention(AnnotationRetention.BINARY)
@Target(
    AnnotationTarget.CLASS,
    AnnotationTarget.ANNOTATION_CLASS,
    AnnotationTarget.PROPERTY,
    AnnotationTarget.FIELD,
    AnnotationTarget.LOCAL_VARIABLE,
    AnnotationTarget.VALUE_PARAMETER,
    AnnotationTarget.CONSTRUCTOR,
    AnnotationTarget.FUNCTION,
    AnnotationTarget.PROPERTY_GETTER,
    AnnotationTarget.PROPERTY_SETTER,
    AnnotationTarget.TYPEALIAS,
)
annotation class ExperimentalCoroutinesApi

typealias CancellationException = java.util.concurrent.CancellationException

typealias CompletionHandler = (cause: Throwable?) -> Unit

interface CoroutineScope {
    val coroutineContext: CoroutineContext
}

@Suppress("FunctionName")
fun CoroutineScope(context: CoroutineContext): CoroutineScope = TODO()

@Suppress("FunctionName")
fun MainScope(): CoroutineScope = TODO()

operator fun CoroutineScope.plus(context: CoroutineContext): CoroutineScope = TODO()

val CoroutineScope.isActive: Boolean
    get() = TODO()

fun CoroutineScope.cancel(cause: CancellationException? = null) {
    TODO()
}

fun CoroutineScope.ensureActive() {
    TODO()
}

@DelicateCoroutinesApi
object GlobalScope : CoroutineScope {
    override val coroutineContext: CoroutineContext
        get() = EmptyCoroutineContext
}

// A dispatcher IS a CoroutineContext element (the ContinuationInterceptor),
// so `launch(Dispatchers.IO)` and `Dispatchers.IO + job` type-check.
abstract class CoroutineDispatcher :
    AbstractCoroutineContextElement(ContinuationInterceptor),
    ContinuationInterceptor {
    abstract fun dispatch(context: CoroutineContext, block: Runnable)

    open fun isDispatchNeeded(context: CoroutineContext): Boolean = TODO()

    open fun limitedParallelism(parallelism: Int, name: String? = null): CoroutineDispatcher = TODO()

    final override fun <T> interceptContinuation(continuation: Continuation<T>): Continuation<T> = TODO()

    final override fun releaseInterceptedContinuation(continuation: Continuation<*>) {
        TODO()
    }

    final override operator fun <E : CoroutineContext.Element> get(key: CoroutineContext.Key<E>): E? = TODO()

    final override fun minusKey(key: CoroutineContext.Key<*>): CoroutineContext = TODO()
}

abstract class MainCoroutineDispatcher : CoroutineDispatcher() {
    abstract val immediate: MainCoroutineDispatcher
}

object Dispatchers {
    @JvmStatic
    val Default: CoroutineDispatcher
        get() = TODO()

    @JvmStatic
    val Main: MainCoroutineDispatcher
        get() = TODO()

    @JvmStatic
    val Unconfined: CoroutineDispatcher
        get() = TODO()

    @JvmStatic
    val IO: CoroutineDispatcher
        get() = TODO()
}

interface DisposableHandle {
    fun dispose()
}

interface Job : CoroutineContext.Element {
    companion object Key : CoroutineContext.Key<Job>

    val parent: Job?

    val isActive: Boolean

    val isCompleted: Boolean

    val isCancelled: Boolean

    val children: Sequence<Job>

    fun start(): Boolean

    fun cancel(cause: CancellationException? = null)

    suspend fun join()

    fun invokeOnCompletion(handler: CompletionHandler): DisposableHandle
}

interface CompletableJob : Job {
    fun complete(): Boolean

    fun completeExceptionally(exception: Throwable): Boolean
}

interface Deferred<out T> : Job {
    suspend fun await(): T

    @ExperimentalCoroutinesApi
    fun getCompleted(): T
}

@Suppress("FunctionName")
fun Job(parent: Job? = null): CompletableJob = TODO()

@Suppress("FunctionName")
fun SupervisorJob(parent: Job? = null): CompletableJob = TODO()

suspend fun Job.cancelAndJoin() {
    TODO()
}

fun Job.ensureActive() {
    TODO()
}

object NonCancellable : AbstractCoroutineContextElement(Job), Job {
    override val parent: Job?
        get() = null

    override val isActive: Boolean
        get() = true

    override val isCompleted: Boolean
        get() = false

    override val isCancelled: Boolean
        get() = false

    override val children: Sequence<Job>
        get() = emptySequence()

    override fun start(): Boolean = false

    override fun cancel(cause: CancellationException?) {}

    override suspend fun join() {
        TODO()
    }

    override fun invokeOnCompletion(handler: CompletionHandler): DisposableHandle = TODO()
}

interface CoroutineExceptionHandler : CoroutineContext.Element {
    companion object Key : CoroutineContext.Key<CoroutineExceptionHandler>

    fun handleException(context: CoroutineContext, exception: Throwable)
}

@Suppress("FunctionName")
inline fun CoroutineExceptionHandler(
    crossinline handler: (CoroutineContext, Throwable) -> Unit,
): CoroutineExceptionHandler = TODO()

data class CoroutineName(val name: String) : AbstractCoroutineContextElement(CoroutineName) {
    companion object Key : CoroutineContext.Key<CoroutineName>
}

interface ThreadContextElement<S> : CoroutineContext.Element {
    fun updateThreadContext(context: CoroutineContext): S

    fun restoreThreadContext(context: CoroutineContext, oldState: S)
}

enum class CoroutineStart {
    DEFAULT,
    LAZY,
    ATOMIC,
    UNDISPATCHED,
}

// ---- builders ----

fun CoroutineScope.launch(
    context: CoroutineContext = EmptyCoroutineContext,
    start: CoroutineStart = CoroutineStart.DEFAULT,
    block: suspend CoroutineScope.() -> Unit,
): Job = TODO()

fun <T> CoroutineScope.async(
    context: CoroutineContext = EmptyCoroutineContext,
    start: CoroutineStart = CoroutineStart.DEFAULT,
    block: suspend CoroutineScope.() -> T,
): Deferred<T> = TODO()

suspend fun <T> withContext(context: CoroutineContext, block: suspend CoroutineScope.() -> T): T = TODO()

fun <T> runBlocking(context: CoroutineContext = EmptyCoroutineContext, block: suspend CoroutineScope.() -> T): T =
    TODO()

suspend fun <R> coroutineScope(block: suspend CoroutineScope.() -> R): R = TODO()

suspend fun <R> supervisorScope(block: suspend CoroutineScope.() -> R): R = TODO()

suspend fun <T> withTimeout(timeMillis: Long, block: suspend CoroutineScope.() -> T): T = TODO()

suspend fun <T> withTimeout(timeout: Duration, block: suspend CoroutineScope.() -> T): T = TODO()

suspend fun <T> withTimeoutOrNull(timeMillis: Long, block: suspend CoroutineScope.() -> T): T? = TODO()

suspend fun <T> withTimeoutOrNull(timeout: Duration, block: suspend CoroutineScope.() -> T): T? = TODO()

suspend fun delay(timeMillis: Long) {
    TODO()
}

suspend fun delay(duration: Duration) {
    TODO()
}

suspend fun <T> awaitAll(vararg deferreds: Deferred<T>): List<T> = TODO()

suspend fun <T> Collection<Deferred<T>>.awaitAll(): List<T> = TODO()

suspend fun joinAll(vararg jobs: Job) {
    TODO()
}

suspend fun currentCoroutineContext(): CoroutineContext = TODO()
