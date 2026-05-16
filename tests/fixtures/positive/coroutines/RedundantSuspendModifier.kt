package fixtures.positive.coroutines

import kotlinx.coroutines.delay

suspend fun simple() {
    println("no suspend calls here")
}

fun helper() = Unit

suspend fun onlyProjectNonSuspendCalls() {
    helper()
}

// Nested suspend fun owns its own scope. The outer body itself has no
// suspend calls, so the outer modifier is redundant even though the inner
// function calls delay.
suspend fun nestedSuspendOwnsItsScope() {
    suspend fun inner() {
        delay(1)
    }
    println("outer has nothing suspending")
}

// Anonymous function with a suspend body should not lend its calls to the
// parent: parent body has no suspending work of its own.
suspend fun anonymousFunctionScopeLeak() {
    val produce: suspend () -> Unit = suspend fun() {
        delay(1)
    }
    println(produce)
}

// Nested object literal with a member that calls suspend APIs should not
// affect the outer modifier.
suspend fun nestedObjectScopeLeak() {
    val box = object {
        suspend fun work() {
            delay(1)
        }
    }
    println(box)
}
