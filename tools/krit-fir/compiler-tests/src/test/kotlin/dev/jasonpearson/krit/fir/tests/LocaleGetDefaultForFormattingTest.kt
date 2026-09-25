package dev.jasonpearson.krit.fir.tests

import dev.jasonpearson.krit.fir.FirRuleCompileContext
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

// LocaleGetDefaultForFormatting cases a golden cannot express: goldens compare
// the set of reported lines in one file, while Go parity compares the number
// of findings per line, and some cases need a second source file (a type alias
// declared elsewhere, a third-party Java formatter).
class LocaleGetDefaultForFormattingTest {

    private fun findingsPerLine(source: String): Map<Int, Int> =
        KritFirProbe.diagnose(mapOf("Main.kt" to source))
            .filter { it.file == "Main.kt" && it.name == "LocaleGetDefaultForFormatting" }
            .groupingBy { it.line }
            .eachCount()

    // Compiles [sources] with only this rule enabled; `.java` keys are Java
    // sources (the probe's source directory is also a Java source root).
    private fun findings(sources: Map<String, String>): List<Pair<String, Int>> {
        val result = KritFirProbe.compile(
            sources,
            FirRuleCompileContext(enabledRuleIds = setOf("LocaleGetDefaultForFormatting")),
            configure = { args -> args.javaSourceRoots = (args.javaSourceRoots.orEmpty().toList() + args.freeArgs).toTypedArray() },
        )
        assertTrue(result.clean, result.problems())
        return result.diags
            .filter { it.name == "LocaleGetDefaultForFormatting" }
            .map { it.file to it.line }
            .sortedWith(compareBy({ it.first }, { it.second }))
    }

    // The Go rule reports each withLocale call on the line where its
    // call_expression starts, which for a qualified call is the receiver's
    // first line, including across multi-line dot and safe-call chains.
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

    // The ThreeTenBP backport's DateTimeFormatter (a Java library, declared
    // here with its real shape: static constants and an instance withLocale).
    private val threeTenBp = """
        package org.threeten.bp.format;

        import java.util.Locale;

        public final class DateTimeFormatter {
            public static final DateTimeFormatter ISO_INSTANT = new DateTimeFormatter();
            public static final DateTimeFormatter RFC_1123_DATE_TIME = new DateTimeFormatter();
            public static final DateTimeFormatter BASIC_ISO_DATE = new DateTimeFormatter();
            private DateTimeFormatter() {}
            public static DateTimeFormatter ofPattern(String pattern) { return new DateTimeFormatter(); }
            public DateTimeFormatter withLocale(Locale locale) { return this; }
        }
    """.trimIndent()

    // Go reports the ThreeTenBP ISO constant once the file mentions
    // java.time.format.DateTimeFormatter (here in a comment), because the
    // receiver is spelled `DateTimeFormatter.ISO_`; the receiver really is the
    // backport's machine-readable ISO formatter, so FIR reports it too. The
    // backport's ofPattern formatter is user-facing, and neither reports it.
    @Test
    fun threeTenBpFormatterInFileMentioningJavaTime() {
        val sources = mapOf(
            "org/threeten/bp/format/DateTimeFormatter.java" to threeTenBp,
            "Migrated.kt" to """
                package lgdffthreeten

                import org.threeten.bp.format.DateTimeFormatter
                import java.util.Locale

                // Migrated off java.time.format.DateTimeFormatter for API < 26.
                val t = DateTimeFormatter.ISO_INSTANT.withLocale(Locale.getDefault())
                val rfc = DateTimeFormatter.RFC_1123_DATE_TIME.withLocale(Locale.getDefault())
                val basic = DateTimeFormatter.BASIC_ISO_DATE.withLocale(Locale.getDefault())
                val pattern = DateTimeFormatter.ofPattern("EEEE d MMMM").withLocale(Locale.getDefault())
            """.trimIndent(),
        )
        assertEquals(listOf("Migrated.kt" to 7, "Migrated.kt" to 8, "Migrated.kt" to 9), findings(sources))
    }

    // Go misses this: without a mention of java.time.format.DateTimeFormatter
    // its file gate fails. The receiver is still the backport's ISO
    // formatter, so FIR reports it (a true positive Go misses).
    @Test
    fun threeTenBpFormatterWithoutJavaTimeMention() {
        val sources = mapOf(
            "org/threeten/bp/format/DateTimeFormatter.java" to threeTenBp,
            "Backport.kt" to """
                package lgdffthreetenonly

                import org.threeten.bp.format.DateTimeFormatter
                import java.util.Locale

                val t = DateTimeFormatter.ISO_INSTANT.withLocale(Locale.getDefault())
            """.trimIndent(),
        )
        assertEquals(listOf("Backport.kt" to 6), findings(sources))
    }

    // Go misses both: a same-package type alias of DateTimeFormatter declared
    // in another file leaves the using file without a mention of
    // java.time.format.DateTimeFormatter (Go's file gate), and a type alias of
    // java.util.Locale changes the argument's spelling. FIR resolves both to
    // the java.time ISO constant and java.util.Locale.getDefault().
    @Test
    fun typeAliasesDeclaredInAnotherFile() {
        val sources = mapOf(
            "Aliases.kt" to """
                package lgdffaliases

                typealias DateTimeFormatter = java.time.format.DateTimeFormatter
                typealias L = java.util.Locale
            """.trimIndent(),
            "FormatterAlias.kt" to """
                package lgdffaliases

                import java.util.Locale

                val t = DateTimeFormatter.ISO_INSTANT.withLocale(Locale.getDefault())
            """.trimIndent(),
            "LocaleAlias.kt" to """
                package lgdffaliases

                val u = java.time.format.DateTimeFormatter.ISO_DATE.withLocale(L.getDefault())
            """.trimIndent(),
        )
        assertEquals(listOf("FormatterAlias.kt" to 5, "LocaleAlias.kt" to 3), findings(sources))
    }
}
