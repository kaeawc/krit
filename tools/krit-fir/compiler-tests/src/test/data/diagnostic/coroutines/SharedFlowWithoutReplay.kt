// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 14, 17, 19, 21, 23, 29, 32, 34, 36, 38, 54, 61, 62, 67, 68, 72, 76, 80, 86, 97
// Positive: a val/var declaration that creates MutableSharedFlow with no
// arguments, reported once per declaration on its name (the `(` of a
// destructuring declaration), the line the Go rule reports.
package test

import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.SharedFlow
import kotlinx.coroutines.flow.asSharedFlow

typealias Events = MutableSharedFlow<String>

val <!SharedFlowWithoutReplay!>topLevel<!> = MutableSharedFlow<Int>()

class EventBus {
    private val <!SharedFlowWithoutReplay!>events<!> = MutableSharedFlow<String>()

    val <!SharedFlowWithoutReplay!>typed<!>: MutableSharedFlow<String> = MutableSharedFlow()

    val <!SharedFlowWithoutReplay!>aliased<!>: Events = MutableSharedFlow()

    val <!SharedFlowWithoutReplay!>qualified<!> = kotlinx.coroutines.flow.MutableSharedFlow<Long>()

    /**
     * Neither the KDoc nor the annotation line is the reported line.
     */
    @Volatile
    private var <!SharedFlowWithoutReplay!>annotated<!> = MutableSharedFlow<Int>()

    // A chained read-only view still creates the lossy flow.
    val <!SharedFlowWithoutReplay!>exposed<!>: SharedFlow<String> = MutableSharedFlow<String>().asSharedFlow()

    val <!SharedFlowWithoutReplay!>wrapped<!> = listOf(MutableSharedFlow<Int>())

    val <!SharedFlowWithoutReplay!>delegated<!> by lazy { MutableSharedFlow<Int>() }

    val <!SharedFlowWithoutReplay!>sameLineGetter<!>: MutableSharedFlow<Int> get() = MutableSharedFlow()

    // Go misses this because tree-sitter parses a getter on the next line as a
    // sibling of the property_declaration, so the declaration text Go matches
    // lacks the call; FIR reports it because every read creates a new lossy flow.
    val <!SharedFlowWithoutReplay!>getterBacked<!>: MutableSharedFlow<Int>
        get() = MutableSharedFlow()

    // Go misses this for the same reason: a setter on the next line is a
    // sibling of the property_declaration in tree-sitter. FIR reports it
    // because every write creates a new lossy flow.
    var <!SharedFlowWithoutReplay!>setterNext<!>: Int = 0
        set(value) { MutableSharedFlow<Int>(); field = value }

    // The message names a backticked declaration as written, backticks
    // included, like Go.
    val <!SharedFlowWithoutReplay!>`back tick`<!> = MutableSharedFlow<Int>()

    // Go misses this next-line getter too; FIR names it `a b` with backticks.
    val <!SharedFlowWithoutReplay!>`a b`<!>: MutableSharedFlow<Int>
        get() = MutableSharedFlow()

    // Both the enclosing declaration and the local one inside its lambda.
    val <!SharedFlowWithoutReplay!>computed<!> = run {
        val <!SharedFlowWithoutReplay!>inner<!> = MutableSharedFlow<Int>()
        inner.asSharedFlow()
    }

    // Both the enclosing declaration and the anonymous object's member.
    val <!SharedFlowWithoutReplay!>holder<!> = object {
        val <!SharedFlowWithoutReplay!>member<!> = MutableSharedFlow<Int>()
    }

    companion object {
        val <!SharedFlowWithoutReplay!>shared<!> = MutableSharedFlow<Unit>()
    }

    fun local() {
        val <!SharedFlowWithoutReplay!>local<!> = MutableSharedFlow<String>()
        local.tryEmit("x")
        // A destructuring declaration is one declaration; like Go, its
        // message names '' because it has no single name.
        val <!SharedFlowWithoutReplay!>(<!>flow, count) = Pair(MutableSharedFlow<Int>(), 1)
        flow.tryEmit(count)
    }
}

object Singleton {
    val <!SharedFlowWithoutReplay!>events<!> = MutableSharedFlow<Int>()
}

interface Source {
    // Go misses this for the same next-line getter reason as getterBacked.
    val <!SharedFlowWithoutReplay!>events<!>: MutableSharedFlow<Int>
        get() = MutableSharedFlow<Int>()
}

enum class Mode {
    A {
        val <!SharedFlowWithoutReplay!>state<!> = MutableSharedFlow<Int>()
    },
    B,
}
