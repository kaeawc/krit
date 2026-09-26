package dev.jasonpearson.krit.fir.tests

import dev.jasonpearson.krit.fir.FirRuleCompileContext
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

// PrngFromSystemTime cases that need a source path or extra Java sources:
// the Go rule's own test-path markers, and the clock owners the shared stubs
// do not declare (android.icu.util.Calendar, org.threeten.bp.Instant).
class PrngFromSystemTimeFilesTest {

    private fun findings(
        sources: Map<String, String>,
        testFiles: Set<String> = emptySet(),
        scanPaths: Map<String, String> = emptyMap(),
    ): Map<String, Int> {
        val result = KritFirProbe.compile(
            sources,
            FirRuleCompileContext(enabledRuleIds = setOf("PrngFromSystemTime")),
            testFiles = testFiles,
            scanPaths = scanPaths,
        )
        assertTrue(result.clean, result.problems())
        return result.diags.filter { it.name == "PrngFromSystemTime" }.groupingBy { it.file }.eachCount()
    }

    private fun seeded(pkg: String) = """
        package $pkg

        import java.util.Random
        import javax.crypto.Cipher

        class Keys {
            fun rng(): Random = Random(System.currentTimeMillis())

            fun cipher(): Cipher = Cipher.getInstance("AES/GCM/NoPadding")
        }
    """.trimIndent()

    // Go skips a lower-cased path holding /src/test/ or /src/androidtest/,
    // unless it also holds /tests/fixtures/, on the scan spelling of the path.
    // A leading `src/test/` has no slash before `src`, so Go checks it.
    @Test fun goTestPathMarkersAreMatchedOnTheScanSpelling() {
        val sources = mapOf(
            "app/src/main/kotlin/MainKeys.kt" to seeded("main"),
            "app/src/test/kotlin/UnitKeys.kt" to seeded("unit"),
            "app/src/androidTest/kotlin/DeviceKeys.kt" to seeded("device"),
            "repo/tests/fixtures/app/src/test/FixtureKeys.kt" to seeded("fixture"),
            "src/test/RootKeys.kt" to seeded("root"),
        )
        val relative = sources.keys.associateWith { it }
        assertEquals(
            mapOf("MainKeys.kt" to 1, "FixtureKeys.kt" to 1, "RootKeys.kt" to 1),
            findings(sources, scanPaths = relative),
        )
    }

    // The Go rule does not consult scanner.IsTestFile, so a request-listed
    // test file on a production path is still checked.
    @Test fun requestListedTestFileIsStillChecked() {
        val sources = mapOf("Listed.kt" to seeded("listed"))
        assertEquals(mapOf("Listed.kt" to 1), findings(sources, testFiles = setOf("Listed.kt")))
    }

    // Go matches `Calendar.getInstance().timeInMillis` and
    // `Instant.now().toEpochMilli()` whatever the owner's package, so the ICU
    // Calendar and the ThreeTen backport Instant, both system clock reads,
    // must be reported. The Java sources stand in for the platform/library.
    @Test fun icuCalendarAndThreeTenInstantAreClockReads() {
        val sources = mapOf(
            "android/icu/util/Calendar.java" to """
                package android.icu.util;

                public abstract class Calendar {
                    public static Calendar getInstance() { return null; }
                    public long getTimeInMillis() { return 0L; }
                }
            """.trimIndent(),
            "org/threeten/bp/Instant.java" to """
                package org.threeten.bp;

                public final class Instant {
                    public static Instant now() { return null; }
                    public long toEpochMilli() { return 0L; }
                }
            """.trimIndent(),
            "Clocks.kt" to """
                package clocks

                import android.icu.util.Calendar
                import java.security.SecureRandom
                import java.util.Random
                import org.threeten.bp.Instant

                fun icu(): Random = Random(Calendar.getInstance().timeInMillis)

                fun icuGetter(): Random = Random(Calendar.getInstance().getTimeInMillis())

                fun threeTen(): Random = Random(Instant.now().toEpochMilli())

                fun secure(): SecureRandom = SecureRandom()
            """.trimIndent(),
        )
        assertEquals(mapOf("Clocks.kt" to 3), findings(sources))
    }
}
