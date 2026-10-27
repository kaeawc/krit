package dev.jasonpearson.krit.fir

import dev.jasonpearson.krit.fir.runner.AnalysisSession
import java.io.File
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue
import org.junit.jupiter.api.Test

class CanaryTest {
    @Test fun canaryRequestReportsItsOneFindingAndCleansUp() {
        val tmp = File(System.getProperty("java.io.tmpdir"))
        val before = tmp.listFiles { f -> f.name.startsWith("krit-fir-canary-") }.orEmpty().size
        val session = AnalysisSession(emptyList(), emptyList())
        try {
            val result = handleRequestLine("""{"id":7,"command":"canary"}""", session, 0) as RequestResult.Response
            assertTrue(result.json.startsWith("""{"id":7,"""), result.json)
            assertEquals(1, Regex(""""rule":"$CANARY_RULE"""").findAll(result.json).count(), result.json)
            assertTrue(result.json.contains(CANARY_FILE), result.json)
        } finally {
            session.dispose()
        }
        assertEquals(before, tmp.listFiles { f -> f.name.startsWith("krit-fir-canary-") }.orEmpty().size)
    }

    @Test fun checkerCanaryOmitsOracleFacts() {
        val session = AnalysisSession(emptyList(), emptyList())
        try {
            val result = handleRequestLine("""{"id":3,"command":"canary"}""", session, 0) as RequestResult.Response
            assertFalse(result.json.contains("oracleCanary"), result.json)
        } finally {
            session.dispose()
        }
    }

    @Test fun oracleCanaryReportsTheCanaryOracleFacts() {
        val session = AnalysisSession(emptyList(), emptyList())
        try {
            val result = handleRequestLine("""{"id":8,"command":"canary","oracle":true}""", session, 0) as RequestResult.Response
            assertEquals(1, Regex(""""rule":"$CANARY_RULE"""").findAll(result.json).count(), result.json)
            val facts = Regex(""""oracleCanary":\{"packageName":"([^"]*)","expressions":(\d+)\}""").find(result.json)
            assertTrue(facts != null, result.json)
            assertEquals(CANARY_PACKAGE, facts.groupValues[1], result.json)
            assertTrue(facts.groupValues[2].toInt() > 0, result.json)
            assertTrue(result.json.endsWith("}"), result.json)
        } finally {
            session.dispose()
        }
    }

    @Test fun selfCheckCanaryPassesOnAHealthyJvm() {
        assertTrue(AotSelfCheck.canaryPassed)
    }

    @Test fun sharedCheckIsMarkedOnlyForAotLaunches() {
        val json = """{"id":1,"findings":[]}"""
        assertEquals(json, markSharedCheck(json, launchedFromCache = false) { error("canary must not run without AOT") })
        assertEquals("""{"id":1,"findings":[],"aotCanary":true}""", markSharedCheck(json, launchedFromCache = true) { true })
        assertNull(markSharedCheck(json, launchedFromCache = true) { false })
    }
}
