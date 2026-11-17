// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 39
// Where FIR follows the resolved type or callee and Go follows the written name.
package test

import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.Flow as Stream
import kotlinx.coroutines.flow.flow

typealias Events = Flow<Int>

// Go reads the written name, so an import alias hides the Flow.
<!SuspendFunWithFlowReturnType!>suspend<!> fun aliasedImport(): Stream<Int> = flow { emit(1) }

// A typealias of Flow hides it from Go the same way.
<!SuspendFunWithFlowReturnType!>suspend<!> fun aliasedType(): Events = flow { emit(1) }

// Go stays silent: a member named like a suspend overload may suspend. The compiler knows it does not.
class Bag {
    fun toList(): List<Int> = emptyList()
}

<!SuspendFunWithFlowReturnType!>suspend<!> fun callsToList(bag: Bag): Flow<Int> {
    val values = bag.toList()
    return flow { values.forEach { emit(it) } }
}

interface Tool {
    fun first(): Int
}

<!SuspendFunWithFlowReturnType!>suspend<!> fun callsProjectFirst(tool: Tool): Flow<Int> {
    val value = tool.first()
    return flow { emit(value) }
}

// Go ignores every lambda, but the lambda of an inline function runs in the suspend function.
suspend fun inlineLambdaSuspends(): Flow<Int> {
    val result = run { delay(1) }
    println(result)
    return flow { emit(1) }
}
