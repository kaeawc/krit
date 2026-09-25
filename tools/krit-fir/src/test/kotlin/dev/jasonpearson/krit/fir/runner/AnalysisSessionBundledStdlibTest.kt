package dev.jasonpearson.krit.fir.runner

import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.nio.file.Path
import kotlin.test.assertEquals
import kotlin.test.assertNotNull
import kotlin.test.assertTrue

class AnalysisSessionBundledStdlibTest {
    @TempDir
    lateinit var tmp: Path

    @Test
    fun stdlibCallsResolveWithAnEmptyUserClasspath() {
        val source = tmp.resolve("Stdlib.kt")
        source.toFile().writeText(
            """
            fun f() = listOf(1, 2).map { it * 2 }
            fun g(s: String) = s.uppercase()
            """.trimIndent(),
        )

        val result = AnalysisSession(listOf(tmp.toFile().absolutePath), emptyList()).analyzeFull(emptyList()).result
        val file = result.files[source.toFile().absolutePath]
        assertNotNull(file, "file payload missing: ${result.files.keys}")
        val mapCall = file.expressions.values.singleOrNull { it.callTarget?.endsWith(".map") == true }
        assertNotNull(mapCall, "map call missing: ${file.expressions.values}")
        assertTrue(mapCall.type != "<error>", "map type was ${mapCall.type}")
        assertEquals(true, mapCall.callTargetResolved)

        val uppercaseCall = file.expressions.values.singleOrNull { it.callTarget?.endsWith(".uppercase") == true }
        assertNotNull(uppercaseCall, "uppercase call missing: ${file.expressions.values}")
        assertTrue(uppercaseCall.type != "<error>", "uppercase type was ${uppercaseCall.type}")
        assertEquals(true, uppercaseCall.callTargetResolved)
    }

    @Test
    fun missingStdlibNamedEntryStillGetsTheBundledStdlib() {
        // A stdlib-named path that doesn't exist provides no classes, so it must
        // not suppress the bundle.
        val missing = "missing/path/kotlin-stdlib-2.1.0.jar"
        val effective = effectiveClasspath(listOf(missing))
        assertEquals(2, effective.size, "bundled stdlib not appended: $effective")
        assertEquals(missing, effective.first())
    }
}
