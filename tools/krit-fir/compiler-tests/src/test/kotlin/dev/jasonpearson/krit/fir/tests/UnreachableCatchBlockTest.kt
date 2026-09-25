package dev.jasonpearson.krit.fir.tests

import org.junit.jupiter.api.Test
import kotlin.test.assertEquals

// UnreachableCatchBlock finding counts per line, which the golden markers
// cannot express. The Go rule reports once per (earlier clause, later clause)
// pair, so a clause shadowed by several earlier clauses gets several findings
// on its line.
class UnreachableCatchBlockTest {

    private fun countsByLine(source: String): Map<Int, Int> =
        KritFirProbe.diagnose(source)
            .filter { it.name == "UnreachableCatchBlock" }
            .groupingBy { it.line }
            .eachCount()

    @Test
    fun onePerShadowingClause() {
        val source = """
            package counts

            import java.io.IOException

            fun r() {}

            fun f() {
                try { r() } catch (e: Throwable) { } catch (e: Exception) { } catch (e: IOException) { }
            }
        """.trimIndent()
        // Exception: shadowed by Throwable. IOException: by Throwable and Exception.
        assertEquals(mapOf(8 to 3), countsByLine(source))
    }

    // Go emits an exact repeat (same line, column and message) once per pair;
    // krit's FIR merge drops exact repeats, so FIR reports each distinct
    // message once.
    @Test
    fun repeatedIdenticalMessagesCollapse() {
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
        // Line 8: the second clause duplicates the first; the third duplicates
        // both, with one message. Line 15 duplicates line 14. Line 16:
        // IOException is shadowed by both Exception clauses, with one message.
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
}
