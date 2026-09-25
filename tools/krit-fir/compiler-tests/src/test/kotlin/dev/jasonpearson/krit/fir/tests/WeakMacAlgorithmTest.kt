package dev.jasonpearson.krit.fir.tests

import org.junit.jupiter.api.Test
import kotlin.test.fail

// WeakMacAlgorithm cases a single-file golden cannot express: Mac lookalikes
// declared in another file, and literals holding raw whitespace characters
// where Kotlin's trim() and Go's strings.TrimSpace disagree (the snippets below
// are built with Kotlin escapes, so the compiled source holds the raw
// characters).
class WeakMacAlgorithmTest {

    private data class Case(val file: String, val source: String, val expected: Int, val why: String)

    private fun assertCases(support: Map<String, String>, cases: List<Case>) {
        val diags = KritFirProbe.diagnose(support + cases.associate { it.file to it.source })
        val failures = cases.mapNotNull { case ->
            val n = diags.count { it.file == case.file && it.name == "WeakMacAlgorithm" }
            if (n == case.expected) null else "[${case.file}] expected ${case.expected}, got $n: ${case.why}"
        }
        if (failures.isNotEmpty()) fail(failures.joinToString("\n"))
    }

    private val lookalikeSupport = mapOf(
        "MacLookalikeOtherPackage.kt" to """
            package com.example.hmac

            class Mac {
                companion object {
                    fun getInstance(algorithm: String): Mac = Mac()
                }
            }
        """.trimIndent(),
        "MacLookalikeSamePackage.kt" to """
            package macsamepackage

            class Mac {
                companion object {
                    fun getInstance(algorithm: String): Mac = Mac()
                }
            }
        """.trimIndent(),
    )

    @Test
    fun lookalikesDeclaredInOtherFiles() = assertCases(
        lookalikeSupport,
        listOf(
            Case(
                "MacImportsOtherPackage.kt",
                """
                    package macimportsother

                    import com.example.hmac.Mac

                    fun mac() {
                        Mac.getInstance("HmacMD5")
                    }
                """.trimIndent(),
                expected = 0,
                why = "an imported com.example.hmac.Mac is not the JDK class (Go is silent too)",
            ),
            Case(
                "MacImportsOtherPackageOverStar.kt",
                """
                    package macimportsotheroverstar

                    import javax.crypto.*
                    import com.example.hmac.Mac

                    fun mac() {
                        Mac.getInstance("HmacMD5")
                    }
                """.trimIndent(),
                expected = 0,
                // Go reports this because the star import satisfies its mention
                // check; FIR is correct because the explicit import wins.
                why = "the explicit lookalike import wins over javax.crypto.* (deliberate precision fix)",
            ),
            Case(
                "MacSamePackageOverStar.kt",
                """
                    package macsamepackage

                    import javax.crypto.*

                    fun mac() {
                        Mac.getInstance("HmacMD5")
                    }
                """.trimIndent(),
                expected = 0,
                // Go reports this because it only looks for a Mac declared in
                // the same file; FIR is correct because a same-package
                // declaration wins over a star import.
                why = "a same-package Mac in another file wins over javax.crypto.* (deliberate precision fix)",
            ),
            Case(
                "MacStarImportOnly.kt",
                """
                    package macstaronly

                    import javax.crypto.*

                    fun mac() {
                        Mac.getInstance("HmacMD5")
                    }
                """.trimIndent(),
                expected = 1,
                why = "control: with no lookalike the star import resolves to the JDK class",
            ),
        ),
    )

    private fun trimCase(file: String, literalContent: String, expected: Int, why: String) = Case(
        file,
        "package ${file.removeSuffix(".kt").lowercase()}\n\n" +
            "import javax.crypto.Mac\n\n" +
            "fun mac() {\n" +
            "    Mac.getInstance(\"" + literalContent + "\")\n" +
            "}\n",
        expected,
        why,
    )

    @Test
    fun trimMatchesGoTrimSpace() = assertCases(
        emptyMap(),
        listOf(
            trimCase("MacTrimNel.kt", "HmacMD5\u0085", 1, "Go's TrimSpace trims U+0085; Kotlin's trim() does not"),
            trimCase("MacTrimNbsp.kt", " HmacSHA1 ", 1, "Go's TrimSpace trims U+00A0"),
            trimCase("MacTrimEmSpace.kt", " HmacMD5　", 1, "Go's TrimSpace trims Unicode White_Space"),
            trimCase("MacTrimUnitSeparator.kt", "HmacMD5\u001F", 0, "Go's TrimSpace keeps U+001F; Kotlin's trim() drops it"),
            trimCase("MacTrimFileSeparator.kt", "\u001CHmacSHA1", 0, "Go's TrimSpace keeps U+001C; Kotlin's trim() drops it"),
        ),
    )
}
