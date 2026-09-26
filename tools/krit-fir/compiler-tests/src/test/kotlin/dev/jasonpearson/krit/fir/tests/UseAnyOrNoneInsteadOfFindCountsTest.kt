package dev.jasonpearson.krit.fir.tests

import dev.jasonpearson.krit.fir.FirRuleCompileContext
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals

// UseAnyOrNoneInsteadOfFind finding counts per line, which the golden markers
// cannot express: each find-and-compare is one finding, as in Go.
class UseAnyOrNoneInsteadOfFindCountsTest {

    private fun countsByLine(source: String): Map<Int, Int> =
        KritFirProbe.diagnose(
            mapOf("Main.kt" to source),
            FirRuleCompileContext(enabledRuleIds = setOf("UseAnyOrNoneInsteadOfFind")),
        )
            .filter { it.name == "UseAnyOrNoneInsteadOfFind" }
            .groupingBy { it.line }
            .eachCount()

    @Test
    fun nestedComparisonsOnOneLine() {
        val source = """
            package counts

            fun nested(groups: List<List<Int>>): Boolean = groups.find { g -> g.find { it > 0 } != null } != null
        """.trimIndent()
        assertEquals(mapOf(3 to 2), countsByLine(source))
    }

    @Test
    fun separateComparisonsOnOneLine() {
        val source = """
            package counts

            fun both(list: List<Int>): Boolean = list.find { it > 0 } != null && list.lastOrNull { it < 0 } == null
        """.trimIndent()
        assertEquals(mapOf(3 to 2), countsByLine(source))
    }
}
