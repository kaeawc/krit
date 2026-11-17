package dev.jasonpearson.krit.fir.tests

import dev.jasonpearson.krit.fir.FirRuleCompileContext
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals

// A finding on a chain whose `.toString()` sits on a later line is reported on the line the chain starts on, as Go does.
class NullableToStringCallLineTest {

    private val source = """
        package chain

        fun chained(value: String?, other: String?): String {
            val first = value
                .toString()
            val second = other
                ?.trim()
                .toString()
            return first + second
        }
    """.trimIndent()

    @Test fun chainIsReportedOnItsFirstLine() {
        val result = KritFirProbe.compile(
            mapOf("Chain.kt" to source),
            FirRuleCompileContext(enabledRuleIds = setOf("NullableToStringCall")),
        )
        assertEquals(emptyList(), result.compileErrors, result.problems())
        val lines = result.diags.filter { it.name == "NullableToStringCall" }.map { it.line }
        assertEquals(listOf(4, 6), lines, result.diags.toString() + result.problems())
    }
}
