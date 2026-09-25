// Compiler-test source stubs; never packaged in the production artifact.
package kotlinx.coroutines.test

import kotlin.coroutines.CoroutineContext
import kotlin.coroutines.EmptyCoroutineContext
import kotlin.time.Duration
import kotlin.time.Duration.Companion.seconds
import kotlinx.coroutines.CoroutineDispatcher
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.ExperimentalCoroutinesApi

// `expect class TestResult` is `actual typealias TestResult = Unit` on the JVM.
typealias TestResult = Unit

class TestCoroutineScheduler {
    val currentTime: Long
        get() = TODO()

    fun advanceUntilIdle() {
        TODO()
    }

    fun advanceTimeBy(delayTimeMillis: Long) {
        TODO()
    }

    fun runCurrent() {
        TODO()
    }
}

abstract class TestDispatcher : CoroutineDispatcher() {
    abstract val scheduler: TestCoroutineScheduler
}

@Suppress("FunctionName")
fun StandardTestDispatcher(scheduler: TestCoroutineScheduler? = null, name: String? = null): TestDispatcher = TODO()

@Suppress("FunctionName")
fun UnconfinedTestDispatcher(scheduler: TestCoroutineScheduler? = null, name: String? = null): TestDispatcher = TODO()

interface TestScope : CoroutineScope {
    val testScheduler: TestCoroutineScheduler

    val backgroundScope: CoroutineScope
}

@Suppress("FunctionName")
fun TestScope(context: CoroutineContext = EmptyCoroutineContext): TestScope = TODO()

@ExperimentalCoroutinesApi
val TestScope.currentTime: Long
    get() = TODO()

@ExperimentalCoroutinesApi
fun TestScope.advanceUntilIdle() {
    TODO()
}

@ExperimentalCoroutinesApi
fun TestScope.advanceTimeBy(delayTimeMillis: Long) {
    TODO()
}

@ExperimentalCoroutinesApi
fun TestScope.runCurrent() {
    TODO()
}

// The test body's receiver is TestScope, not a plain CoroutineScope.
fun runTest(
    context: CoroutineContext = EmptyCoroutineContext,
    timeout: Duration = 60.seconds,
    testBody: suspend TestScope.() -> Unit,
): TestResult = TODO()

fun TestScope.runTest(timeout: Duration = 60.seconds, testBody: suspend TestScope.() -> Unit): TestResult = TODO()

@ExperimentalCoroutinesApi
fun Dispatchers.setMain(dispatcher: CoroutineDispatcher) {
    TODO()
}

@ExperimentalCoroutinesApi
fun Dispatchers.resetMain() {
    TODO()
}
