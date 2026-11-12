// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 18, 22, 26, 31, 36, 41, 44, 49, 50, 51, 56, 60, 61, 62, 63, 64, 69, 74, 80
// Thread.sleep in a suspend function and in the lambdas that run as coroutines
// or in place of the suspend caller. Go and FIR agree on every line here.
package test

import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.coroutineScope
import kotlinx.coroutines.launch
import kotlinx.coroutines.runBlocking
import kotlinx.coroutines.withContext
import kotlinx.coroutines.flow.callbackFlow
import kotlinx.coroutines.flow.channelFlow
import kotlinx.coroutines.flow.flow

suspend fun plain() {
    <!SleepInsteadOfDelay!>Thread.sleep(10)<!>
}

suspend fun qualified() {
    <!SleepInsteadOfDelay!>java.lang.Thread.sleep(10)<!>
}

suspend fun twoArguments() {
    <!SleepInsteadOfDelay!>Thread.sleep(10, 5)<!>
}

fun launched(scope: CoroutineScope) {
    scope.launch(Dispatchers.IO) {
        <!SleepInsteadOfDelay!>Thread.sleep(10)<!>
    }
}

fun blocking() = runBlocking {
    <!SleepInsteadOfDelay!>Thread.sleep(10)<!>
}

suspend fun contexts() {
    withContext(Dispatchers.IO) {
        <!SleepInsteadOfDelay!>Thread.sleep(10)<!>
    }
    coroutineScope {
        <!SleepInsteadOfDelay!>Thread.sleep(10)<!>
    }
}

fun flows() {
    flow<Int> { <!SleepInsteadOfDelay!>Thread.sleep(10)<!> }
    channelFlow<Int> { <!SleepInsteadOfDelay!>Thread.sleep(10)<!> }
    callbackFlow<Int> { <!SleepInsteadOfDelay!>Thread.sleep(10)<!> }
}

fun suspendLambda() {
    val block: suspend () -> Unit = { <!SleepInsteadOfDelay!>Thread.sleep(10)<!> }
    val builder = suspend { <!SleepInsteadOfDelay!>Thread.sleep(10)<!> }
}

suspend fun inPlaceLambdas(items: List<Int>) {
    run { <!SleepInsteadOfDelay!>Thread.sleep(10)<!> }
    1.let { <!SleepInsteadOfDelay!>Thread.sleep(10)<!> }
    items.forEach { <!SleepInsteadOfDelay!>Thread.sleep(10)<!> }
    items.map { <!SleepInsteadOfDelay!>Thread.sleep(10)<!> }
    repeat(2) { <!SleepInsteadOfDelay!>Thread.sleep(10)<!> }
}

fun nestedInline(scope: CoroutineScope) {
    scope.launch {
        run { <!SleepInsteadOfDelay!>Thread.sleep(10)<!> }
    }
}

suspend fun referenceArgument(items: List<Long>) {
    <!SleepInsteadOfDelay!>items.forEach(Thread::sleep)<!>
}

// A local variable's initializer runs in the suspend function.
suspend fun localInitializer(): Int {
    val result = run {
        <!SleepInsteadOfDelay!>Thread.sleep(10)<!>
        42
    }
    return result
}
