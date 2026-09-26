// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 16, 18, 22, 24
// Deliberate precision differences from the Go rule, which substring-matches
// the declaration text instead of resolving the MutableSharedFlow call.
package test

import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.MutableSharedFlow as EventFlow

fun <T> makeFlow(): MutableSharedFlow<T> = MutableSharedFlow(replay = 1)

class EventBus {
    // Go reports these because the text contains "MutableSharedFlow<" and an
    // unrelated ">()" call; FIR is correct because the only MutableSharedFlow
    // created passes replay.
    val factoryMade: MutableSharedFlow<Int> = makeFlow<Int>()

    val tagged = MutableSharedFlow<Int>(replay = 1).also { listOf<Int>() }

    // Go reports these because a comment or a string spells the call; FIR is
    // correct because no MutableSharedFlow is created.
    val commented = /* MutableSharedFlow() */ 1

    val label = "MutableSharedFlow()"

    // Go misses these because another call in the same declaration is spelled
    // "MutableSharedFlow(replay"; FIR reports the lossy no-argument call.
    <!SharedFlowWithoutReplay!>val<!> mixed: List<MutableSharedFlow<Int>> = listOf(MutableSharedFlow(replay = 1), MutableSharedFlow())

    <!SharedFlowWithoutReplay!>val<!> multiline: List<MutableSharedFlow<Int>> = listOf(
        MutableSharedFlow(
            replay = 1,
        ),
        MutableSharedFlow(),
    )

    // Go misses these: an import alias hides the name, and whitespace sits
    // inside the empty argument list.
    <!SharedFlowWithoutReplay!>val<!> aliased = EventFlow<Int>()

    <!SharedFlowWithoutReplay!>val<!> spaced = MutableSharedFlow<Int>( )
}
