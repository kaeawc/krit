package dev.jasonpearson.krit.fir.tests

import dev.jasonpearson.krit.fir.FirRuleCompileContext
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

// SimpleDateFormat cases on ICU4J's com.ibm.icu.text.SimpleDateFormat. A
// single-file golden cannot declare the Java library, so these sources carry
// their own Java declarations of the ICU4J classes, with the real constructor
// shapes. Go reports every call spelled SimpleDateFormat with fewer than two
// arguments; ICU4J's () and (String) constructors do use the default format
// locale, so those findings are true and FIR keeps them.
class SimpleDateFormatIcu4jTest {

    private val icu4j = mapOf(
        "com/ibm/icu/util/ULocale.java" to """
            package com.ibm.icu.util;

            public final class ULocale {
                public static final ULocale US = new ULocale("en_US");

                public ULocale(String localeID) {
                }
            }
        """.trimIndent(),
        "com/ibm/icu/text/UFormat.java" to """
            package com.ibm.icu.text;

            public abstract class UFormat extends java.text.Format {
                public UFormat() {
                }
            }
        """.trimIndent(),
        "com/ibm/icu/text/DateFormatSymbols.java" to """
            package com.ibm.icu.text;

            import java.util.Locale;

            public class DateFormatSymbols {
                public DateFormatSymbols(Locale locale) {
                }
            }
        """.trimIndent(),
        "com/ibm/icu/text/DateFormat.java" to """
            package com.ibm.icu.text;

            import java.text.FieldPosition;
            import java.text.ParsePosition;
            import java.util.Date;

            public abstract class DateFormat extends UFormat {
                protected DateFormat() {
                }

                public final StringBuffer format(Object obj, StringBuffer toAppendTo, FieldPosition fieldPosition) {
                    throw new RuntimeException("Stub!");
                }

                public final String format(Date date) {
                    throw new RuntimeException("Stub!");
                }

                public Object parseObject(String source, ParsePosition pos) {
                    throw new RuntimeException("Stub!");
                }
            }
        """.trimIndent(),
        "com/ibm/icu/text/SimpleDateFormat.java" to """
            package com.ibm.icu.text;

            import com.ibm.icu.util.ULocale;
            import java.util.Locale;

            public class SimpleDateFormat extends DateFormat {
                public SimpleDateFormat() {
                }

                public SimpleDateFormat(String pattern) {
                }

                public SimpleDateFormat(String pattern, Locale loc) {
                }

                public SimpleDateFormat(String pattern, ULocale loc) {
                }

                public SimpleDateFormat(String pattern, String override, ULocale loc) {
                }

                public SimpleDateFormat(String pattern, DateFormatSymbols formatData) {
                }

                public SimpleDateFormat(String pattern, DateFormatSymbols formatData, ULocale loc) {
                }
            }
        """.trimIndent(),
    )

    private fun findings(sources: Map<String, String>): List<Pair<String, Int>> {
        val result = KritFirProbe.compile(
            icu4j + sources,
            FirRuleCompileContext(enabledRuleIds = setOf("SimpleDateFormat")),
        )
        assertTrue(result.clean, result.problems())
        return result.diags.filter { it.name == "SimpleDateFormat" }.map { it.file to it.line }
    }

    // Go reports each of these by the call name. The imported and qualified
    // ICU4J calls use the default format locale; so does a project subclass
    // named SimpleDateFormat.
    @Test fun importedAndQualifiedIcu4jCallsReport() {
        val sources = mapOf(
            "Icu.kt" to """
                package demo

                import com.ibm.icu.text.SimpleDateFormat
                import java.util.Date

                fun imported(d: Date): String = SimpleDateFormat("yyyy").format(d)

                fun qualified(): Any = com.ibm.icu.text.SimpleDateFormat()

                class SimpleDateFormatHolder {
                    class SimpleDateFormat(pattern: String) : com.ibm.icu.text.SimpleDateFormat(pattern)

                    fun subclass(): Any = SimpleDateFormat("yyyy")
                }
            """.trimIndent(),
        )
        assertEquals(listOf("Icu.kt" to 6, "Icu.kt" to 8, "Icu.kt" to 13), findings(sources))
    }

    // Two or more arguments are never reported, whatever the second one is;
    // Go does not report them either.
    @Test fun icu4jCallsWithTwoOrMoreArgumentsDoNotReport() {
        val sources = mapOf(
            "IcuNegative.kt" to """
                package demo

                import com.ibm.icu.text.DateFormatSymbols
                import com.ibm.icu.text.SimpleDateFormat
                import com.ibm.icu.util.ULocale
                import java.util.Locale

                fun javaLocale(): Any = SimpleDateFormat("yyyy", Locale.US)

                fun uLocale(): Any = SimpleDateFormat("yyyy", ULocale.US)

                fun symbols(): Any = SimpleDateFormat("yyyy", DateFormatSymbols(Locale.US))

                fun override(): Any = SimpleDateFormat("yyyy", "y=hanidec", ULocale.US)

                fun symbolsAndLocale(): Any = SimpleDateFormat("yyyy", DateFormatSymbols(Locale.US), ULocale.US)
            """.trimIndent(),
        )
        assertEquals(emptyList(), findings(sources))
    }
}
