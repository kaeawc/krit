package dev.jasonpearson.krit.fir.tests

import org.junit.jupiter.api.Test
import kotlin.test.assertEquals

// LocaleGetDefaultForFormatting cases a golden cannot express: goldens compare
// the set of reported lines, while Go parity compares the number of findings
// per line. The Go rule reports each withLocale call on the line where its
// call_expression starts, which for a qualified call is the receiver's first
// line, including across multi-line dot and safe-call chains.
class LocaleGetDefaultForFormattingTest {

    private fun findingsPerLine(source: String): Map<Int, Int> =
        KritFirProbe.diagnose(mapOf("Main.kt" to source))
            .filter { it.file == "Main.kt" && it.name == "LocaleGetDefaultForFormatting" }
            .groupingBy { it.line }
            .eachCount()

    @Test
    fun countsAndLinesMatchGo() {
        val source = """
            package lgdffcounts

            import java.time.ZoneOffset
            import java.time.format.DateTimeFormatter
            import java.util.Locale

            val twice = DateTimeFormatter.ISO_INSTANT.withLocale(Locale.getDefault()).withLocale(Locale.getDefault())

            val dotChain =
                DateTimeFormatter.ISO_INSTANT
                    .withZone(ZoneOffset.UTC)
                    .withLocale(Locale.getDefault())

            val safeChain =
                DateTimeFormatter.ISO_INSTANT
                    ?.withZone(ZoneOffset.UTC)
                    ?.withLocale(Locale.getDefault())
                    ?.withLocale(Locale.getDefault())

            val argumentOnLaterLine = DateTimeFormatter.RFC_1123_DATE_TIME.withLocale(
                Locale.getDefault(),
            )
        """.trimIndent()
        assertEquals(mapOf(7 to 2, 10 to 1, 15 to 2, 20 to 1), findingsPerLine(source))
    }
}
