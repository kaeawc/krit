package dev.jasonpearson.krit.fir.checkers.testingquality

import dev.jasonpearson.krit.fir.support.classIdOf
import org.jetbrains.kotlin.fir.expressions.FirBlock
import org.jetbrains.kotlin.name.CallableId

internal object TestWithoutAssertionCalls {
    val junitOwners = setOf(
        classIdOf("org.junit", "Assert"),
        classIdOf("junit.framework", "Assert"),
        classIdOf("junit.framework", "TestCase"),
        classIdOf("org.junit.jupiter.api", "Assertions"),
    )
    val verifyNames = setOf(
        "verify", "coVerify", "verifyAll", "coVerifyAll", "verifyOrder", "coVerifyOrder",
        "verifySequence", "coVerifySequence", "confirmVerified", "verifyNoInteractions",
        "verifyNoMoreInteractions", "verifyBlocking",
    )
    val turbineNames = setOf(
        "awaitItem", "awaitComplete", "awaitError", "expectMostRecentItem", "expectNoEvents",
        "ensureAllEventsConsumed", "cancelAndConsumeRemainingEvents", "cancelAndIgnoreRemainingEvents",
    )
    val nonAssertingCalls = setOf(
        "kotlin.io.print", "kotlin.io.println",
        "kotlin.collections.listOf", "kotlin.collections.emptyList", "kotlin.collections.mutableListOf",
        "kotlin.collections.setOf", "kotlin.collections.emptySet", "kotlin.collections.mapOf",
        "kotlin.collections.emptyMap", "kotlin.to", "kotlin.Unit.<init>",
    )

    fun libraryAssertion(id: CallableId): Boolean =
        isJvmAssertion(id) || isAssertionFrameworkCall(id) || isMockCall(id) || isTurbineCall(id) ||
            isAndroidCall(id) || isBuildAndCompilerCall(id) || isReactiveCall(id)

    fun builtinCheck(id: CallableId): Boolean = id.packageName.asString() == "kotlin" &&
        id.callableName.asString() in setOf("assert", "require", "requireNotNull", "check", "checkNotNull")

    fun builtinError(id: CallableId): Boolean =
        id.packageName.asString() == "kotlin" && id.callableName.asString() == "error"

    fun trailingCheck(body: FirBlock, depth: Int): Boolean =
        trailingResultCheck(TestWithoutAssertionTestBody.resultOf(body), depth)
}

private fun isMockCall(id: CallableId): Boolean =
    (inPackage(id.packageName.asString(), "io.mockk") || inPackage(id.packageName.asString(), "org.mockito")) &&
        id.callableName.asString() in TestWithoutAssertionCalls.verifyNames

private fun isTurbineCall(id: CallableId): Boolean =
    inPackage(id.packageName.asString(), "app.cash.turbine") &&
        id.callableName.asString() in TestWithoutAssertionCalls.turbineNames

internal fun inPackage(actual: String, expected: String): Boolean =
    actual == expected || actual.startsWith("$expected.")
