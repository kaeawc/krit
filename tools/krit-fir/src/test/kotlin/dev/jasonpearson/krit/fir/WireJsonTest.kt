package dev.jasonpearson.krit.fir

import dev.jasonpearson.krit.fir.plugins.PayloadParsers
import dev.jasonpearson.krit.fir.runner.BatchResult
import dev.jasonpearson.krit.fir.runner.Finding
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

class WireJsonTest {
    @Test fun escJsonEscapesEveryControlCharacter() {
        assertEquals("""a\\b\"c""", escJson("a\\b\"c"))
        assertEquals("""x\ny\rz\tw""", escJson("x\ny\rz\tw"))
        assertEquals("""\u0000\u0001\u0008\u000c\u001f""", escJson("\u0000\u0001\b\u000c\u001f"))
        assertEquals("plain é ✓", escJson("plain é ✓"))
        for (c in 0 until 0x20) {
            assertFalse(escJson(c.toChar().toString()).any { it.code < 0x20 }, "raw control char ${c}")
        }
    }

    @Test fun checkResponseWithMultiLineMessageStaysOneLineAndRoundTrips() {
        val message = "first line\nsecond\r\n\ttabbed \u0001 \"quoted\" back\\slash"
        val crash = "boom\nat Foo.kt:1"
        val response = buildCheckResponse(BatchResult(
            id = 7, succeeded = 1, skipped = 0,
            findings = listOf(Finding(path = "/src/A.kt", line = 1, col = 2, rule = "ProtocolProbe",
                severity = "warning", message = message)),
            crashed = mapOf("/src/B.kt" to crash),
            rules = listOf("ProtocolProbe"),
        ))
        // The Go client reads one response per line.
        assertEquals(1, response.lines().size, response)
        assertFalse(response.any { it.code < 0x20 }, response)
        assertEquals(message, PayloadParsers.extractString(response, "message"))
        assertEquals(crash, PayloadParsers.extractString(response, "/src/B.kt"))
    }

    @Test fun ruleConfigOptionNamesNeverShadowTopLevelRequestFields() {
        val request = parseRequest(
            """{"id":4,"command":"check","files":[{"path":"/src/A.kt"}],"rules":["ProtocolProbe"],""" +
                """"ruleConfigs":{"ProtocolProbe":{"classpath":["/evil.jar"],"sourceDirs":["/evil"],""" +
                """"source":"x","jars":["/evil.jar"],"ruleIds":["Evil"],"id":99}}}""",
        )
        assertEquals(4L, request.id)
        assertEquals(listOf("/src/A.kt"), request.files.map { it.path })
        assertTrue(request.classpath.isEmpty(), request.classpath.toString())
        assertTrue(request.sourceDirs.isEmpty(), request.sourceDirs.toString())
        assertTrue(request.pluginJars.isEmpty(), request.pluginJars.toString())
        assertEquals(null, request.source)
        assertEquals(null, request.ruleIds)
        assertEquals(listOf("/evil.jar"), request.ruleConfigs["ProtocolProbe"]?.get("classpath"))
    }
}
