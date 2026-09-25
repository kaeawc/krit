// Compiler-test source stubs; never packaged in the production artifact.
package kotlinx.coroutines.channels

import kotlin.coroutines.CoroutineContext
import kotlin.coroutines.EmptyCoroutineContext
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.DelicateCoroutinesApi
import kotlinx.coroutines.ExperimentalCoroutinesApi

enum class BufferOverflow {
    SUSPEND,
    DROP_OLDEST,
    DROP_LATEST,
}

interface SendChannel<in E> {
    @DelicateCoroutinesApi
    val isClosedForSend: Boolean

    suspend fun send(element: E)

    fun trySend(element: E): ChannelResult<Unit>

    fun close(cause: Throwable? = null): Boolean
}

interface ReceiveChannel<out E> {
    @DelicateCoroutinesApi
    val isClosedForReceive: Boolean

    suspend fun receive(): E

    fun tryReceive(): ChannelResult<E>

    operator fun iterator(): ChannelIterator<E>

    fun cancel(cause: CancellationException? = null)
}

interface ChannelIterator<out E> {
    suspend operator fun hasNext(): Boolean

    operator fun next(): E
}

// An interface (not a class) created through the Channel(...) factory.
interface Channel<E> : SendChannel<E>, ReceiveChannel<E> {
    companion object Factory {
        const val UNLIMITED: Int = Int.MAX_VALUE
        const val RENDEZVOUS: Int = 0
        const val CONFLATED: Int = -1
        const val BUFFERED: Int = -2
    }
}

@Suppress("FunctionName")
fun <E> Channel(
    capacity: Int = Channel.RENDEZVOUS,
    onBufferOverflow: BufferOverflow = BufferOverflow.SUSPEND,
    onUndeliveredElement: ((E) -> Unit)? = null,
): Channel<E> = TODO()

@JvmInline
value class ChannelResult<out T> internal constructor(private val holder: Any?) {
    val isSuccess: Boolean
        get() = TODO()

    val isFailure: Boolean
        get() = TODO()

    val isClosed: Boolean
        get() = TODO()

    fun getOrNull(): T? = TODO()
}

interface ProducerScope<in E> : CoroutineScope, SendChannel<E> {
    val channel: SendChannel<E>
}

suspend fun ProducerScope<*>.awaitClose(block: () -> Unit = {}) {
    TODO()
}

@ExperimentalCoroutinesApi
fun <E> CoroutineScope.produce(
    context: CoroutineContext = EmptyCoroutineContext,
    capacity: Int = 0,
    block: suspend ProducerScope<E>.() -> Unit,
): ReceiveChannel<E> = TODO()

suspend inline fun <E> ReceiveChannel<E>.consumeEach(action: (E) -> Unit) {
    TODO()
}

// receiveAsFlow/consumeAsFlow live in kotlinx.coroutines.flow (flow.kt).
