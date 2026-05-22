package dev.jasonpearson.krit.intellij

import com.intellij.codeInspection.ProblemHighlightType
import com.intellij.lang.annotation.HighlightSeverity
import kotlin.test.Test
import kotlin.test.assertEquals

class KritSeverityTest {
    private fun finding(severity: String, confidence: Double = 0.0): KritFinding =
        KritFinding(
            file = "X.kt",
            line = 1,
            column = 1,
            ruleSet = "rs",
            rule = "r",
            severity = severity,
            message = "m",
            confidence = confidence,
        )

    @Test
    fun `error severity wins over confidence`() {
        assertEquals(HighlightSeverity.ERROR, KritSeverity.highlightSeverity(finding("error", 0.1)))
        assertEquals(ProblemHighlightType.ERROR, KritSeverity.problemHighlightType(finding("error", 0.1)))
    }

    @Test
    fun `info severity wins over confidence`() {
        assertEquals(HighlightSeverity.INFORMATION, KritSeverity.highlightSeverity(finding("info", 0.1)))
    }

    @Test
    fun `low confidence maps to weak warning`() {
        assertEquals(HighlightSeverity.WEAK_WARNING, KritSeverity.highlightSeverity(finding("warning", 0.3)))
        assertEquals(
            ProblemHighlightType.WEAK_WARNING,
            KritSeverity.problemHighlightType(finding("warning", 0.3)),
        )
    }

    @Test
    fun `confidence at threshold renders as warning`() {
        // 0.5 is the boundary — equal-and-above gets normal warning weight.
        assertEquals(HighlightSeverity.WARNING, KritSeverity.highlightSeverity(finding("warning", 0.5)))
        assertEquals(HighlightSeverity.WARNING, KritSeverity.highlightSeverity(finding("warning", 0.9)))
    }

    @Test
    fun `confidence zero means unset and renders as warning`() {
        // Krit's JSON omits confidence when unset; default 0.0 should NOT be
        // treated as "very low confidence" or every finding without a
        // confidence score would be weakened.
        assertEquals(HighlightSeverity.WARNING, KritSeverity.highlightSeverity(finding("warning", 0.0)))
    }

    @Test
    fun `unknown severity strings default to warning`() {
        assertEquals(HighlightSeverity.WARNING, KritSeverity.highlightSeverity(finding("style")))
    }
}
