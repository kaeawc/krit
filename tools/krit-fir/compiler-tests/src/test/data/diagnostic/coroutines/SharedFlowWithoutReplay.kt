// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 14, 17, 19, 21, 23, 28, 32, 34, 36, 38, 47, 48, 53, 54, 58, 62, 66, 72, 83
// Positive: a val/var declaration that creates MutableSharedFlow with no
// arguments, reported once per declaration on its first line (modifier list,
// else val/var), the line the Go rule reports.
package test

import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.SharedFlow
import kotlinx.coroutines.flow.asSharedFlow

typealias Events = MutableSharedFlow<String>

<!SharedFlowWithoutReplay!>val<!> topLevel = MutableSharedFlow<Int>()

class EventBus {
    <!SharedFlowWithoutReplay!>private<!> val events = MutableSharedFlow<String>()

    <!SharedFlowWithoutReplay!>val<!> typed: MutableSharedFlow<String> = MutableSharedFlow()

    <!SharedFlowWithoutReplay!>val<!> aliased: Events = MutableSharedFlow()

    <!SharedFlowWithoutReplay!>val<!> qualified = kotlinx.coroutines.flow.MutableSharedFlow<Long>()

    /**
     * KDoc is not part of the reported line.
     */
    <!SharedFlowWithoutReplay!>@Volatile<!>
    private var annotated = MutableSharedFlow<Int>()

    // A chained read-only view still creates the lossy flow.
    <!SharedFlowWithoutReplay!>val<!> exposed: SharedFlow<String> = MutableSharedFlow<String>().asSharedFlow()

    <!SharedFlowWithoutReplay!>val<!> wrapped = listOf(MutableSharedFlow<Int>())

    <!SharedFlowWithoutReplay!>val<!> delegated by lazy { MutableSharedFlow<Int>() }

    <!SharedFlowWithoutReplay!>val<!> sameLineGetter: MutableSharedFlow<Int> get() = MutableSharedFlow()

    // Go misses this because tree-sitter parses a getter on the next line as a
    // sibling of the property_declaration, so the declaration text Go matches
    // lacks the call; FIR reports it because every read creates a new lossy flow.
    <!SharedFlowWithoutReplay!>val<!> getterBacked: MutableSharedFlow<Int>
        get() = MutableSharedFlow()

    // Both the enclosing declaration and the local one inside its lambda.
    <!SharedFlowWithoutReplay!>val<!> computed = run {
        <!SharedFlowWithoutReplay!>val<!> inner = MutableSharedFlow<Int>()
        inner.asSharedFlow()
    }

    // Both the enclosing declaration and the anonymous object's member.
    <!SharedFlowWithoutReplay!>val<!> holder = object {
        <!SharedFlowWithoutReplay!>val<!> member = MutableSharedFlow<Int>()
    }

    companion object {
        <!SharedFlowWithoutReplay!>val<!> shared = MutableSharedFlow<Unit>()
    }

    fun local() {
        <!SharedFlowWithoutReplay!>val<!> local = MutableSharedFlow<String>()
        local.tryEmit("x")
        // A destructuring declaration is one declaration; like Go, its
        // message names '' because it has no single name.
        <!SharedFlowWithoutReplay!>val<!> (flow, count) = Pair(MutableSharedFlow<Int>(), 1)
        flow.tryEmit(count)
    }
}

object Singleton {
    <!SharedFlowWithoutReplay!>val<!> events = MutableSharedFlow<Int>()
}

interface Source {
    // Go misses this for the same next-line getter reason as getterBacked.
    <!SharedFlowWithoutReplay!>val<!> events: MutableSharedFlow<Int>
        get() = MutableSharedFlow<Int>()
}

enum class Mode {
    A {
        <!SharedFlowWithoutReplay!>val<!> state = MutableSharedFlow<Int>()
    },
    B,
}
