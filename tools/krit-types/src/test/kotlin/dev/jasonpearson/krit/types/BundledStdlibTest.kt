package dev.jasonpearson.krit.types

import com.intellij.openapi.util.Disposer
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.nio.file.Path
import kotlin.test.assertEquals
import kotlin.test.assertTrue

class BundledStdlibTest {
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
        val args = ParsedArgs(
            sourceDirs = listOf(tmp.toString()),
            classpath = emptyList(),
            jdkHome = null,
            output = null,
            diagnostics = false,
        )
        val disposable = Disposer.newDisposable("bundled-stdlib-test")
        try {
            val output = analyzeAndExport(disposable, args)
            assertTrue(
                output.contains("\"callTarget\": \"kotlin.collections.map\", \"callTargetResolved\": true"),
                "map call did not resolve to stdlib: $output",
            )
            assertTrue(
                output.contains("\"callTarget\": \"kotlin.text.uppercase\", \"callTargetResolved\": true"),
                "uppercase call did not resolve to stdlib: $output",
            )
        } finally {
            Disposer.dispose(disposable)
        }
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
