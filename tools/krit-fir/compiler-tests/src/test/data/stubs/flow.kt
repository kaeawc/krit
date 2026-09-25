// Top-level flow operators compile into the real multifile facade FlowKt.
@file:JvmName("FlowKt")

// Compiler-test source stubs; never packaged in the production artifact.
package kotlinx.coroutines.flow

import kotlin.coroutines.CoroutineContext
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.Job
import kotlinx.coroutines.channels.BufferOverflow
import kotlinx.coroutines.channels.ProducerScope
import kotlinx.coroutines.channels.ReceiveChannel

// `flow.collect { }` resolves to this suspend member via SAM conversion of
// FlowCollector; there is no lambda-taking collect extension any more.
interface Flow<out T> {
    suspend fun collect(collector: FlowCollector<T>)
}

fun interface FlowCollector<in T> {
    suspend fun emit(value: T)
}

interface SharedFlow<out T> : Flow<T> {
    val replayCache: List<T>

    override suspend fun collect(collector: FlowCollector<T>): Nothing
}

interface MutableSharedFlow<T> : SharedFlow<T>, FlowCollector<T> {
    val subscriptionCount: StateFlow<Int>

    override suspend fun emit(value: T)

    fun tryEmit(value: T): Boolean

    fun resetReplayCache()
}

interface StateFlow<out T> : SharedFlow<T> {
    val value: T
}

interface MutableStateFlow<T> : StateFlow<T>, MutableSharedFlow<T> {
    override var value: T

    fun compareAndSet(expect: T, update: T): Boolean
}

@Suppress("FunctionName")
fun <T> MutableStateFlow(value: T): MutableStateFlow<T> = TODO()

@Suppress("FunctionName")
fun <T> MutableSharedFlow(
    replay: Int = 0,
    extraBufferCapacity: Int = 0,
    onBufferOverflow: BufferOverflow = BufferOverflow.SUSPEND,
): MutableSharedFlow<T> = TODO()

fun <T> MutableStateFlow<T>.asStateFlow(): StateFlow<T> = TODO()

fun <T> MutableSharedFlow<T>.asSharedFlow(): SharedFlow<T> = TODO()

inline fun <T> MutableStateFlow<T>.update(function: (T) -> T) {
    TODO()
}

fun interface SharingStarted {
    fun command(subscriptionCount: StateFlow<Int>): Flow<SharingCommand>

    companion object {
        val Eagerly: SharingStarted
            get() = TODO()

        val Lazily: SharingStarted
            get() = TODO()

        @Suppress("FunctionName")
        fun WhileSubscribed(
            stopTimeoutMillis: Long = 0,
            replayExpirationMillis: Long = Long.MAX_VALUE,
        ): SharingStarted = TODO()
    }
}

enum class SharingCommand {
    START,
    STOP,
    STOP_AND_RESET_REPLAY_CACHE,
}

// ---- builders ----

fun <T> flow(block: suspend FlowCollector<T>.() -> Unit): Flow<T> = TODO()

fun <T> flowOf(vararg elements: T): Flow<T> = TODO()

fun <T> flowOf(value: T): Flow<T> = TODO()

fun <T> emptyFlow(): Flow<T> = TODO()

fun <T> Iterable<T>.asFlow(): Flow<T> = TODO()

fun <T> callbackFlow(block: suspend ProducerScope<T>.() -> Unit): Flow<T> = TODO()

fun <T> channelFlow(block: suspend ProducerScope<T>.() -> Unit): Flow<T> = TODO()

fun <T> ReceiveChannel<T>.receiveAsFlow(): Flow<T> = TODO()

fun <T> ReceiveChannel<T>.consumeAsFlow(): Flow<T> = TODO()

// ---- terminal operators ----

suspend fun Flow<*>.collect() {
    TODO()
}

suspend fun <T> Flow<T>.collectLatest(action: suspend (value: T) -> Unit) {
    TODO()
}

suspend inline fun <T> Flow<T>.collectIndexed(crossinline action: suspend (index: Int, value: T) -> Unit) {
    TODO()
}

fun <T> Flow<T>.launchIn(scope: CoroutineScope): Job = TODO()

suspend fun <T> Flow<T>.first(): T = TODO()

suspend fun <T> Flow<T>.firstOrNull(): T? = TODO()

suspend fun <T> Flow<T>.single(): T = TODO()

suspend fun <T> Flow<T>.toList(destination: MutableList<T> = ArrayList()): List<T> = TODO()

// ---- intermediate operators (suspend transform lambdas) ----

inline fun <T, R> Flow<T>.map(crossinline transform: suspend (value: T) -> R): Flow<R> = TODO()

inline fun <T, R : Any> Flow<T>.mapNotNull(crossinline transform: suspend (value: T) -> R?): Flow<R> = TODO()

inline fun <T> Flow<T>.filter(crossinline predicate: suspend (T) -> Boolean): Flow<T> = TODO()

fun <T> Flow<T>.onEach(action: suspend (T) -> Unit): Flow<T> = TODO()

fun <T> Flow<T>.onStart(action: suspend FlowCollector<T>.() -> Unit): Flow<T> = TODO()

fun <T> Flow<T>.onCompletion(action: suspend FlowCollector<T>.(cause: Throwable?) -> Unit): Flow<T> = TODO()

fun <T> Flow<T>.catch(action: suspend FlowCollector<T>.(cause: Throwable) -> Unit): Flow<T> = TODO()

fun <T> Flow<T>.flowOn(context: CoroutineContext): Flow<T> = TODO()

fun <T> Flow<T>.distinctUntilChanged(): Flow<T> = TODO()

fun <T> Flow<T>.debounce(timeoutMillis: Long): Flow<T> = TODO()

fun <T> Flow<T>.take(count: Int): Flow<T> = TODO()

fun <T> Flow<T>.drop(count: Int): Flow<T> = TODO()

@ExperimentalCoroutinesApi
inline fun <T, R> Flow<T>.flatMapLatest(crossinline transform: suspend (value: T) -> Flow<R>): Flow<R> = TODO()

@JvmName("flowCombine")
fun <T1, T2, R> Flow<T1>.combine(flow: Flow<T2>, transform: suspend (a: T1, b: T2) -> R): Flow<R> = TODO()

fun <T1, T2, R> combine(flow: Flow<T1>, flow2: Flow<T2>, transform: suspend (a: T1, b: T2) -> R): Flow<R> = TODO()

fun <T> merge(vararg flows: Flow<T>): Flow<T> = TODO()

// ---- sharing ----

fun <T> Flow<T>.stateIn(scope: CoroutineScope, started: SharingStarted, initialValue: T): StateFlow<T> = TODO()

suspend fun <T> Flow<T>.stateIn(scope: CoroutineScope): StateFlow<T> = TODO()

fun <T> Flow<T>.shareIn(scope: CoroutineScope, started: SharingStarted, replay: Int = 0): SharedFlow<T> = TODO()
