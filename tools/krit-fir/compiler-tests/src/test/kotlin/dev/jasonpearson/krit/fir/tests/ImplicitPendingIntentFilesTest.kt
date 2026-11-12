package dev.jasonpearson.krit.fir.tests

import dev.jasonpearson.krit.fir.FirRuleCompileContext
import dev.jasonpearson.krit.fir.SdkLevels
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

// ImplicitPendingIntent cases that need request facts: the module's targetSdk and the file's path.
class ImplicitPendingIntentFilesTest {

    private fun source(pkg: String) = """
        package $pkg

        import android.app.PendingIntent
        import android.content.Context
        import android.content.Intent

        fun pending(context: Context, intent: Intent) =
            PendingIntent.getActivity(context, 0, intent, PendingIntent.FLAG_UPDATE_CURRENT)
    """.trimIndent()

    private fun findings(
        sources: Map<String, String>,
        testFiles: Set<String> = emptySet(),
        scanPaths: Map<String, String> = emptyMap(),
        sdkLevels: Map<String, SdkLevels> = emptyMap(),
    ): Map<String, Int> {
        val result = KritFirProbe.compile(
            sources,
            FirRuleCompileContext(enabledRuleIds = setOf("ImplicitPendingIntent")),
            testFiles = testFiles,
            scanPaths = scanPaths,
            sdkLevels = sdkLevels,
        )
        assertTrue(result.clean, result.problems())
        return result.diags.filter { it.name == "ImplicitPendingIntent" }.groupingBy { it.file }.eachCount()
    }

    // Go skips a module whose targetSdk is known and below 31; an unknown level (0) is checked.
    @Test fun targetSdkBelow31IsSkipped() {
        val sources = mapOf(
            "Old.kt" to source("old"),
            "Boundary.kt" to source("boundary"),
            "Current.kt" to source("current"),
            "Unknown.kt" to source("unknown"),
            "MinOnly.kt" to source("minonly"),
        )
        val levels = mapOf(
            "Old.kt" to SdkLevels(minSdk = 21, targetSdk = 30),
            "Boundary.kt" to SdkLevels(minSdk = 21, targetSdk = 31),
            "Current.kt" to SdkLevels(minSdk = 21, targetSdk = 35),
            "MinOnly.kt" to SdkLevels(minSdk = 21),
        )
        assertEquals(
            mapOf("Boundary.kt" to 1, "Current.kt" to 1, "Unknown.kt" to 1, "MinOnly.kt" to 1),
            findings(sources, sdkLevels = levels),
        )
    }

    @Test fun requestListedTestFileIsSkipped() {
        val sources = mapOf("Listed.kt" to source("listed"), "Plain.kt" to source("plain"))
        assertEquals(mapOf("Plain.kt" to 1), findings(sources, testFiles = setOf("Listed.kt")))
    }

    // Go also skips `/testFixtures/` directories and directories named like a test module, on the scan spelling.
    @Test fun testFixtureSourcesAreSkipped() {
        val sources = mapOf(
            "app/src/main/kotlin/Main.kt" to source("main"),
            "app/src/testFixtures/kotlin/Fixture.kt" to source("fixture"),
            "feature-test/src/main/kotlin/Module.kt" to source("module"),
            "feature-testing/src/main/kotlin/Testing.kt" to source("testing"),
            "app-testfixtures/src/main/kotlin/Fixtures.kt" to source("fixtures"),
            "contest/src/main/kotlin/Contest.kt" to source("contest"),
        )
        assertEquals(
            mapOf("Main.kt" to 1, "Contest.kt" to 1),
            findings(sources, scanPaths = sources.keys.associateWith { it }),
        )
    }
}
