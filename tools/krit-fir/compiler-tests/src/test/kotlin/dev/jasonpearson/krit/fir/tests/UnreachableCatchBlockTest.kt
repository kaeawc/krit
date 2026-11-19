package dev.jasonpearson.krit.fir.tests

import org.junit.jupiter.api.Test
import kotlin.test.assertEquals

// UnreachableCatchBlock finding counts per line, which the golden markers
// cannot express. A clause shadowed by several earlier clauses is reported
// once, naming the first of them, as detekt and Go report it.
class UnreachableCatchBlockTest {

    private fun countsByLine(source: String): Map<Int, Int> =
        KritFirProbe.diagnose(source)
            .filter { it.name == "UnreachableCatchBlock" }
            .groupingBy { it.line }
            .eachCount()

    private fun messages(source: String): List<String> =
        KritFirProbe.diagnose(source).filter { it.name == "UnreachableCatchBlock" }.map { it.message.removePrefix("[UnreachableCatchBlock] ") }

    @Test
    fun onePerShadowedClause() {
        val source = """
            package counts

            import java.io.IOException

            fun r() {}

            fun f() {
                try { r() } catch (e: Throwable) { } catch (e: Exception) { } catch (e: IOException) { }
            }
        """.trimIndent()
        // Exception: shadowed by Throwable. IOException: by Throwable first, then Exception.
        assertEquals(mapOf(8 to 2), countsByLine(source))
        assertEquals(
            listOf(
                "Catch block for 'Exception' is unreachable because 'Throwable' is caught above.",
                "Catch block for 'IOException' is unreachable because 'Throwable' is caught above.",
            ),
            messages(source).sorted(),
        )
    }

    // A clause that repeats several earlier ones is still one finding.
    @Test
    fun repeatedClausesReportOnce() {
        val source = """
            package repeats

            import java.io.IOException

            fun r() {}

            fun duplicates() {
                try { r() } catch (e: Exception) { } catch (e: Exception) { } catch (e: Exception) { }
            }

            fun sameParentTwice() {
                try {
                    r()
                } catch (e: Exception) {
                } catch (e: Exception) {
                } catch (e: IOException) {
                }
            }
        """.trimIndent()
        // Line 8: the second clause duplicates the first and the third
        // duplicates both, one finding each. Line 15 duplicates line 14.
        // Line 16: IOException is shadowed by both Exception clauses.
        assertEquals(mapOf(8 to 2, 15 to 1, 16 to 1), countsByLine(source))
    }

    // A reified type parameter in a catch clause compiles from language version
    // 2.4. Go compares the written text, so `catch (e: T)` twice is a
    // duplicate; FIR must not drop it for not being a class type.
    @Test
    fun reifiedTypeParameterDuplicate() {
        val source = """
            package reified

            fun r() {}

            inline fun <reified T : Throwable> f() {
                try { r() } catch (e: T) { } catch (e: T) { }
            }

            inline fun <reified T : Exception> g() {
                try { r() } catch (e: Exception) { } catch (e: T) { }
            }
        """.trimIndent()
        val result = KritFirProbe.compile(mapOf("Main.kt" to source)) { it.languageVersion = "2.4" }
        check(result.clean) { "reified catch did not compile:\n" + result.problems() }
        val counts = result.diags
            .filter { it.name == "UnreachableCatchBlock" }
            .groupingBy { it.line }
            .eachCount()
        // Line 6: T duplicates T. Line 10: T is bounded by Exception, so the
        // Exception clause above catches everything it could.
        assertEquals(mapOf(6 to 1, 10 to 1), counts)
    }

    @Test
    fun annotatedCatchType() {
        val source = """
            package annotated

            import java.io.IOException

            @Target(AnnotationTarget.TYPE)
            annotation class Marker

            fun r() {}

            fun f() {
                try { r() } catch (e: Exception) { } catch (e: @Marker IOException) { }
            }
        """.trimIndent()
        assertEquals(mapOf(11 to 1), countsByLine(source))
    }

    // kotlin.coroutines.cancellation.CancellationException is a typealias of
    // java.util.concurrent.CancellationException, which extends
    // IllegalStateException. The alias must expand to that class: the later
    // CancellationException clause is shadowed, the reverse order is not, and
    // an unrelated supertype does not shadow it.
    @Test
    fun cancellationExceptionAliasIsAnIllegalStateException() {
        val source = """
            package aliases

            import java.io.IOException
            import java.net.SocketTimeoutException
            import java.time.DateTimeException
            import java.time.zone.ZoneRulesException
            import kotlin.coroutines.cancellation.CancellationException

            fun r() {}

            fun shadowed() {
                try { r() } catch (e: IllegalStateException) { } catch (e: CancellationException) { }
            }

            fun reached() {
                try { r() } catch (e: CancellationException) { } catch (e: IllegalStateException) { }
            }

            fun unrelated() {
                try { r() } catch (e: IllegalArgumentException) { } catch (e: CancellationException) { }
            }

            fun io() {
                try { r() } catch (e: IOException) { } catch (e: SocketTimeoutException) { }
            }

            fun time() {
                try { r() } catch (e: DateTimeException) { } catch (e: ZoneRulesException) { }
            }
        """.trimIndent()
        assertEquals(mapOf(12 to 1, 24 to 1, 28 to 1), countsByLine(source))
    }
}
