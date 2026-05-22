package dev.jasonpearson.krit.intellij

import kotlin.test.Test
import kotlin.test.assertEquals

class KritJsonParserConfidenceTest {
    @Test
    fun `parser populates confidence from JSON`() {
        val json = """
            {"findings":[{
                "file":"X.kt","line":1,"column":1,
                "ruleSet":"rs","rule":"r","severity":"warning","message":"m",
                "confidence":0.3
            }]}
        """.trimIndent()
        val report = KritJsonParser.parse(json)
        assertEquals(1, report.findings.size)
        assertEquals(0.3, report.findings.single().confidence)
    }

    @Test
    fun `parser defaults confidence to zero when absent`() {
        // Krit omits confidence when unset (omitempty on the Go side).
        // The default must be 0.0 so severity mapping doesn't treat
        // missing confidence as "very low confidence".
        val json = """
            {"findings":[{
                "file":"X.kt","line":1,"column":1,
                "ruleSet":"rs","rule":"r","severity":"warning","message":"m"
            }]}
        """.trimIndent()
        assertEquals(0.0, KritJsonParser.parse(json).findings.single().confidence)
    }
}
