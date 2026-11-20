package dev.jasonpearson.krit.fir

import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

class ListRulesTest {
    @Test fun listsDiscoveredRuleIdsOnePerLine() {
        val ids = listRulesOutput().lines().filter { it.isNotEmpty() }
        assertEquals(FirRuleDiscovery.rules.map { it.ruleId }, ids)
        assertEquals(ids.sorted(), ids)
        assertTrue("BadHostnameVerifier" in ids, "BadHostnameVerifier has a FIR checker")
        assertTrue(
            ids.containsAll(listOf(
                "CollectInOnCreateWithoutLifecycle", "ComposeRememberWithoutKey",
                "CoroutineScopeCreatedButNeverCancelled", "InjectDispatcher", "PlainFileWriteOfSensitive",
            )),
            ids.containsAll(listOf("BadHostnameVerifier", "CollectInOnCreateWithoutLifecycle", "ComposeRememberWithoutKey", "InjectDispatcher")),
            ids.containsAll(listOf("CollectInOnCreateWithoutLifecycle", "ComposeRememberWithoutKey", "InjectDispatcher")),
            ids.toString(),
        )
        assertTrue(listRulesOutput().endsWith("\n"))
    }
}
