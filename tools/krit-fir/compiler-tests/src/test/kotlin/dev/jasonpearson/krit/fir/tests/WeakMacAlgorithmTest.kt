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
        "MacLookalikeNestedInBase.kt" to """
            package com.example.base

            open class Base2 {
                class Mac {
                    companion object {
                        fun getInstance(algorithm: String): Mac = Mac()
                    }
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
            // In the next three cases Go's mention check is satisfied by
            // something other than an import of javax.crypto.Mac, so Go
            // reports; FIR is correct because the explicit import of the
            // lookalike decides what the bare `Mac` resolves to.
            Case(
                "MacImportsOtherPackageCommentMention.kt",
                """
                    package macimportsothercomment

                    import com.example.hmac.Mac

                    /** Not javax.crypto.Mac: this wraps the in-house HMAC factory. */
                    fun mac() {
                        // Unlike javax.crypto.Mac, this Mac is ours.
                        Mac.getInstance("HmacMD5")
                    }
                """.trimIndent(),
                expected = 0,
                // Go's byte search for "javax.crypto.Mac" finds the comment
                // and KDoc text.
                why = "a comment or KDoc naming javax.crypto.Mac does not change the imported lookalike (deliberate precision fix)",
            ),
            Case(
                "MacImportsOtherPackageMacSpi.kt",
                """
                    package macimportsothermacspi

                    import javax.crypto.MacSpi
                    import com.example.hmac.Mac

                    fun spi(spi: MacSpi): MacSpi = spi

                    fun mac() {
                        Mac.getInstance("HmacMD5")
                    }
                """.trimIndent(),
                expected = 0,
                // Go's byte search for "javax.crypto.Mac" matches inside
                // "javax.crypto.MacSpi".
                why = "importing javax.crypto.MacSpi does not make the imported lookalike the JDK Mac (deliberate precision fix)",
            ),
            Case(
                "MacImportsOtherPackageAliasedJdk.kt",
                """
                    package macimportsotheraliasedjdk

                    import javax.crypto.Mac as JMac
                    import com.example.hmac.Mac

                    fun jdk(mac: JMac): JMac = mac

                    fun mac() {
                        Mac.getInstance("HmacMD5")
                    }
                """.trimIndent(),
                expected = 0,
                // Go's byte search for "javax.crypto.Mac" matches the aliased
                // import, which binds JMac, not Mac.
                why = "javax.crypto.Mac imported as JMac leaves the bare Mac bound to the lookalike (deliberate precision fix)",
            ),
            Case(
                "MacInheritedNestedClass.kt",
                """
                    package macinheritednested

                    import com.example.base.Base2
                    import javax.crypto.Mac

                    class Sub : Base2() {
                        fun mac() {
                            Mac.getInstance("HmacMD5")
                        }
                    }
                """.trimIndent(),
                expected = 0,
                // Go reports this because the file imports javax.crypto.Mac and
                // declares no Mac; FIR is correct because the nested Mac
                // inherited from Base2 shadows the import inside Sub.
                why = "a nested Mac inherited from a supertype in another file shadows the import (deliberate precision fix)",
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
