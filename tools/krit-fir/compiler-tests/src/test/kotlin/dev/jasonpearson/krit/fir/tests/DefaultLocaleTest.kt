package dev.jasonpearson.krit.fir.tests

import org.junit.jupiter.api.Test
import kotlin.test.assertEquals

// DefaultLocale cases a golden cannot express: goldens compare the set of
// reported lines, while Go parity compares the number of findings per line.
// The Go rule reports each call on the line where its call_expression starts,
// which for a qualified call is the receiver's first line.
class DefaultLocaleTest {

    private fun findingsPerLine(source: String): Map<Int, Int> =
        KritFirProbe.diagnose(mapOf("Main.kt" to source))
            .filter { it.file == "Main.kt" && it.name == "DefaultLocale" }
            .groupingBy { it.line }
            .eachCount()

    @Test
    fun countsAndLinesMatchGo() {
        val source = """
            package dlcounts

            fun nested(value: Int): String = String.format("%s", String.format("%d", value))

            fun split(value: Int): String =
                String
                    .format(
                        "%s",
                        String.format("%d", value),
                    )

            @Suppress("DEPRECATION_ERROR")
            fun chain(s: String?): String? =
                s
                    ?.toLowerCase()
                    ?.toUpperCase()
        """.trimIndent()
        assertEquals(mapOf(3 to 2, 6 to 1, 9 to 1, 14 to 2), findingsPerLine(source))
    }
}
