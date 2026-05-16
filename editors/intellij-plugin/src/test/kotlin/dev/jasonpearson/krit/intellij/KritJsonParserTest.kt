package dev.jasonpearson.krit.intellij

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNotNull
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
    fun `parse suggested fixes preserves rule-defined order`() {
        val report = KritJsonParser.parse(
            """
            {
              "findings": [
                {
                  "file": "/repo/Example.kt",
                  "line": 5,
                  "column": 3,
                  "ruleSet": "style",
                  "rule": "Suggester",
                  "severity": "warning",
                  "message": "pick one",
                  "fixable": false,
                  "suggestedFixes": [
                    {
                      "id": "use-val",
                      "title": "Convert to val",
                      "edits": [
                        {"startLine":5,"endLine":5,"replacement":"val x = 1"}
                      ]
                    },
                    {
                      "id": "explain",
                      "title": "Explain the warning",
                      "detail": "var becomes val when read-only.",
                      "applicationToken": "help:val-vs-var"
                    }
                  ]
                }
              ]
            }
            """.trimIndent(),
        )

        val finding = report.findings.single()
        assertFalse(finding.fixable)
        assertEquals(2, finding.suggestedFixes.size)
        assertEquals("use-val", finding.suggestedFixes[0].id)
        assertEquals("Convert to val", finding.suggestedFixes[0].title)
        val firstEdit = finding.suggestedFixes[0].edits.single()
        assertEquals("val x = 1", firstEdit.replacement)
        assertEquals(5, firstEdit.startLine)

        assertEquals("explain", finding.suggestedFixes[1].id)
        assertEquals("var becomes val when read-only.", finding.suggestedFixes[1].detail)
        assertEquals("help:val-vs-var", finding.suggestedFixes[1].applicationToken)
        assertTrue(finding.suggestedFixes[1].edits.isEmpty())

        assertEquals("Suggester:/repo/Example.kt:5:3", finding.findingId)
    }

    @Test
    fun `suggestions absent defaults to empty list`() {
        val report = KritJsonParser.parse(
            """
            {"findings":[{"file":"A.kt","line":1,"column":1,"ruleSet":"x","rule":"R","severity":"warning","message":"m"}]}
            """.trimIndent(),
        )

        val finding = report.findings.single()
        assertNotNull(finding.suggestedFixes)
        assertTrue(finding.suggestedFixes.isEmpty())
    }

    @Test
    fun `intentions for finding with suggestions yield ordered per-suggestion intentions and skip autofix`() {
        val finding = KritFinding(
            file = "/repo/A.kt",
            line = 1,
            column = 1,
            ruleSet = "style",
            rule = "R",
            severity = "warning",
            message = "m",
            fixable = true,
            fixLevel = "idiomatic",
            suggestedFixes = listOf(
                KritSuggestedFix(
                    id = "a",
                    title = "Apply A",
                    edits = listOf(KritSuggestedEdit(startLine = 1, endLine = 1, replacement = "A")),
                ),
                KritSuggestedFix(
                    id = "b",
                    title = "Apply B",
                    edits = listOf(KritSuggestedEdit(startLine = 1, endLine = 1, replacement = "B")),
                ),
            ),
        )

        val actions = KritIntentions.forFinding(finding)
        assertEquals(2, actions.size)
        actions.forEach { assertTrue(it is KritApplySuggestionIntention) }
        assertEquals("Krit suggestion: Apply A", actions[0].text)
        assertEquals("Krit suggestion: Apply B", actions[1].text)
    }

    @Test
    fun `intentions for finding without suggestions fall back to autofix when fixable`() {
        val finding = KritFinding(
            file = "/repo/A.kt",
            line = 1,
            column = 1,
            ruleSet = "style",
            rule = "R",
            severity = "warning",
            message = "m",
            fixable = true,
            fixLevel = "cosmetic",
        )

        val actions = KritIntentions.forFinding(finding)
        assertEquals(1, actions.size)
        assertTrue(actions.single() is KritApplyFixesIntention)
        assertEquals("Apply Krit cosmetic auto-fixes", actions.single().text)
    }

    @Test
    fun `intentions filter out suggestions without machine-applicable edits`() {
        val finding = KritFinding(
            file = "/repo/A.kt",
            line = 1,
            column = 1,
            ruleSet = "style",
            rule = "R",
            severity = "warning",
            message = "m",
            suggestedFixes = listOf(
                KritSuggestedFix(id = "explain", title = "Explain", applicationToken = "help:x"),
                KritSuggestedFix(
                    id = "use-val",
                    title = "Convert to val",
                    edits = listOf(KritSuggestedEdit(startLine = 1, endLine = 1, replacement = "val x")),
                ),
            ),
        )

        val actions = KritIntentions.forFinding(finding)
        assertEquals(1, actions.size)
        assertEquals("Krit suggestion: Convert to val", actions.single().text)
    }

    @Test
    fun `intentions fall back to autofix when all suggestions are informational`() {
        val finding = KritFinding(
            file = "/repo/A.kt",
            line = 1,
            column = 1,
            ruleSet = "style",
            rule = "R",
            severity = "warning",
            message = "m",
            fixable = true,
            fixLevel = "idiomatic",
            suggestedFixes = listOf(
                KritSuggestedFix(id = "explain", title = "Explain", applicationToken = "help:x"),
            ),
        )

        val actions = KritIntentions.forFinding(finding)
        assertEquals(1, actions.size)
        assertTrue(actions.single() is KritApplyFixesIntention)
    }

    @Test
    fun `intentions for non-fixable finding with no suggestions are empty`() {
        val finding = KritFinding(
            file = "/repo/A.kt",
            line = 1,
            column = 1,
            ruleSet = "style",
            rule = "R",
            severity = "warning",
            message = "m",
        )

        assertTrue(KritIntentions.forFinding(finding).isEmpty())
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
