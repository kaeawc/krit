package dev.krit.types

import kotlin.test.Test
import kotlin.test.assertFalse
import kotlin.test.assertTrue

class KotlinPerfTest {
    @Test
    fun mergeFromPreservesFileTimingSummaries() {
        val left = KotlinPerf(true)
        left.recordFile(
            FilePerf(
                path = "/tmp/A.kt",
                totalNs = 10_000_000,
                analysisSessionNs = 4_000_000,
                declarationsNs = 3_000_000,
                importDepsNs = 1_000_000,
                callCollectNs = 1_000_000,
                callResolveNs = 1_000_000,
                declarations = 1,
                calls = 2,
                expressions = 3,
                ok = true
            )
        )

        val right = KotlinPerf(true)
        right.recordFile(
            FilePerf(
                path = "/tmp/B.kt",
                totalNs = 20_000_000,
                analysisSessionNs = 5_000_000,
                declarationsNs = 4_000_000,
                importDepsNs = 2_000_000,
                callCollectNs = 2_000_000,
                callResolveNs = 7_000_000,
                declarations = 4,
                calls = 5,
                expressions = 6,
                ok = false
            )
        )

        left.mergeFrom(right)

        val json = left.toJson()
        assertTrue(json.contains("\"name\":\"kotlinFileTotalSummary\""))
        assertTrue(json.contains("\"count\":2"))
        assertTrue(json.contains("\"name\":\"kotlinSlowFilesTop25\""))
    }

    @Test
    fun declarationExportProfileParsesExplicitNone() {
        val profile = DeclarationExportProfile.parse("none")

        assertFalse(profile.classShell)
        assertFalse(profile.supertypes)
        assertFalse(profile.classAnnotations)
        assertFalse(profile.members)
        assertFalse(profile.memberSignatures)
        assertFalse(profile.memberAnnotations)
        assertFalse(profile.sourceDependencyClosure)
    }
}
