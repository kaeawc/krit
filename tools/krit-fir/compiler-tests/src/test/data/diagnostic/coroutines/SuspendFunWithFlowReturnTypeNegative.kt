// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Flow functions and suspend functions that Go and FIR leave alone.
package test

import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.flow
import kotlinx.coroutines.withContext
import kotlinx.coroutines.Dispatchers

fun plainFlow(): Flow<Int> = flow { emit(1) }

suspend fun suspendOnly(): Int = 42

suspend fun nullableOnly(): Int? = null

// A suspend call before the flow is built is why the modifier is needed.
suspend fun delaysFirst(): Flow<Int> {
    delay(10)
    return flow { emit(1) }
}

suspend fun switchesContext(): Flow<Int> = withContext(Dispatchers.Default) { flow { emit(1) } }

suspend fun loadValue(): Int = 1

suspend fun awaitsProjectSuspend(): Flow<Int> {
    val value = loadValue()
    return flow { emit(value) }
}

// A signature that a supertype or a subclass fixes.
interface Source {
    suspend fun events(): Flow<Int>

    suspend fun withDefault(): Flow<Int> = flow { emit(1) }
}

abstract class Base {
    abstract suspend fun events(): Flow<Int>

    open suspend fun hook(): Flow<Int> = flow { emit(1) }
}

class Impl : Base(), Source {
    override suspend fun events(): Flow<Int> = flow { emit(1) }

    override suspend fun hook(): Flow<Int> = flow { emit(2) }
}

// A suspend function that returns something other than a Flow.
suspend fun listOfFlows(): List<Flow<Int>> = listOf(flow { emit(1) })

// The comment mentions suspend fun example(): Flow<Int> = flow { emit(1) } and is not code.
val text = "suspend fun inString(): Flow<Int> = flow { emit(1) }"
