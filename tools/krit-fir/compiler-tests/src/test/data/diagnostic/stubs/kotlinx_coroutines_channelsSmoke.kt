// Smoke: Channel factory, send/receive, trySend, iteration, and close.
package stubs

import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.channels.BufferOverflow
import kotlinx.coroutines.channels.Channel
import kotlinx.coroutines.channels.ReceiveChannel
import kotlinx.coroutines.channels.SendChannel
import kotlinx.coroutines.channels.consumeEach
import kotlinx.coroutines.channels.produce
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.receiveAsFlow
import kotlinx.coroutines.launch

suspend fun channelSmoke(scope: CoroutineScope) {
    val channel = Channel<Int>(Channel.BUFFERED)
    val conflated = Channel<String>(capacity = Channel.CONFLATED, onBufferOverflow = BufferOverflow.DROP_OLDEST)
    val sender: SendChannel<Int> = channel
    val receiver: ReceiveChannel<Int> = channel
    scope.launch {
        sender.send(1)
        <!PrintlnInProduction!>println<!>(sender.trySend(2).isSuccess)
        sender.close()
    }
    for (value in receiver) <!PrintlnInProduction!>println<!>(value)
    val first: Int = channel.receive()
    val maybe: String? = conflated.tryReceive().getOrNull()
    val asFlow: Flow<Int> = Channel<Int>(Channel.UNLIMITED).receiveAsFlow()
    scope.produce { send(1) }.consumeEach { <!PrintlnInProduction!>println<!>(it) }
    conflated.cancel()
    <!PrintlnInProduction!>println<!>("$first $maybe $asFlow ${channel.isClosedForSend}")
}
