package dev.jasonpearson.krit.fir.tests

import dev.jasonpearson.krit.fir.FirRuleCompileContext
import java.io.File
import org.junit.jupiter.api.Tag
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

class RunBlockingInTestFilesTest {
    private val source = """
        import kotlinx.coroutines.runBlocking
        import org.junit.Test
        class BlockingPathTest {
            @Test fun blocking() = runBlocking { assert(true) }
        }
    """.trimIndent()

    private fun compile(path: String, markers: Any? = emptyList<String>()) = KritFirProbe.compile(
        mapOf("BlockingPathTest.kt" to source),
        FirRuleCompileContext(
            enabledRuleIds = setOf("RunBlockingInTest"),
            ruleConfigs = mapOf("RunBlockingInTest" to mapOf("realDispatcherMarkers" to markers)),
        ),
        scanPaths = mapOf("BlockingPathTest.kt" to path),
    )

    @Test fun androidInstrumentedScanPathsAreExempt() {
        for (directory in listOf("androidTest", "androidInstrumentedTest", "androidCommonTest", "commonJvmAndroidTest")) {
            val result = compile("module/src/$directory/kotlin/BlockingPathTest.kt")
            assertTrue(result.clean, result.problems())
            assertEquals(emptyList(), result.diags)
        }
    }

    @Test fun ordinaryTestScanPathReportsAndEmptyMarkerDoesNotExempt() {
        val result = compile("module/src/test/kotlin/BlockingPathTest.kt", listOf(""))
        assertTrue(result.clean, result.problems())
        assertEquals(1, result.diags.size)
        assertEquals("[RunBlockingInTest] Use `runTest` instead of `runBlocking` in test functions.",
            result.diags.single().message)
    }

    @Test fun configuredOwnerMarkerIsExempt() {
        val result = compile("module/src/test/kotlin/BlockingPathTest.kt", listOf("BlockingPathTest"))
        assertTrue(result.clean, result.problems())
        assertEquals(emptyList(), result.diags)
    }

    @Test fun configuredMarkerAcceptsGoScalarAndValueWithReasonShapes() {
        val configurations = listOf(
            "BlockingPathTest",
            listOf(mapOf("value" to "BlockingPathTest", "reason" to "real dispatcher test")),
        )
        for (markers in configurations) {
            val result = compile("module/src/test/kotlin/BlockingPathTest.kt", markers)
            assertTrue(result.clean, result.problems())
            assertEquals(emptyList(), result.diags)
        }
    }

    @Test fun nestedRunnerUsesGoMessage() {
        val result = KritFirProbe.compile(
            mapOf("BlockingNestedTest.kt" to """
                import kotlinx.coroutines.runBlocking
                import kotlinx.coroutines.test.runTest
                import org.junit.Test
                class BlockingNestedTest {
                    @Test fun nested() = runTest { runBlocking { assert(true) } }
                }
            """.trimIndent()),
            FirRuleCompileContext(enabledRuleIds = setOf("RunBlockingInTest")),
        )
        assertTrue(result.clean, result.problems())
        assertEquals(1, result.diags.size)
        assertEquals("[RunBlockingInTest] Remove the nested `runBlocking`; the test already runs its body in `runTest`.",
            result.diags.single().message)
    }

    @Test fun kotlinTestJvmAnnotationResolves() {
        val result = KritFirProbe.compile(
            mapOf("BlockingKotlinTest.kt" to """
                import kotlinx.coroutines.runBlocking
                class BlockingKotlinTest {
                    @kotlin.test.Test fun blocking() = runBlocking { assert(true) }
                }
            """.trimIndent()),
            FirRuleCompileContext(enabledRuleIds = setOf("RunBlockingInTest")),
        ) { arguments ->
            arguments.classpath = System.getProperty("java.class.path")
        }
        assertTrue(result.clean, result.problems())
        assertEquals(listOf("RunBlockingInTest"), result.diags.map { it.name })
    }

    // Reads the Go fixtures, so it runs in `fixtureParityTest` (see FixtureParityTest).
    @Tag(FixtureParityTest.GO_FIXTURES_TAG)
    @Test fun goFixturesKeepTheirAnchors() {
        for (kind in listOf("positive", "negative")) {
            val fixture = File("../../../tests/fixtures/$kind/testing-quality/RunBlockingInTest.kt")
            val text = fixture.readText()
            val result = KritFirProbe.compile(
                mapOf(fixture.name to text),
                FirRuleCompileContext(enabledRuleIds = setOf("RunBlockingInTest")),
            )
            assertTrue(result.clean, result.problems())
            val lines = if (kind == "positive") listOf(text.lines().indexOfFirst { "= runBlocking" in it } + 1)
                else emptyList()
            assertEquals(lines, result.diags.map { it.line })
        }
    }
}
