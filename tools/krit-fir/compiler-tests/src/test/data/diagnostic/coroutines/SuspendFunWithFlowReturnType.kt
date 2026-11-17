// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 16, 21, 23, 25, 27, 30, 33, 35, 40, 43, 46, 52, 55, 59, 65, 69, 75
// Suspend functions that return a Flow without suspending before it is built.
package test

import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.SharedFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.flow
import kotlinx.coroutines.launch

<!SuspendFunWithFlowReturnType!>suspend<!> fun getFlow(): Flow<Int> = flow {
    emit(1)
    emit(2)
}

<!SuspendFunWithFlowReturnType!>suspend<!> fun getState(): StateFlow<String> = MutableStateFlow("hello")

<!SuspendFunWithFlowReturnType!>suspend<!> fun getShared(): SharedFlow<Int> = MutableSharedFlow()

<!SuspendFunWithFlowReturnType!>suspend<!> fun getMutableState(): MutableStateFlow<Int> = MutableStateFlow(0)

<!SuspendFunWithFlowReturnType!>suspend<!> fun getMutableShared(): MutableSharedFlow<Int> = MutableSharedFlow()

// A nullable Flow is still a Flow.
<!SuspendFunWithFlowReturnType!>suspend<!> fun maybeFlow(enabled: Boolean): Flow<Int>? = if (enabled) flow { emit(1) } else null

// The return type is inferred from the expression body.
<!SuspendFunWithFlowReturnType!>suspend<!> fun inferred() = flow { emit("x") }

<!SuspendFunWithFlowReturnType!>suspend<!> fun blockBody(): Flow<Int> {
    println("building")
    return flow { emit(1) }
}

<!SuspendFunWithFlowReturnType!>private<!> suspend fun privateFlow(): Flow<Int> = flow { emit(1) }

// A fully qualified name is a Flow too.
<!SuspendFunWithFlowReturnType!>suspend<!> fun qualified(): kotlinx.coroutines.flow.Flow<Int> = flow { emit(1) }

// The lambda of launch is its own coroutine, so the delay does not justify the modifier.
<!SuspendFunWithFlowReturnType!>suspend<!> fun startsCoroutine(scope: CoroutineScope): Flow<Int> {
    scope.launch { delay(1) }
    return flow { emit(1) }
}

// The type is inferred from a call that returns a MutableStateFlow.
<!SuspendFunWithFlowReturnType!>suspend<!> fun inferredState() = MutableStateFlow(0)

class Repository {
    <!SuspendFunWithFlowReturnType!>suspend<!> fun events(): Flow<Int> = flow { emit(1) }

    // A local function is reported like any other.
    fun holder() {
        <!SuspendFunWithFlowReturnType!>suspend<!> fun local(): Flow<Int> = flow { emit(1) }
        println("local")
    }
}

object Singleton {
    <!SuspendFunWithFlowReturnType!>suspend<!> fun events(): Flow<Int> = flow { emit(1) }
}

// The anchor stays on the first modifier line.
<!SuspendFunWithFlowReturnType!>@Suppress("unused")<!>
suspend fun annotated(): Flow<Int> = flow { emit(1) }

/**
 * Documented.
 */
<!SuspendFunWithFlowReturnType!>suspend<!> fun documented(): Flow<Int> = flow { emit(1) }
