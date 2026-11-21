package dev.jasonpearson.krit.fir.support

import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.lexer.KtTokens

internal object GoTestAssertionNames {
    val assertionCallNames = setOf(
        "assertEquals", "assertNotEquals", "assertTrue", "assertFalse", "assertNull", "assertNotNull",
        "assertSame", "assertNotSame", "assertThat", "assertThrows", "assertFailsWith", "assertIs",
        "assertIsNot", "assertContains", "assertContentEquals", "fail", "expectThat", "expect",
        "expectClean", "expectErrorCount", "expectWarningCount", "expectFixDiffs", "expectFixes",
        "expectMatches", "expectContains", "isEqualTo", "isNotEqualTo", "isSameInstanceAs",
        "isNotSameInstanceAs", "isNull", "isNotNull", "isTrue", "isFalse", "containsExactly",
        "containsAtLeast", "containsNoneOf", "hasSize", "isEmpty", "isNotEmpty", "shouldBe",
        "shouldNotBe", "shouldThrow", "shouldNotThrow", "buildAndAssertThatOutput",
        "buildAndFailAndAssertThatOutput", "printPluginsAndAssertOutput", "check", "snapshot",
        "captureRoboImage", "captureToImage", "measureRepeated", "waitUntil", "testExecute",
        "testEnqueue", "complete", "intended", "intending",
    )
    val verifyCallNames = setOf(
        "verify", "coVerify", "confirmVerified", "verifyNoInteractions", "verifyNoMoreInteractions",
        "verifyBlocking", "verifyOrder",
    )
    val turbineAssertionCalls = setOf(
        "awaitItem", "awaitComplete", "awaitError", "expectMostRecentItem", "expectNoEvents",
        "ensureAllEventsConsumed", "cancelAndConsumeRemainingEvents", "cancelAndIgnoreRemainingEvents",
    )
    val runnerCalls = setOf(
        "runTest", "runBlocking", "runBlockingTest",
    )
    val throwingCheckCalls = setOf(
        "checkNotNull", "requireNotNull", "assertIs", "assertIsNot", "check", "require",
    )
    val waitHelpersNeedingEvidence = setOf(
        "waitForIdle", "waitForIdleSync", "waitUntilAtLeastOneExists", "waitUntilExactlyOneExists",
        "waitUntilDoesNotExist", "waitFor",
    )
    val benchmarkImports = setOf(
        "androidx.benchmark.macro.junit4.MacrobenchmarkRule", "androidx.benchmark.macro.junit4.BaselineProfileRule",
        "androidx.benchmark.junit4.BenchmarkRule", "androidx.compose.testutils.benchmark.ComposeBenchmarkRule",
        "com.github.takahirom.roborazzi.RoborazziRule",
    )
    val equivalentTokens = listOf(
        "assertion", "assert", "check", "verification", "verify", "expectation", "expect", "snapshot", "await",
    )
    val frameworkNames = setOf(
        "compile", "compileKotlinAndFail", "build", "waitFor", "expectError", "test", "testWithLifecycle",
        "runTest", "isDisplayed", "doesNotExist", "gif", "testContent", "isNotDisplayed",
        "waitUntilAtLeastOneExists", "waitUntilExactlyOneExists", "waitUntilDoesNotExist", "matches",
    )
    val noExceptionMarkers = listOf(
        "no crash", "not crash", "without crash", "without crashing", "does not crash", "doesn't crash",
        "doesnt crash", "no exception", "not throw", "without throw", "without throwing", "does not throw",
        "doesn't throw", "doesnt throw", "completes without", "robust", "handles any", "works for any",
    )
    val smokePhrases = listOf(
        "shouldn't crash", "should not crash", "doesn't crash", "does not crash", "shouldn't throw",
        "should not throw", "doesn't throw", "does not throw", "no exception", "without throwing",
        "without crashing", "without crash", "no crash", "does not cause an exception",
        "doesn't cause an exception", "shouldn't fail", "should not fail", "doesn't fail", "does not fail",
    )
    val composeSmokeCommentWords = listOf(
        "without", "reaching here", "reach here", "completes", "completed", "crash", "throw", "exception",
        "survive", "measure", "layout", "no error",
    )
    private val specialNames = setOf(
        "throw", "error", "TODO", "NotImplementedError", "AssertionError", "AssertionFailedError", "this", "super",
    )

    fun exempts(name: String, patterns: List<String>): Boolean =
        namedAssertion(name) || conventionAssertion(name) || containsAssertionToken(name) ||
            matchesPattern(name, patterns)

    private fun namedAssertion(name: String): Boolean =
        name in assertionCallNames || name in verifyCallNames || name in turbineAssertionCalls ||
            name in frameworkNames ||
            name in specialNames

    private fun conventionAssertion(name: String): Boolean =
        name.startsWith("assert") || name.startsWith("verify") ||
            name.startsWith("validate") ||
            name.startsWith("should") || name.endsWith("RoboImage") || name.startsWith("ensure") ||
            name.startsWith("waitFor") || name.startsWith("waitUntil") ||
            name.startsWith("expect") || name.startsWith("await")

    private fun containsAssertionToken(name: String): Boolean = equivalentTokens.any { containsToken(name, it) }

    private fun matchesPattern(name: String, patterns: List<String>): Boolean =
        patterns.any { pattern -> goNameGlobMatches(pattern.trim(), name) }

    fun textExempts(text: String): Boolean =
        "AssertionError" in text || verifyCallNames.any { "$it(" in text }

    private fun containsToken(name: String, token: String): Boolean {
        val lower = name.lowercase()
        var offset = lower.indexOf(token)
        while (offset >= 0) {
            val end = offset + token.length
            val before = offset == 0 || !name[offset - 1].isLetterOrDigit() ||
                name[offset - 1] in 'a'..'z' && name[offset] in 'A'..'Z'
            val after = end == name.length || !name[end].isLetterOrDigit() || name[end] in 'A'..'Z'
            if (before && after) return true
            offset = lower.indexOf(token, end)
        }
        return false
    }

    fun identifiers(source: KtSourceElement?): Set<String> {
        if (source == null) return emptySet()
        val names = HashSet<String>()
        fun collect(node: com.intellij.lang.LighterASTNode) {
            if (node.tokenType == KtTokens.IDENTIFIER) names += lightText(source, node).trim('`')
            if (node.tokenType == KtTokens.THROW_KEYWORD) names += "throw"
            if (node.tokenType == KtTokens.THIS_KEYWORD) names += "this"
            if (node.tokenType == KtTokens.SUPER_KEYWORD) names += "super"
            lightChildren(source, node).forEach(::collect)
        }
        collect(source.lighterASTNode)
        return names
    }
}
