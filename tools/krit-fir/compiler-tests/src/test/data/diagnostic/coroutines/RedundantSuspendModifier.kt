// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 21, 30, 37, 46, 48, 51, 55, 127
// Functions marked suspend that never suspend, and the calls that justify the modifier.
package test

import kotlinx.coroutines.Deferred
import kotlinx.coroutines.coroutineScope
import kotlinx.coroutines.delay
import kotlinx.coroutines.withContext
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.channels.Channel
import kotlinx.coroutines.flow.Flow
import kotlin.coroutines.coroutineContext

fun helper() = Unit

suspend fun suspendHelper() {
    delay(1)
}

<!RedundantSuspendModifier!>suspend<!> fun simple() {
    println("no suspend calls here")
}

// Go stays silent because it cannot tell whether helper() suspends; the compiler resolves it.
<!RedundantSuspendModifier!>suspend<!> fun onlyProjectNonSuspendCalls() {
    helper()
}

<!RedundantSuspendModifier!>suspend<!> fun nestedSuspendOwnsItsScope() {
    suspend fun inner() {
        delay(1)
    }
    println("outer has nothing suspending")
}

<!RedundantSuspendModifier!>suspend<!> fun nestedObjectScopeLeak() {
    val box = object {
        suspend fun work() {
            delay(1)
        }
    }
    println(box)
}

<!RedundantSuspendModifier!>suspend<!> fun expressionBody() = 42

private <!RedundantSuspendModifier!>suspend<!> fun withModifiers() {}

class Holder {
    <!RedundantSuspendModifier!>suspend<!> fun member() {
        println("member")
    }

    private <!RedundantSuspendModifier!>suspend<!> fun privateMember() = Unit
}

// Calls that suspend: the modifier is needed.
suspend fun delays() {
    delay(100)
}

suspend fun callsProjectSuspend() {
    suspendHelper()
}

suspend fun invokesSuspendValue(block: suspend () -> Unit) {
    block()
}

suspend fun suspendsInQualifiedCall(deferred: Deferred<Int>): Int = deferred.await()

suspend fun scoped() = coroutineScope {
    helper()
}

suspend fun contextSwitch() {
    withContext(Dispatchers.IO) {
        helper()
    }
}

suspend fun receivesFromChannel(channel: Channel<Int>) {
    for (item in channel) {
        println(item)
    }
}

suspend fun collectsFlow(flow: Flow<Int>) {
    flow.collect { println(it) }
}

suspend fun readsCoroutineContext() {
    println(coroutineContext)
}

// A lambda passed to an inline function runs in the suspend function's context.
suspend fun suspendInInlineLambda() {
    runCatching {
        delay(10)
    }
}

suspend fun suspendInApply(sb: StringBuilder) {
    sb.apply {
        delay(1)
    }
}

suspend fun computeSuspending(): Int {
    delay(1)
    return 1
}

suspend fun suspendInTry() {
    try {
        delay(1)
    } finally {
        println("done")
    }
}

// Signatures fixed by a supertype or the platform keep the modifier.
open class Base {
    open suspend fun overridable() {}

    <!RedundantSuspendModifier!>suspend<!> fun neverSuspends() {}
}

abstract class Abstract {
    abstract suspend fun required()
}

class Child : Abstract() {
    override suspend fun required() {}
}

interface Contract {
    suspend fun fromInterface() {
        println("default body")
    }
}

fun interface Callback {
    suspend fun run()
}
