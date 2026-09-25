package dev.jasonpearson.krit.fir.tests

import org.junit.jupiter.api.Test
import kotlin.test.assertEquals

// UpperLowerInvariantMisuse cases a golden cannot express: goldens compare the
// set of reported lines, while Go parity compares the number of findings per
// line. The Go rule reports each call on the line where its call_expression
// starts, which for a qualified call is the receiver's first line.
class UpperLowerInvariantMisuseTest {

    private fun findingsPerLine(source: String): Map<Int, Int> =
        KritFirProbe.diagnose(mapOf("Main.kt" to source))
            .filter { it.file == "Main.kt" && it.name == "UpperLowerInvariantMisuse" }
            .groupingBy { it.line }
            .eachCount()

    @Test
    fun countsAndLinesMatchGo() {
        val source = """
            package ulimcounts

            fun nested(label: String): String = label.uppercase().lowercase()

            fun chain(title: String?): String? =
                title
                    ?.trim()
                    ?.uppercase()
                    ?.lowercase()

            fun receiverSpansLines(parts: List<String>): String = listOf(
                parts.first(),
                parts.last(),
            ).joinToString().lowercase()

            fun exemptOnLaterLine(parts: List<String>): String = listOf(
                parts.first(),
                "currencyCode",
            ).joinToString().uppercase()
        """.trimIndent()
        assertEquals(mapOf(3 to 2, 6 to 2, 11 to 1), findingsPerLine(source))
    }
}
