// Compiler-test source stubs; never packaged in the production artifact.
package io.mockk

import kotlin.reflect.KClass

inline fun <reified T : Any> mockk(
    name: String? = null,
    relaxed: Boolean = false,
    vararg moreInterfaces: KClass<*>,
    relaxUnitFun: Boolean = false,
    block: T.() -> Unit = {},
): T = TODO()

inline fun <reified T : Any> spyk(
    name: String? = null,
    vararg moreInterfaces: KClass<*>,
    recordPrivateCalls: Boolean = false,
    block: T.() -> Unit = {},
): T = TODO()

inline fun <reified T : Any> spyk(
    objToCopy: T,
    name: String? = null,
    vararg moreInterfaces: KClass<*>,
    recordPrivateCalls: Boolean = false,
    block: T.() -> Unit = {},
): T = TODO()

fun <T> every(stubBlock: MockKMatcherScope.() -> T): MockKStubScope<T, T> = TODO()

fun <T> coEvery(stubBlock: suspend MockKMatcherScope.() -> T): MockKStubScope<T, T> = TODO()

fun verify(
    ordering: Ordering = Ordering.UNORDERED,
    inverse: Boolean = false,
    atLeast: Int = 1,
    atMost: Int = Int.MAX_VALUE,
    exactly: Int = -1,
    timeout: Long = 0,
    verifyBlock: MockKVerificationScope.() -> Unit,
) {
    TODO()
}

fun coVerify(
    ordering: Ordering = Ordering.UNORDERED,
    inverse: Boolean = false,
    atLeast: Int = 1,
    atMost: Int = Int.MAX_VALUE,
    exactly: Int = -1,
    timeout: Long = 0,
    verifyBlock: suspend MockKVerificationScope.() -> Unit,
) {
    TODO()
}

fun confirmVerified(vararg mocks: Any) {
    TODO()
}

fun clearAllMocks() {
    TODO()
}

fun unmockkAll() {
    TODO()
}

enum class Ordering {
    UNORDERED,
    ALL,
    ORDERED,
    SEQUENCE,
}

open class MockKMatcherScope {
    inline fun <reified T : Any> any(): T = TODO()

    inline fun <reified T : Any> eq(value: T, inverse: Boolean = false): T = TODO()

    inline fun <reified T : Any> match(noinline matcher: (T) -> Boolean): T = TODO()

    inline fun <reified T : Any> capture(lst: CapturingSlot<T>): T = TODO()
}

open class MockKVerificationScope : MockKMatcherScope()

class CapturingSlot<T : Any> {
    val captured: T
        get() = TODO()

    val isCaptured: Boolean
        get() = TODO()
}

inline fun <reified T : Any> slot(): CapturingSlot<T> = TODO()

class Call

class MockKAnswerScope<T, B> {
    val call: Call
        get() = TODO()

    inline fun <reified T> firstArg(): T = TODO()

    inline fun <reified T> secondArg(): T = TODO()

    inline fun <reified T> arg(n: Int): T = TODO()
}

class MockKAdditionalAnswerScope<T, B> {
    infix fun andThen(answer: T): MockKAdditionalAnswerScope<T, B> = TODO()
}

class MockKStubScope<T, B> {
    infix fun returns(returnValue: T): MockKAdditionalAnswerScope<T, B> = TODO()

    infix fun returnsMany(values: List<T>): MockKAdditionalAnswerScope<T, B> = TODO()

    infix fun answers(answer: MockKAnswerScope<T, B>.(Call) -> T): MockKAdditionalAnswerScope<T, B> = TODO()

    infix fun coAnswers(answer: suspend MockKAnswerScope<T, B>.(Call) -> T): MockKAdditionalAnswerScope<T, B> = TODO()

    infix fun throws(ex: Throwable): MockKAdditionalAnswerScope<T, B> = TODO()
}

object Runs

infix fun MockKStubScope<Unit, Unit>.just(runs: Runs) {
    TODO()
}
