package dev.krit.types

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

class CallFilterTest {
    @Test
    fun nameOnlyFilterKeepsExistingBehaviorWhenHintsAreAbsent() {
        val filter = CallFilter(
            enabled = true,
            calleeNames = setOf("launch")
        )

        assertTrue(filter.shouldResolve(site("launch")))
        assertFalse(filter.shouldResolve(site("update")))
    }

    @Test
    fun lexicalHintsRequireCheapEvidenceForBroadCallee() {
        val filter = CallFilter(
            enabled = true,
            calleeNames = setOf("launch"),
            lexicalHintsByCallee = mapOf("launch" to setOf("kotlinx.coroutines", "CoroutineScope"))
        )

        assertFalse(filter.shouldResolve(site("launch")))
        assertTrue(filter.shouldResolve(site("launch", imports = setOf("kotlinx.coroutines.launch"))))
        assertTrue(filter.shouldResolve(site("launch", imports = setOf("kotlinx.coroutines.*"))))
        assertTrue(filter.shouldResolve(site("launch", receiverText = "CoroutineScope")))
        assertTrue(filter.shouldResolve(site("launch", packageName = "kotlinx.coroutines.test")))
    }

    @Test
    fun exactReceiverAndStarImportMatchQualifiedHints() {
        val filter = CallFilter(
            enabled = true,
            calleeNames = setOf("open"),
            lexicalHintsByCallee = mapOf("open" to setOf("android.hardware.Camera"))
        )

        assertTrue(filter.shouldResolve(site("open", receiverText = "Camera")))
        assertTrue(filter.shouldResolve(site("open", receiverText = "android.hardware.Camera")))
        assertTrue(filter.shouldResolve(site("open", imports = setOf("android.hardware.*"))))
        assertFalse(filter.shouldResolve(site("open", receiverText = "file")))
    }

    @Test
    fun lexicalSkipHintsRejectStructurallyHandledReceivers() {
        val filter = CallFilter(
            enabled = true,
            calleeNames = setOf("w", "show"),
            lexicalSkipByCallee = mapOf(
                "w" to setOf("Log", "Timber"),
                "show" to setOf("Snackbar")
            )
        )

        assertFalse(filter.shouldResolve(site("w", receiverText = "Log")))
        assertFalse(filter.shouldResolve(site("w", receiverText = "Timber.tag(TAG)")))
        assertFalse(filter.shouldResolve(site("show", receiverText = "Snackbar.make(view, text, length)")))
        assertTrue(filter.shouldResolve(site("show", receiverText = "dialog")))
    }

    @Test
    fun profileAwareFilteringResolvesWhenAnyRuleStillNeedsTheCallee() {
        val filter = CallFilter(
            enabled = true,
            calleeNames = setOf("get"),
            ruleProfiles = listOf(
                RuleTargetProfile(
                    ruleID = "MapGet",
                    allCalls = false,
                    discardedOnly = false,
                    calleeNames = setOf("get"),
                    targetFqns = emptySet(),
                    lexicalHintsByCallee = emptyMap(),
                    lexicalSkipByCallee = mapOf("get" to setOf("*")),
                    annotatedIdentifiers = emptySet(),
                    derivedCalleeNames = emptySet(),
                    disabledReason = ""
                ),
                RuleTargetProfile(
                    ruleID = "AnnotatedGet",
                    allCalls = false,
                    discardedOnly = false,
                    calleeNames = setOf("get"),
                    targetFqns = emptySet(),
                    lexicalHintsByCallee = emptyMap(),
                    lexicalSkipByCallee = emptyMap(),
                    annotatedIdentifiers = emptySet(),
                    derivedCalleeNames = emptySet(),
                    disabledReason = ""
                )
            )
        )

        assertTrue(filter.shouldResolve(site("get", receiverText = "map")))
    }

    @Test
    fun discardedOnlyProfilesSkipCallsUsedAsExpressions() {
        val filter = CallFilter(
            enabled = true,
            calleeNames = setOf("update"),
            ruleProfiles = listOf(
                RuleTargetProfile(
                    ruleID = "IgnoredReturnValue",
                    allCalls = false,
                    discardedOnly = true,
                    calleeNames = setOf("update"),
                    targetFqns = emptySet(),
                    lexicalHintsByCallee = emptyMap(),
                    lexicalSkipByCallee = emptyMap(),
                    annotatedIdentifiers = emptySet(),
                    derivedCalleeNames = emptySet(),
                    disabledReason = ""
                )
            )
        )

        assertFalse(filter.shouldResolve(site("update", discarded = false)))
        assertTrue(filter.shouldResolve(site("update", discarded = true)))
    }

    @Test
    fun parsesLexicalHintMapFromFilterJson() {
        val json = """{"lexicalHintsByCallee":{"launch":["kotlinx.coroutines","CoroutineScope"],"open":["android.hardware.Camera"]}}"""

        val got = extractJsonStringArrayMap(json, "lexicalHintsByCallee") ?: error("missing lexical hint map")

        assertEquals(listOf("kotlinx.coroutines", "CoroutineScope"), got["launch"])
        assertEquals(listOf("android.hardware.Camera"), got["open"])
    }

    private fun site(
        callee: String,
        receiverText: String? = null,
        packageName: String = "com.example",
        imports: Set<String> = emptySet(),
        discarded: Boolean = true
    ): LexicalCallSite = LexicalCallSite(
        callee = callee,
        receiverText = receiverText,
        packageName = packageName,
        imports = imports,
        discarded = discarded
    )
}
