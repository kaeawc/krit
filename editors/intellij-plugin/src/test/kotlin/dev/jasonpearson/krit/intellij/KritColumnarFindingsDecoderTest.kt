package dev.jasonpearson.krit.intellij

import com.google.gson.JsonParser
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

class KritColumnarFindingsDecoderTest {
    @Test
    fun `null payload decodes to empty list`() {
        assertTrue(KritColumnarFindingsDecoder.decode(null).isEmpty())
    }

    @Test
    fun `empty columns decode to empty list`() {
        val json = JsonParser.parseString("{}").asJsonObject
        assertTrue(KritColumnarFindingsDecoder.decode(json).isEmpty())
    }

    @Test
    fun `single-finding columnar payload decodes to flat finding`() {
        // Mirrors the wire shape internal/scanner/findings_json.go writes.
        // Two index columns dereference into the file/rule/message pools;
        // severity is the uint8 id (1 = warning); confidence is 0..100
        // scaled back to 0.0..1.0.
        val json = JsonParser.parseString(
            """
            {
                "files": ["/repo/Foo.kt"],
                "ruleSets": ["release-engineering"],
                "rules": ["PrintlnInProduction"],
                "messages": ["println/print in production code"],
                "fileIdx": [0],
                "line": [7],
                "col": [12],
                "ruleSetIdx": [0],
                "ruleIdx": [0],
                "severityID": [1],
                "messageIdx": [0],
                "confidence": [85],
                "n": 1
            }
            """.trimIndent(),
        ).asJsonObject
        val findings = KritColumnarFindingsDecoder.decode(json)
        assertEquals(1, findings.size)
        val f = findings.single()
        assertEquals("/repo/Foo.kt", f.file)
        assertEquals(7, f.line)
        assertEquals(12, f.column)
        assertEquals("release-engineering", f.ruleSet)
        assertEquals("PrintlnInProduction", f.rule)
        assertEquals("warning", f.severity)
        assertEquals("println/print in production code", f.message)
        assertEquals(0.85, f.confidence)
    }

    @Test
    fun `severity id 0 decodes to info, 2 decodes to error`() {
        // Pin the severity ID mapping. If the Go side renumbers the
        // severity enum, this test catches the wire-format drift.
        val payload = """
            {
                "files":["/x.kt"],"ruleSets":["rs"],"rules":["r"],
                "messages":["a","b"],
                "fileIdx":[0,0],"line":[1,2],"col":[1,1],
                "ruleSetIdx":[0,0],"ruleIdx":[0,0],
                "severityID":[0,2],"messageIdx":[0,1],"confidence":[0,0],
                "n":2
            }
        """.trimIndent()
        val findings = KritColumnarFindingsDecoder.decode(JsonParser.parseString(payload).asJsonObject)
        assertEquals(listOf("info", "error"), findings.map { it.severity })
    }

    @Test
    fun `decoder infers row count from fileIdx when n is absent`() {
        // The Go side omits 'n' when zero, so the decoder must fall back
        // to fileIdx length to find the row count.
        val payload = """
            {
                "files":["/x.kt"],"ruleSets":["rs"],"rules":["r"],
                "messages":["m"],
                "fileIdx":[0,0,0],"line":[1,2,3],"col":[1,1,1],
                "ruleSetIdx":[0,0,0],"ruleIdx":[0,0,0],
                "severityID":[1,1,1],"messageIdx":[0,0,0],"confidence":[50,50,50]
            }
        """.trimIndent()
        val findings = KritColumnarFindingsDecoder.decode(JsonParser.parseString(payload).asJsonObject)
        assertEquals(3, findings.size)
        assertEquals(listOf(1, 2, 3), findings.map { it.line })
    }
}
