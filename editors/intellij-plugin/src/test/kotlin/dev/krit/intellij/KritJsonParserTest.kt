package dev.krit.intellij

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

class KritJsonParserTest {
    @Test
    fun `parse findings from krit json`() {
        val report = KritJsonParser.parse(
            """
            {
              "success": false,
              "findings": [
                {
                  "file": "/repo/src/main/kotlin/Example.kt",
                  "line": 3,
                  "column": 7,
                  "ruleSet": "style",
                  "rule": "ExampleRule",
                  "severity": "warning",
                  "message": "Example message",
                  "fixable": true
                }
              ]
            }
            """.trimIndent(),
        )

        assertEquals(1, report.findings.size)
        val finding = report.findings.single()
        assertEquals("ExampleRule", finding.rule)
        assertEquals("warning", finding.severity)
        assertEquals("Krit style/ExampleRule: Example message", finding.displayMessage)
        assertTrue(finding.fixable)
        assertEquals(null, finding.fixLevel)
    }

    @Test
    fun `invalid json is empty report`() {
        assertTrue(KritJsonParser.parse("not json").findings.isEmpty())
    }

    @Test
    fun `missing fixable defaults false`() {
        val report = KritJsonParser.parse(
            """
            {"findings":[{"file":"A.kt","line":1,"column":0,"ruleSet":"style","rule":"R","severity":"error","message":"m"}]}
            """.trimIndent(),
        )

        assertFalse(report.findings.single().fixable)
    }

    @Test
    fun `parse fix level for fixable findings`() {
        val report = KritJsonParser.parse(
            """
            {"findings":[{"file":"A.kt","line":1,"column":1,"ruleSet":"style","rule":"NewLineAtEndOfFile","severity":"warning","message":"m","fixable":true,"fixLevel":"cosmetic"}]}
            """.trimIndent(),
        )

        val finding = report.findings.single()
        assertTrue(finding.fixable)
        assertEquals("cosmetic", finding.fixLevel)
    }
}
