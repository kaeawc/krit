package dev.jasonpearson.krit.fir.tests

import org.junit.jupiter.api.Test
import kotlin.test.assertFalse
import kotlin.test.fail

// StaticIv cases a single-file golden cannot express: the Android
// android.util.Base64 decoder (not part of the stub library, so it is declared
// here as a Java source), and spec or Base64 lookalikes declared in another
// file of the same package.
class StaticIvTest {

    private data class Case(val file: String, val source: String, val expected: Int, val why: String)

    private fun assertCases(support: Map<String, String>, cases: List<Case>) {
        val diags = KritFirProbe.diagnose(support + cases.associate { it.file to it.source })
        val failures = cases.mapNotNull { case ->
            val n = diags.count { it.file == case.file && it.name == "StaticIv" }
            if (n == case.expected) null else "[${case.file}] expected ${case.expected}, got $n: ${case.why}"
        }
        if (failures.isNotEmpty()) fail(failures.joinToString("\n"))
    }

    // The real android.util.Base64 is a final class of static methods.
    private val androidBase64 = mapOf(
        "android/util/Base64.java" to """
            package android.util;

            public class Base64 {
                public static final int DEFAULT = 0;
                public static byte[] decode(String str, int flags) { return null; }
                public static byte[] decode(byte[] input, int flags) { return null; }
                public static byte[] decode(byte[] input, int offset, int len, int flags) { return null; }
            }
        """.trimIndent(),
    )

    @Test
    fun androidBase64Decode() = assertCases(
        androidBase64,
        listOf(
            Case(
                "AndroidDecode.kt",
                """
                    package androiddecode

                    import android.util.Base64
                    import javax.crypto.spec.GCMParameterSpec
                    import javax.crypto.spec.IvParameterSpec

                    class Crypto(private val stored: String) {
                        fun literal() = IvParameterSpec(Base64.decode("AAAAAAAAAAAAAAAAAAAAAA==", Base64.DEFAULT))
                        fun literalBytes() = GCMParameterSpec(128, Base64.decode("AAAAAAAAAAAAAAAA".toByteArray(), Base64.DEFAULT))
                        fun qualified() = IvParameterSpec(android.util.Base64.decode("AAAA", 0).copyOf(16))
                    }
                """.trimIndent(),
                expected = 3,
                why = "Go reports each: the text holds `Base64.decode(` and a quote",
            ),
            Case(
                "AndroidDecodeNegative.kt",
                """
                    package androiddecodenegative

                    import android.util.Base64
                    import javax.crypto.spec.IvParameterSpec

                    const val IV_TEXT = "AAAA"

                    class Crypto(private val stored: String, private val keys: Map<String, String>) {
                        fun stored() = IvParameterSpec(Base64.decode(stored, Base64.DEFAULT))
                        fun constant() = IvParameterSpec(Base64.decode(IV_TEXT, Base64.DEFAULT))
                        // Go reports this: the quote belongs to the map key.
                        fun keyed() = IvParameterSpec(Base64.decode(keys.getValue("iv"), Base64.DEFAULT))
                    }
                """.trimIndent(),
                expected = 0,
                why = "the decoded data is not a literal (keyed: deliberate precision fix)",
            ),
        ),
    )

    private val samePackageLookalikes = mapOf(
        "Lookalikes.kt" to """
            package samepackage

            class IvParameterSpec(val bytes: ByteArray)

            object Base64 {
                fun decode(text: String, flags: Int): ByteArray = ByteArray(16)
            }
        """.trimIndent(),
    )

    @Test
    fun lookalikesDeclaredInOtherFiles() = assertCases(
        samePackageLookalikes + androidBase64,
        listOf(
            Case(
                "SamePackageSpec.kt",
                """
                    package samepackage

                    import javax.crypto.spec.*

                    fun spec() = IvParameterSpec(byteArrayOf(1, 2, 3))
                """.trimIndent(),
                expected = 0,
                // Go reports this because the star import satisfies its import
                // check and the file itself declares no IvParameterSpec; FIR is
                // correct because a same-package class wins over a star import.
                why = "a same-package IvParameterSpec wins over javax.crypto.spec.* (deliberate precision fix)",
            ),
            Case(
                "SamePackageBase64.kt",
                """
                    package samepackage

                    import android.util.*
                    import javax.crypto.spec.GCMParameterSpec

                    fun gcmSpec() = GCMParameterSpec(128, Base64.decode("AAAAAAAAAAAAAAAA", 0))
                """.trimIndent(),
                expected = 0,
                // Go reports this because it matches the text `Base64.decode(`;
                // FIR is correct because the same-package Base64 wins over
                // android.util.* and decodes nothing from the literal.
                why = "a same-package Base64 wins over android.util.* (deliberate precision fix)",
            ),
            Case(
                "StarImportOnly.kt",
                """
                    package staronly

                    import javax.crypto.spec.*

                    fun spec() = IvParameterSpec(byteArrayOf(1, 2, 3))
                """.trimIndent(),
                expected = 1,
                why = "control: with no lookalike the star import resolves to the JDK class",
            ),
        ),
    )

    // Go also accepts `"...".getBytes(` and a `"...".bytes` suffix, the Java
    // spellings. Kotlin's String hides java.lang.String.getBytes, so neither
    // compiles in Kotlin and FIR needs no counterpart.
    @Test
    fun javaGetBytesDoesNotCompileInKotlin() {
        for ((file, expression) in listOf("GetBytes.kt" to "\"abc\".getBytes()", "Bytes.kt" to "\"abc\".bytes")) {
            val compilation = KritFirProbe.compile(
                mapOf(file to "package getbytes\n\nfun iv(): ByteArray = $expression\n"),
            )
            assertFalse(compilation.clean, "$expression unexpectedly compiles; StaticIv must handle it")
        }
    }
}
