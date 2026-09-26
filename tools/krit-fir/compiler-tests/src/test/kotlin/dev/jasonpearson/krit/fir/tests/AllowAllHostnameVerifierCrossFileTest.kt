package dev.jasonpearson.krit.fir.tests

import dev.jasonpearson.krit.fir.FirRuleCompileContext
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

// AllowAllHostnameVerifier cases a single-file golden cannot express: a
// HostnameVerifier lookalike declared in another file and package.
class AllowAllHostnameVerifierCrossFileTest {

    private fun findings(sources: Map<String, String>): List<Pair<String, Int>> {
        val result = KritFirProbe.compile(
            sources,
            FirRuleCompileContext(enabledRuleIds = setOf("AllowAllHostnameVerifier")),
        )
        assertTrue(result.clean, result.problems())
        return result.diags.filter { it.name == "AllowAllHostnameVerifier" }.map { it.file to it.line }
    }

    // Go reports LenientVerifier (line 8): the comment mentions the javax FQN,
    // which satisfies its file gate, the header names HostnameVerifier, and
    // its shadow check only looks for a HostnameVerifier declared in the same
    // file. FIR is correct to drop it: the class implements
    // other.HostnameVerifier, so its verify is not a TLS hostname check (the
    // same reasoning as Go's own same-file exemption, pinned in
    // AllowAllHostnameVerifierLookalike.kt). The JDK verifier in the same
    // file is still reported.
    @Test fun importedLookalikeFromAnotherPackageIsNotAVerifier() {
        val sources = mapOf(
            "Lookalike.kt" to """
                package other

                import javax.net.ssl.SSLSession

                interface HostnameVerifier {
                    fun verify(hostname: String, session: SSLSession): Boolean
                }
            """.trimIndent(),
            "CrossFileLookalike.kt" to """
                package demo

                import javax.net.ssl.SSLSession
                import other.HostnameVerifier

                // Not a javax.net.ssl.HostnameVerifier.
                class LenientVerifier : HostnameVerifier {
                    override fun verify(hostname: String, session: SSLSession): Boolean = true
                }

                class JdkVerifier : javax.net.ssl.HostnameVerifier {
                    override fun verify(hostname: String, session: SSLSession): Boolean = true
                }
            """.trimIndent(),
        )
        assertEquals(listOf("CrossFileLookalike.kt" to 12), findings(sources))
    }
}
