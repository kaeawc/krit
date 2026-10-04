package dev.jasonpearson.krit.fir.tests

import org.junit.jupiter.api.Test
import kotlin.test.assertEquals

// UnicodeNormalizationMissing cases a golden cannot express: goldens compare
// the set of reported lines, while Go parity compares the number of findings
// per line. The Go rule reports each contains() call on the line where its
// call_expression starts, which for a qualified call is the receiver's first
// line.
class UnicodeNormalizationMissingTest {

    private fun findingsPerLine(source: String): Map<Int, Int> =
        KritFirProbe.diagnose(mapOf("Main.kt" to source))
            .filter { it.file == "Main.kt" && it.name == "UnicodeNormalizationMissing" }
            .groupingBy { it.line }
            .eachCount()

    @Test
    fun countsAndLinesMatchGo() {
        val source = """
            package unmcounts

            fun findBoth(a: String, b: String, q: String): Boolean = a.contains(q) || b.contains(q)

            fun findNested(a: String, b: String, q: String): Boolean = a.contains(b.contains(q).toString())

            fun searchChain(title: String?, q: String): Boolean =
                title
                    ?.trim()
                    ?.contains(q)
                    ?.toString()
                    ?.contains(q) == true

            fun findWithMatcher(q: String, contains: (String) -> Boolean): Boolean = contains(q) && contains(q)
        """.trimIndent()
        assertEquals(mapOf(3 to 2, 5 to 2, 8 to 2, 14 to 2), findingsPerLine(source))
    }
}
