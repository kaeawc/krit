package dev.jasonpearson.krit.fir.tests

import dev.jasonpearson.krit.fir.FirRuleCompileContext
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals

// MapGetWithNotNullAssertionOperator finding counts per line, which the
// golden markers cannot express: each asserted lookup is one finding.
class MapGetWithNotNullAssertionOperatorCountsTest {

    private fun countsByLine(source: String): Map<Int, Int> =
        KritFirProbe.diagnose(
            mapOf("Main.kt" to source),
            FirRuleCompileContext(enabledRuleIds = setOf("MapGetWithNotNullAssertionOperator")),
        )
            .filter { it.name == "MapGetWithNotNullAssertionOperator" }
            .groupingBy { it.line }
            .eachCount()

    // Go reports only the inner lookup (it cannot type the outer receiver,
    // the `!!` expression); both are Map.get calls asserted with `!!`.
    @Test
    fun nestedLookupsOnOneLine() {
        val source = """
            package counts

            fun nested(tables: Map<String, Map<String, Int>>): Int = tables["a"]!!["b"]!!
        """.trimIndent()
        assertEquals(mapOf(3 to 2), countsByLine(source))
    }

    @Test
    fun separateLookupsOnOneLine() {
        val source = """
            package counts

            fun sum(map: Map<String, Int>): Int = map["a"]!! + map.get("b")!!
        """.trimIndent()
        assertEquals(mapOf(3 to 2), countsByLine(source))
    }
}
