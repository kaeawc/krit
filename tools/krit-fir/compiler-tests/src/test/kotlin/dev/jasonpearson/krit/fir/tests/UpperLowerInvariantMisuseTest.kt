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

    // A project `String.uppercase()` / `lowercase()` brought in by an explicit
    // or a star import beats the default import of kotlin.text, so the call is
    // the project function, which takes no Locale. Go reports both calls
    // because it matches the spelled name; FIR does not. The unimported name
    // is still the stdlib conversion. Goldens are single-file, so this needs
    // the probe.
    @Test
    fun importedProjectExtensionsAreNotTheStdlib() {
        val other = """
            package ulimother

            fun String.uppercase(): String = this + "!"

            fun String.lowercase(): String = this + "?"
        """.trimIndent()
        val explicit = """
            package ulimexplicit

            import ulimother.uppercase

            fun explicitImport(userName: String): String = userName.uppercase()

            fun stillStdlib(email: String): String = email.lowercase()
        """.trimIndent()
        val star = """
            package ulimstar

            import ulimother.*

            fun starImport(userName: String, email: String): String = userName.uppercase() + email.lowercase()
        """.trimIndent()
        val diags = KritFirProbe.diagnose(mapOf("Other.kt" to other, "Explicit.kt" to explicit, "Star.kt" to star))
            .filter { it.name == "UpperLowerInvariantMisuse" }
            .map { it.file to it.line }
        assertEquals(listOf("Explicit.kt" to 7), diags)
    }
}
