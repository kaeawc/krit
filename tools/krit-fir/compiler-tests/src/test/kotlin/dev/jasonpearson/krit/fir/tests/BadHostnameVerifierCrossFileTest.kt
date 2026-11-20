package dev.jasonpearson.krit.fir.tests

import dev.jasonpearson.krit.fir.FirRuleCompileContext
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

// BadHostnameVerifier cases a single-file golden cannot express: a
// HostnameVerifier lookalike declared in another file and package.
class BadHostnameVerifierCrossFileTest {

    private fun findings(sources: Map<String, String>): List<Pair<String, Int>> {
        val result = KritFirProbe.compile(
            sources,
            FirRuleCompileContext(enabledRuleIds = setOf("BadHostnameVerifier")),
        )
        assertTrue(result.clean, result.problems())
        return result.diags.filter { it.name == "BadHostnameVerifier" }.map { it.file to it.line }
    }

    // An explicit lookalike import is silent; the JDK FQN remains reportable.
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
