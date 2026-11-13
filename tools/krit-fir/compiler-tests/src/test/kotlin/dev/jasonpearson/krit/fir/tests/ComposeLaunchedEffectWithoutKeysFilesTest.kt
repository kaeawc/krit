package dev.jasonpearson.krit.fir.tests

import dev.jasonpearson.krit.fir.FirRuleCompileContext
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

// ComposeLaunchedEffectWithoutKeys and ComposeRememberWithoutKey skip test files and test-support
// directories, matched on the scan path, as Go does.
class ComposeLaunchedEffectWithoutKeysFilesTest {

    private val source = """
        package effectfiles

        import androidx.compose.runtime.Composable
        import androidx.compose.runtime.LaunchedEffect
        import androidx.compose.runtime.remember

        fun fetch(value: Any?) {}

        @Composable
        fun Screen(userId: String) {
            LaunchedEffect(Unit) { fetch(userId) }
            val cached = remember { userId.length }
            fetch(cached)
        }
    """.trimIndent()

    private fun reported(
        testFiles: Set<String> = emptySet(),
        scanPaths: Map<String, String> = emptyMap(),
    ): Map<String, Int> {
        val result = KritFirProbe.compile(
            mapOf("F.kt" to source),
            FirRuleCompileContext(enabledRuleIds = setOf("ComposeLaunchedEffectWithoutKeys", "ComposeRememberWithoutKey")),
            testFiles = testFiles,
            scanPaths = scanPaths,
        )
        assertTrue(result.clean, result.problems())
        return result.diags.groupingBy { it.name }.eachCount()
    }

    private val both = mapOf("ComposeLaunchedEffectWithoutKeys" to 1, "ComposeRememberWithoutKey" to 1)

    @Test fun productionFileIsReported() = assertEquals(both, reported())

    @Test fun testFilesAreSkipped() = assertEquals(emptyMap(), reported(testFiles = setOf("F.kt")))

    @Test fun testSupportDirectoriesAreSkipped() {
        for (dir in listOf("fakes", "mocks", "testing", "test-utils", "idling-resources", "testFixtures")) {
            assertEquals(emptyMap(), reported(scanPaths = mapOf("F.kt" to "app/$dir/F.kt")), dir)
        }
    }

    // The markers are matched on the scan spelling; a checkout under such a directory does not hide the file.
    @Test fun scanPathWithoutMarkerIsReported() =
        assertEquals(both, reported(scanPaths = mapOf("F.kt" to "app/src/main/kotlin/F.kt")))
}
