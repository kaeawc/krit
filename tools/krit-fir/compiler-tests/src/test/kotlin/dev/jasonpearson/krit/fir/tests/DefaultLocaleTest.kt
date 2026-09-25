package dev.jasonpearson.krit.fir.tests

import org.junit.jupiter.api.Test
import kotlin.test.assertEquals

// DefaultLocale cases a golden cannot express: goldens compare the set of
// reported lines, while Go parity compares the number of findings per line.
// The Go rule reports each call on the line where its call_expression starts,
// which for a qualified call is the receiver's first line.
class DefaultLocaleTest {

    private fun findingsPerLine(source: String, java: Map<String, String> = emptyMap()): Map<Int, Int> =
        KritFirProbe.diagnose(java + ("Main.kt" to source))
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

    // ICU's UCharacter.toLowerCase(String) / toUpperCase(String) use the
    // default locale (ICU documents the String overloads that way), in both
    // the Android platform copy and ICU4J. Go reports them by name, and they
    // are true positives. The ICU classes are declared here with their real
    // overloads, as Java sources, because the shared stub layer has no
    // android.icu.lang or com.ibm.icu package.
    @Test
    fun icuStringCaseConversionsAreReported() {
        val source = """
            package dlicu

            import android.icu.lang.UCharacter

            fun lower(s: String): String = UCharacter.toLowerCase(s)

            fun upper(s: String): String = UCharacter.toUpperCase(s)

            fun ibmLower(s: String): String = com.ibm.icu.lang.UCharacter.toLowerCase(s)

            fun ibmUpper(s: String): String = com.ibm.icu.lang.UCharacter.toUpperCase(s)

            fun split(s: String?): String =
                UCharacter
                    .toLowerCase(s)

            fun inLambda(values: List<String>): List<String> = values.map { UCharacter.toUpperCase(it) }
        """.trimIndent()
        assertEquals(mapOf(5 to 1, 7 to 1, 9 to 1, 11 to 1, 14 to 1, 17 to 1), findingsPerLine(source, icuAndGuava))
    }

    // Go reports every call below (by name, with no argument mentioning the
    // identifier `Locale`); FIR reports none, because none uses the default
    // locale, so the message is not true of them:
    // - the int code-point overloads map case without a locale;
    // - the (ULocale, String) overloads take an explicit locale (Go only
    //   recognizes the identifier `Locale`, not `ULocale`);
    // - Guava's Ascii maps only ASCII letters, with no locale.
    // The (Locale, String) overload is left alone by both.
    @Test
    fun localeIndependentAndExplicitIcuOverloadsAreNotReported() {
        val source = """
            package dlicuneg

            import android.icu.lang.UCharacter
            import android.icu.util.ULocale
            import com.google.common.base.Ascii
            import java.util.Locale

            fun codePointLower(c: Int): Int = UCharacter.toLowerCase(c)

            fun codePointUpper(c: Int): Int = com.ibm.icu.lang.UCharacter.toUpperCase(c)

            fun uLocale(s: String): String = UCharacter.toUpperCase(ULocale.ROOT, s)

            fun ibmULocale(s: String): String =
                com.ibm.icu.lang.UCharacter.toLowerCase(com.ibm.icu.util.ULocale.ROOT, s)

            fun uLocaleVariable(locale: ULocale, s: String): String = UCharacter.toLowerCase(locale, s)

            fun locale(s: String): String = UCharacter.toLowerCase(Locale.ROOT, s)

            fun asciiLower(s: String): String = Ascii.toLowerCase(s)

            fun asciiUpper(s: CharSequence): String = Ascii.toUpperCase(s)

            fun asciiChar(c: Char): Char = Ascii.toLowerCase(c)

            object UCharacterLookalike {
                fun toLowerCase(s: String): String = s
            }

            fun lookalike(s: String): String = UCharacterLookalike.toLowerCase(s)
        """.trimIndent()
        assertEquals(emptyMap(), findingsPerLine(source, icuAndGuava))
    }

    private companion object {
        private fun uCharacter(pkg: String, uLocale: String) = """
            package $pkg;

            import java.util.Locale;
            import $uLocale;

            public final class UCharacter {
                private UCharacter() {
                }

                public static int toLowerCase(int ch) {
                    return ch;
                }

                public static String toLowerCase(String str) {
                    return str;
                }

                public static String toLowerCase(Locale locale, String str) {
                    return str;
                }

                public static String toLowerCase(ULocale locale, String str) {
                    return str;
                }

                public static int toUpperCase(int ch) {
                    return ch;
                }

                public static String toUpperCase(String str) {
                    return str;
                }

                public static String toUpperCase(Locale locale, String str) {
                    return str;
                }

                public static String toUpperCase(ULocale locale, String str) {
                    return str;
                }
            }
        """.trimIndent()

        private fun uLocale(pkg: String) = """
            package $pkg;

            public final class ULocale {
                public static final ULocale ROOT = new ULocale();

                private ULocale() {
                }
            }
        """.trimIndent()

        private val ascii = """
            package com.google.common.base;

            public final class Ascii {
                private Ascii() {
                }

                public static String toLowerCase(String string) {
                    return string;
                }

                public static String toLowerCase(CharSequence chars) {
                    return chars.toString();
                }

                public static char toLowerCase(char c) {
                    return c;
                }

                public static String toUpperCase(String string) {
                    return string;
                }

                public static String toUpperCase(CharSequence chars) {
                    return chars.toString();
                }

                public static char toUpperCase(char c) {
                    return c;
                }
            }
        """.trimIndent()

        val icuAndGuava = mapOf(
            "android/icu/lang/UCharacter.java" to uCharacter("android.icu.lang", "android.icu.util.ULocale"),
            "android/icu/util/ULocale.java" to uLocale("android.icu.util"),
            "com/ibm/icu/lang/UCharacter.java" to uCharacter("com.ibm.icu.lang", "com.ibm.icu.util.ULocale"),
            "com/ibm/icu/util/ULocale.java" to uLocale("com.ibm.icu.util"),
            "com/google/common/base/Ascii.java" to ascii,
        )
    }
}
