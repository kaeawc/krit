package dev.jasonpearson.krit.fir.tests

import dev.jasonpearson.krit.fir.FirRuleCompileContext
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

// ComposeRawTextLiteral and ComposeSemanticsMissingRole skip test files; the text rule also skips debug, playground,
// sample and benchmark sources and the role rule test-support directories, matched on the scan path as Go does.
class ComposeRawTextLiteralFilesTest {

    private val source = """
        package rawtext

        import androidx.compose.foundation.clickable
        import androidx.compose.material3.Text
        import androidx.compose.runtime.Composable
        import androidx.compose.ui.Modifier

        @Composable
        fun Screen(onClick: () -> Unit) {
            Text("Welcome", Modifier.clickable { onClick() })
        }
    """.trimIndent()

    private fun reported(
        testFiles: Set<String> = emptySet(),
        scanPaths: Map<String, String> = emptyMap(),
        configs: Map<String, Map<String, Any?>> = emptyMap(),
    ): Map<String, Int> {
        val result = KritFirProbe.compile(
            mapOf("F.kt" to source),
            FirRuleCompileContext(enabledRuleIds = setOf("ComposeRawTextLiteral", "ComposeSemanticsMissingRole"), ruleConfigs = configs),
            testFiles = testFiles,
            scanPaths = scanPaths,
        )
        assertTrue(result.clean, result.problems())
        return result.diags.groupingBy { it.name }.eachCount()
    }

    private val both = mapOf("ComposeRawTextLiteral" to 1, "ComposeSemanticsMissingRole" to 1)

    @Test fun productionFileIsReported() = assertEquals(both, reported())

    @Test fun testFilesAreSkipped() = assertEquals(emptyMap(), reported(testFiles = setOf("F.kt")))

    @Test fun nonProductionDirectoriesSkipTheTextRule() {
        for (dir in listOf("debug", "playground", "sandbox", "samples", "sample", "demos", "demo", "benchmarks", "ui-tooling")) {
            assertEquals(mapOf("ComposeSemanticsMissingRole" to 1), reported(scanPaths = mapOf("F.kt" to "app/$dir/F.kt")), dir)
        }
    }

    // Go skips test-support directories only for the role rule; the text rule skips just the markers above.
    @Test fun testSupportDirectoriesSkipTheRoleRule() {
        for (dir in listOf("fakes", "mocks", "testFixtures")) {
            assertEquals(mapOf("ComposeRawTextLiteral" to 1), reported(scanPaths = mapOf("F.kt" to "app/$dir/F.kt")), dir)
        }
        assertEquals(emptyMap(), reported(scanPaths = mapOf("F.kt" to "app/test-utils/F.kt")))
    }

    @Test fun configuredMarkersSkipTheTextRule() {
        val configs = mapOf("ComposeRawTextLiteral" to mapOf<String, Any?>("nonProductionPathMarkers" to listOf("Internal-Tools")))
        assertEquals(
            mapOf("ComposeSemanticsMissingRole" to 1),
            reported(scanPaths = mapOf("F.kt" to "app/internal-tools/F.kt"), configs = configs),
        )
    }

    // The markers are matched on the scan spelling; a checkout under such a directory does not hide the file.
    @Test fun scanPathWithoutMarkerIsReported() =
        assertEquals(both, reported(scanPaths = mapOf("F.kt" to "app/src/main/kotlin/F.kt")))
}
