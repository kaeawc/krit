package dev.jasonpearson.krit.fir.tests

import dev.jasonpearson.krit.fir.FirRuleCompileContext
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals

// UseAnyOrNoneInsteadOfFind reports only when the receiver has the suggested
// any / none: these cases need the counterpart declared in another file or
// package, or missing, which a single-file golden cannot express.
class UseAnyOrNoneInsteadOfFindCounterpartTest {

    private fun findingLines(sources: Map<String, String>): List<Pair<String, Int>> =
        KritFirProbe.diagnose(
            sources,
            FirRuleCompileContext(enabledRuleIds = setOf("UseAnyOrNoneInsteadOfFind")),
        )
            .filter { it.name == "UseAnyOrNoneInsteadOfFind" }
            .map { it.file to it.line }

    private val usage = """
        package app

        import kotlinx.coroutines.flow.Flow
        import kotlinx.coroutines.flow.firstOrNull

        suspend fun notNull(flow: Flow<Int>): Boolean = flow.firstOrNull { it > 0 } != null

        suspend fun isNull(flow: Flow<Int>): Boolean = flow.firstOrNull { it > 0 } == null
    """.trimIndent()

    @Test
    fun flowWithAnyAndNoneFromAnotherPackage() {
        val operators = """
            package kotlinx.coroutines.flow

            suspend fun <T> Flow<T>.firstOrNull(predicate: suspend (T) -> Boolean): T? = TODO()
            suspend fun <T> Flow<T>.any(predicate: suspend (T) -> Boolean): Boolean = TODO()
            suspend fun <T> Flow<T>.none(predicate: suspend (T) -> Boolean): Boolean = TODO()
        """.trimIndent()
        assertEquals(
            listOf("Main.kt" to 6, "Main.kt" to 8),
            findingLines(mapOf("Operators.kt" to operators, "Main.kt" to usage)).sortedBy { it.second },
        )
    }

    // Coroutines before 1.10 has firstOrNull(predicate) but no Flow.any / none:
    // there is nothing to use instead, so the checker stays silent.
    @Test
    fun flowWithoutAnyOrNone() {
        val operators = """
            package kotlinx.coroutines.flow

            suspend fun <T> Flow<T>.firstOrNull(predicate: suspend (T) -> Boolean): T? = TODO()
        """.trimIndent()
        assertEquals(emptyList(), findingLines(mapOf("Operators.kt" to operators, "Main.kt" to usage)))
    }

    // An extension any declared next to a class's own member find.
    @Test
    fun extensionAnyInTheFindPackage() {
        val library = """
            package lib

            class Catalog {
                fun find(predicate: (String) -> Boolean): String? = null
            }

            fun Catalog.any(predicate: (String) -> Boolean): Boolean = find(predicate) != null
        """.trimIndent()
        val main = """
            package app

            import lib.Catalog

            fun notNull(catalog: Catalog): Boolean = catalog.find { it.isEmpty() } != null

            fun isNull(catalog: Catalog): Boolean = catalog.find { it.isEmpty() } == null
        """.trimIndent()
        // Only `!= null`: there is no none. (Library.kt passes a function
        // value, not a lambda, so its own comparison is not reported.)
        assertEquals(
            listOf("Main.kt" to 5),
            findingLines(mapOf("Library.kt" to library, "Main.kt" to main)),
        )
    }
}
