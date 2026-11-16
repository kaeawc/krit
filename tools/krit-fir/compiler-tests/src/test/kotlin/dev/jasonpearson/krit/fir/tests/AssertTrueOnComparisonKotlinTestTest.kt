package dev.jasonpearson.krit.fir.tests

import dev.jasonpearson.krit.fir.FirRuleCompileContext
import java.io.File
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

// AssertTrueOnComparison on kotlin.test, compiled against the real kotlin-test jar because a stub cannot declare
// the `kotlin` package. Reported lines are the 1-based lines of the source below.
class AssertTrueOnComparisonKotlinTestTest {

    private fun reportedLines(source: String): List<Int> {
        val jar = requireNotNull(System.getProperty("kotlin.test.jar")) { "kotlin.test.jar is not set" }
        val stdlib = System.getProperty("kotlin.stdlib.jar")
        val result = KritFirProbe.compile(
            mapOf("Main.kt" to source),
            FirRuleCompileContext(enabledRuleIds = setOf("AssertTrueOnComparison")),
            configure = { it.classpath = listOfNotNull(stdlib, jar).joinToString(File.pathSeparator) },
        )
        assertTrue(result.clean, result.problems())
        return result.diags.filter { it.name == "AssertTrueOnComparison" }.map { it.line }
    }

    @Test fun reportsOneArgumentEqualityAndNullComparisons() {
        val source = """
            import kotlin.test.assertTrue

            fun check(actual: Int, value: String?) {
                assertTrue(actual == 42)
                assertTrue(value == null)
                assertTrue(value != null)
            }
        """.trimIndent()
        assertEquals(listOf(4, 5, 6), reportedLines(source))
    }

    @Test fun leavesOtherShapesAlone() {
        val source = """
            import kotlin.test.assertTrue

            fun check(actual: Int, flag: Boolean) {
                assertTrue(actual > 0)
                assertTrue(actual != 42)
                assertTrue(flag == true)
                assertTrue(actual == 42, "message")
                assertTrue { actual == 42 }
                assertTrue("message") { actual == 42 }
            }
        """.trimIndent()
        assertEquals(emptyList(), reportedLines(source))
    }

    @Test fun reportsTheQualifiedAndAliasedCalls() {
        val source = """
            import kotlin.test.assertTrue as check

            fun check(actual: Int) {
                check(actual == 42)
                kotlin.test.assertTrue(actual == 42)
            }
        """.trimIndent()
        assertEquals(listOf(4, 5), reportedLines(source))
    }

    @Test fun ignoresAProjectFunctionNamedAssertTrue() {
        val source = """
            fun assertTrue(condition: Boolean) {}

            fun check(actual: Int) {
                assertTrue(actual == 42)
            }
        """.trimIndent()
        assertEquals(emptyList(), reportedLines(source))
    }
}
