// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 20, 38, 40, 42, 55, 61
// A declaration that gets its flow from a factory function. Go matches the
// declaration's spelling ("MutableSharedFlow<" plus ">()", or a name ending in
// "MutableSharedFlow()"); FIR follows a source factory's body and reports when
// a value the factory returns is a MutableSharedFlow created with no
// arguments.
package test.factory

import kotlinx.coroutines.flow.MutableSharedFlow

fun <T> lossy(): MutableSharedFlow<T> = MutableSharedFlow()

fun <T> createMutableSharedFlow(): MutableSharedFlow<T> = MutableSharedFlow()

fun <T> replayMutableSharedFlow(): MutableSharedFlow<T> = MutableSharedFlow(replay = 1)

fun <T> blockLossy(): MutableSharedFlow<T> {
    // The factory's own local declaration is reported, like Go.
    <!SharedFlowWithoutReplay!>val<!> flow = MutableSharedFlow<T>()
    return flow
}

// A factory that delegates to another lossy factory.
fun <T> chained(): MutableSharedFlow<T> = lossy()

// A recursive factory that never creates a flow terminates.
fun <T> recursive(depth: Int): MutableSharedFlow<T> =
    if (depth > 0) recursive(depth - 1) else MutableSharedFlow(replay = 1)

interface FlowFactory {
    fun <T> create(): MutableSharedFlow<T>
}

class EventBus(factory: FlowFactory) {
    // Go reports these by their spelling; FIR reports them because the
    // factory returns a MutableSharedFlow created with no arguments.
    <!SharedFlowWithoutReplay!>val<!> bus: MutableSharedFlow<Int> = lossy<Int>()

    <!SharedFlowWithoutReplay!>val<!> suffixed: MutableSharedFlow<Int> = createMutableSharedFlow()

    <!SharedFlowWithoutReplay!>val<!> blockMade: MutableSharedFlow<Int> = blockLossy<Int>()

    // Go misses these: without an explicit type argument, or without the
    // declared type, the text holds neither "MutableSharedFlow<" plus ">()"
    // nor "MutableSharedFlow()". The factory still returns a lossy flow.
    <!SharedFlowWithoutReplay!>val<!> untyped: MutableSharedFlow<Int> = lossy()

    <!SharedFlowWithoutReplay!>val<!> inferred = lossy<Int>()

    <!SharedFlowWithoutReplay!>val<!> viaChain: MutableSharedFlow<Int> = chained()

    // Go reports this because the name ends in "MutableSharedFlow()"; FIR is
    // correct because the factory's only flow passes replay.
    val replaySuffixed: MutableSharedFlow<Int> = replayMutableSharedFlow()

    val recursed: MutableSharedFlow<Int> = recursive<Int>(2)

    // Go reports this by its spelling. FIR cannot see an abstract factory's
    // body, so it cannot tell whether the flow it returns passes replay.
    val fromInterface: MutableSharedFlow<Int> = factory.create<Int>()
}
