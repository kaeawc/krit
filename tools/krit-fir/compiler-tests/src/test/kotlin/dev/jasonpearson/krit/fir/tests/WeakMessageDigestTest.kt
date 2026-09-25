package dev.jasonpearson.krit.fir.tests

import org.junit.jupiter.api.Test
import kotlin.test.fail

// WeakMessageDigest cases a single-file golden cannot express: MessageDigest
// lookalikes declared in another file, and literals holding raw whitespace
// characters where Kotlin's trim() and Go's strings.TrimSpace disagree (the
// snippets below are built with Kotlin escapes, so the compiled source holds
// the raw characters).
class WeakMessageDigestTest {

    private data class Case(val file: String, val source: String, val expected: Int, val why: String)

    private fun assertCases(support: Map<String, String>, cases: List<Case>) {
        val diags = KritFirProbe.diagnose(support + cases.associate { it.file to it.source })
        val failures = cases.mapNotNull { case ->
            val n = diags.count { it.file == case.file && it.name == "WeakMessageDigest" }
            if (n == case.expected) null else "[${case.file}] expected ${case.expected}, got $n: ${case.why}"
        }
        if (failures.isNotEmpty()) fail(failures.joinToString("\n"))
    }

    private val lookalikeSupport = mapOf(
        "LookalikeOtherPackage.kt" to """
            package com.example.crypto

            class MessageDigest {
                companion object {
                    fun getInstance(algorithm: String): MessageDigest = MessageDigest()
                }
            }
        """.trimIndent(),
        "LookalikeSamePackage.kt" to """
            package samepackage

            class MessageDigest {
                companion object {
                    fun getInstance(algorithm: String): MessageDigest = MessageDigest()
                }
            }
        """.trimIndent(),
    )

    @Test
    fun lookalikesDeclaredInOtherFiles() = assertCases(
        lookalikeSupport,
        listOf(
            Case(
                "ImportsOtherPackage.kt",
                """
                    package importsother

                    import com.example.crypto.MessageDigest

                    fun hash() {
                        MessageDigest.getInstance("MD5")
                    }
                """.trimIndent(),
                expected = 0,
                why = "an imported com.example.crypto.MessageDigest is not the JDK class (Go is silent too)",
            ),
            Case(
                "ImportsOtherPackageOverStar.kt",
                """
                    package importsotheroverstar

                    import java.security.*
                    import com.example.crypto.MessageDigest

                    fun hash() {
                        MessageDigest.getInstance("MD5")
                    }
                """.trimIndent(),
                expected = 0,
                // Go reports this because the star import satisfies its mention
                // check; FIR is correct because the explicit import wins.
                why = "the explicit lookalike import wins over java.security.* (deliberate precision fix)",
            ),
            Case(
                "SamePackageOverStar.kt",
                """
                    package samepackage

                    import java.security.*

                    fun hash() {
                        MessageDigest.getInstance("MD5")
                    }
                """.trimIndent(),
                expected = 0,
                // Go reports this because it only looks for a MessageDigest
                // declared in the same file; FIR is correct because a
                // same-package declaration wins over a star import.
                why = "a same-package MessageDigest in another file wins over java.security.* (deliberate precision fix)",
            ),
            Case(
                "StarImportOnly.kt",
                """
                    package staronly

                    import java.security.*

                    fun hash() {
                        MessageDigest.getInstance("MD5")
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
            "import java.security.MessageDigest\n\n" +
            "fun hash() {\n" +
            "    MessageDigest.getInstance(\"" + literalContent + "\")\n" +
            "}\n",
        expected,
        why,
    )

    @Test
    fun trimMatchesGoTrimSpace() = assertCases(
        emptyMap(),
        listOf(
            trimCase("TrimNel.kt", "MD5\u0085", 1, "Go's TrimSpace trims U+0085; Kotlin's trim() does not"),
            trimCase("TrimNbsp.kt", " SHA-1 ", 1, "Go's TrimSpace trims U+00A0"),
            trimCase("TrimEmSpace.kt", "　MD5 ", 1, "Go's TrimSpace trims Unicode White_Space"),
            trimCase("TrimUnitSeparator.kt", "MD5\u001F", 0, "Go's TrimSpace keeps U+001F; Kotlin's trim() drops it"),
            trimCase("TrimFileSeparator.kt", "\u001CSHA1", 0, "Go's TrimSpace keeps U+001C; Kotlin's trim() drops it"),
        ),
    )
}
