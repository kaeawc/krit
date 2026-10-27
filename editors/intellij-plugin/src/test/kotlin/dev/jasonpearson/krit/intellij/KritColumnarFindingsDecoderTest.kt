package dev.jasonpearson.krit.intellij

import com.google.gson.JsonParseException
import com.google.gson.JsonParser
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
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

    @Test
    fun `byte columns decode from the base64 strings Go writes`() {
        // encoding/json marshals the []uint8 severityID and confidence
        // columns as base64 strings. This is a payload a real daemon sent
        // for analyze-buffer ("AQ==" = [1], "VQ==" = [85]).
        val payload = """
            {
                "files":["/repo/Foo.kt"],"ruleSets":["release-engineering"],
                "rules":["PrintlnInProduction"],
                "messages":["println/print in production code; use a logging framework instead."],
                "fileIdx":[0],"line":[3],"col":[2],"ruleSetIdx":[0],"ruleIdx":[0],
                "severityID":"AQ==","messageIdx":[0],"confidence":"VQ==",
                "fixStart":[0],"binaryFixStart":[0]
            }
        """.trimIndent()
        val f = KritColumnarFindingsDecoder.decode(JsonParser.parseString(payload).asJsonObject).single()
        assertEquals("warning", f.severity)
        assertEquals(0.85, f.confidence)
        assertEquals("PrintlnInProduction", f.rule)
        assertEquals(3, f.line)
    }

    @Test
    fun `base64 byte columns are unsigned and per row`() {
        // "AAI=" = [0, 2]; "AMg=" = [0, 200] (200 must not read as -56).
        val payload = """
            {
                "files":["/x.kt"],"ruleSets":["rs"],"rules":["r"],"messages":["a","b"],
                "fileIdx":[0,0],"line":[1,2],"col":[1,1],"ruleSetIdx":[0,0],"ruleIdx":[0,0],
                "severityID":"AAI=","messageIdx":[0,1],"confidence":"AMg="
            }
        """.trimIndent()
        val findings = KritColumnarFindingsDecoder.decode(JsonParser.parseString(payload).asJsonObject)
        assertEquals(listOf("info", "error"), findings.map { it.severity })
        assertEquals(listOf(0.0, 2.0), findings.map { it.confidence })
    }

    private fun row(severity: String, confidence: String, extra: String = "") = """
        {
            "files":["/x.kt"],"ruleSets":["rs"],"rules":["r"],"messages":["a"],
            "fileIdx":[0],"line":[1],"col":[1],"ruleSetIdx":[0],"ruleIdx":[0],
            "severityID":$severity,"messageIdx":[0],"confidence":$confidence$extra
        }
    """.trimIndent()

    @Test
    fun `a malformed byte column fails the decode instead of defaulting`() {
        // Defaulting would report every row as info with no confidence; the
        // throw makes the daemon caller fall back to the CLI.
        listOf(
            row("\"not base64!\"", "\"VQ==\""),
            row("\"AQ==\"", "\"%%%\""),
            row("{}", "\"VQ==\""),
            row("\"AQE=\"", "\"VQ==\""), // two severities for one row
        ).forEach { payload ->
            assertFailsWith<JsonParseException>(payload) {
                KritColumnarFindingsDecoder.decode(JsonParser.parseString(payload).asJsonObject)
            }
        }
    }

    @Test
    fun `fix references outside their pools fail the decode`() {
        listOf(
            row("\"AQ==\"", "\"VQ==\"", ",\"fixPool\":[],\"fixStart\":[1]"),
            row("\"AQ==\"", "\"VQ==\"", ",\"suggestedFixPool\":[],\"suggestedFixStart\":[1],\"suggestedFixCount\":[1]"),
        ).forEach { payload ->
            assertFailsWith<JsonParseException>(payload) {
                KritColumnarFindingsDecoder.decode(JsonParser.parseString(payload).asJsonObject)
            }
        }
    }
}
