package dev.jasonpearson.krit.fir

import dev.jasonpearson.krit.fir.runner.AnalysisSession
import java.io.File
import kotlin.test.assertEquals
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
}
