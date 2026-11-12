package dev.jasonpearson.krit.fir.tests

import dev.jasonpearson.krit.fir.FirRuleCompileContext
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

// AssertNullableWithNotNullAssertion cases that need request facts (which files are test support) or the message.
class AssertNullableWithNotNullAssertionFilesTest {

    private fun junitSource(pkg: String) = """
        package $pkg

        import org.junit.Assert.assertEquals

        fun check(maybe: String?) {
            assertEquals("x", maybe!!)
        }
    """.trimIndent()

    private fun diagnostics(
        sources: Map<String, String>,
        testFiles: Set<String> = emptySet(),
    ): List<KritFirProbe.Diag> {
        val result = KritFirProbe.compile(
            sources,
            FirRuleCompileContext(enabledRuleIds = setOf("AssertNullableWithNotNullAssertion")),
            testFiles = testFiles,
        )
        assertTrue(result.clean, result.problems())
        return result.diags.filter { it.name == "AssertNullableWithNotNullAssertion" }
    }

    private fun filesWithFindings(sources: Map<String, String>, testFiles: Set<String> = emptySet()) =
        diagnostics(sources, testFiles).map { it.file }.toSet()

    @Test fun runsOnlyOnTestSupportFiles() {
        val sources = mapOf(
            "Plain.kt" to junitSource("plain"),
            "Listed.kt" to junitSource("listed"),
            "ExampleTest.kt" to junitSource("example"),
            "feature-testing/src/main/kotlin/Support.kt" to junitSource("support"),
            "app/testing/Helper.kt" to junitSource("helper"),
            "lib/test-utils/Utils.kt" to junitSource("utils"),
            "contest/src/main/kotlin/Contests.kt" to junitSource("contest"),
        )
        assertEquals(
            setOf("Listed.kt", "ExampleTest.kt", "Support.kt", "Helper.kt", "Utils.kt"),
            filesWithFindings(sources, testFiles = setOf("Listed.kt")).map { it.substringAfterLast('/') }.toSet(),
        )
    }

    @Test fun messageNamesTheOperandAndTheAssertionLibrary() {
        val truth = """
            package truth

            import com.google.common.truth.Truth.assertThat

            fun check(maybe: String?) {
                assertThat(maybe!!).isEqualTo("x")
            }
        """.trimIndent()
        val diags = diagnostics(mapOf("TruthTest.kt" to truth, "JunitTest.kt" to junitSource("junit")))
        val messages = diags.associate { it.file to it.message }
        assertEquals(
            "Avoid `!!` in assertions; use `assertThat(maybe).isNotNull()` first, then use a local val or smart-cast.",
            messages.getValue("TruthTest.kt").removePrefix("[AssertNullableWithNotNullAssertion] "),
        )
        assertEquals(
            "Avoid `!!` in assertions; use `assertNotNull(maybe)` first and smart-cast.",
            messages.getValue("JunitTest.kt").removePrefix("[AssertNullableWithNotNullAssertion] "),
        )
    }

    @Test fun memberReceiversAreAssertionArguments() {
        val source = """
            package members

            import org.junit.Assert.assertEquals

            class Box(val items: List<String>)

            fun check(first: Box?, second: Box?, third: Box?) {
                assertEquals(1, first!!.items.size)
                assertEquals(second!!.items.size, third!!.items.size)
                assertEquals(1, second!!.items.size)
            }
        """.trimIndent()
        val messages = diagnostics(mapOf("BoxTest.kt" to source)).map { it.message.substringAfter("] ") }
        assertEquals(
            listOf(
                "Avoid `!!` in assertion arguments; use `assertNotNull(first)` first and smart-cast.",
                "Avoid `!!` in assertion arguments; use `assertNotNull(second)` and `assertNotNull(third)` first and smart-cast.",
            ),
            messages,
        )
    }

    @Test fun nonReceiverOperandsAreAssertions() {
        val source = """
            package operands

            import org.junit.Assert.assertEquals

            class Box(val items: List<String>?)

            fun check(box: Box, other: Box) {
                assertEquals("x", box.items!!)
                assertEquals("x", other.items!!.first())
            }
        """.trimIndent()
        val messages = diagnostics(mapOf("OperandTest.kt" to source)).map { it.message.substringAfter("] ") }
        assertEquals(
            listOf(
                "Avoid `!!` in assertions; use `assertNotNull(box.items)` first and smart-cast.",
                "Avoid `!!` in assertion arguments; use `assertNotNull(other.items)` first and smart-cast.",
            ),
            messages,
        )
    }

    @Test fun longOperandsAreCutToSixtyCharacters() {
        val source = """
            package long

            import org.junit.Assert.assertEquals

            class Box(val aVeryLongPropertyNameThatKeepsGoingAndGoingAndGoingAndGoingAndGoing: String?)

            fun check(box: Box) {
                assertEquals("x", box.aVeryLongPropertyNameThatKeepsGoingAndGoingAndGoingAndGoingAndGoing!!)
            }
        """.trimIndent()
        val message = diagnostics(mapOf("LongTest.kt" to source)).single().message.substringAfter("] ")
        assertEquals(
            "Avoid `!!` in assertions; use `assertNotNull(box.aVeryLongPropertyNameThatKeepsGoingAndGoingAndGoingAn...)` first and smart-cast.",
            message,
        )
    }
}
