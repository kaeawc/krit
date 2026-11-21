package dev.jasonpearson.krit.fir.tests

import dev.jasonpearson.krit.fir.FirRuleCompileContext
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

class MutableStateInObjectTestFileTest {
    private val sources = mapOf(
        "Listed.kt" to "package listed\nobject Counter { var total = 0 }",
        "src/test/kotlin/Unlisted.kt" to "package unlisted\nobject Counter { var total = 0 }",
    )

    private fun findings(
        testFiles: Set<String> = emptySet(),
        scanPaths: Map<String, String> = emptyMap(),
    ): Map<String, Int> {
        val result = KritFirProbe.compile(
            sources,
            FirRuleCompileContext(enabledRuleIds = setOf("MutableStateInObject")),
            testFiles = testFiles,
            scanPaths = scanPaths,
        )
        assertTrue(result.clean, result.problems())
        return result.diags.filter { it.name == "MutableStateInObject" }.groupingBy { it.file }.eachCount()
    }

    @Test fun requestListedTestFileIsSkipped() {
        assertEquals(mapOf("Unlisted.kt" to 1), findings(testFiles = setOf("Listed.kt")))
    }

    @Test fun unlistedFilesAreCheckedWhateverTheirTestPath() {
        assertEquals(mapOf("Listed.kt" to 1, "Unlisted.kt" to 1), findings())
    }

    @Test fun everyListedFileIsSkipped() {
        assertEquals(emptyMap(), findings(testFiles = sources.keys))
    }

    @Test fun sampleAndPlaygroundExemptionsUseTheScanPath() {
        for (directory in listOf("sample", "samples", "playground", "SAMPLES")) {
            assertEquals(
                mapOf("Unlisted.kt" to 1),
                findings(scanPaths = mapOf("Listed.kt" to "project/$directory/Counter.kt")),
            )
        }
    }

    @Test fun scanRootNamedSamplesDoesNotAddAnExemptionGoLacks() {
        assertEquals(
            mapOf("Listed.kt" to 1, "Unlisted.kt" to 1),
            findings(scanPaths = mapOf("Listed.kt" to "samples/Counter.kt")),
        )
    }

    @Test fun messageAndMultilineAnchorMatchGo() {
        val result = KritFirProbe.compile(
            mapOf("Anchor.kt" to "object Counter {\n    private\n    var total = 0\n    fun bump() { total++ }\n}"),
            FirRuleCompileContext(enabledRuleIds = setOf("MutableStateInObject")),
        )
        assertTrue(result.clean, result.problems())
        val finding = result.diags.single { it.name == "MutableStateInObject" }
        assertEquals(3, finding.line)
        assertTrue(
            finding.message.contains("Mutable 'var total' in object declaration. Shared mutable state without synchronization is a race condition."),
            finding.message,
        )
    }

    @Test fun kotlinTestLifecycleExpansionsAreSilent() {
        for ((before, after) in listOf(
            "org.junit.Before" to "org.junit.After",
            "org.junit.jupiter.api.BeforeEach" to "org.junit.jupiter.api.AfterEach",
            "org.testng.annotations.BeforeMethod" to "org.testng.annotations.AfterMethod",
        )) {
            val result = KritFirProbe.compile(
                mapOf(
                    "Aliases.kt" to "package kotlin.test\ntypealias BeforeTest = $before\ntypealias AfterTest = $after",
                    "State.kt" to """
                        object BeforeState { var count = 0 }
                        object AfterState { var count = 0 }
                        class Owner {
                            @kotlin.test.BeforeTest fun setUp() { repeat(2) { BeforeState.count = 0 } }
                            @kotlin.test.AfterTest fun tearDown() = kotlinx.coroutines.runBlocking { AfterState.count = 0 }
                        }
                    """.trimIndent(),
                ),
                FirRuleCompileContext(enabledRuleIds = setOf("MutableStateInObject")),
                configure = { it.allowKotlinPackage = true },
            )
            assertTrue(result.clean, result.problems())
            assertEquals(emptyList(), result.diags.filter { it.name == "MutableStateInObject" })
        }
    }
}
