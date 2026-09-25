package dev.jasonpearson.krit.fir.checkers.releaseengineering

import dev.jasonpearson.krit.fir.FirRuleCompileContext
import dev.jasonpearson.krit.fir.FirRuleContext
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.nio.file.Files
import java.nio.file.Path
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue

// The Go rule tests the scan's own spelling of the path, which Go sends in the
// check request (`scanPaths`) and FirRule.scanPath returns. The checker
// matches the markers on that string exactly as Go does.
class PrintlnInProductionPathTest {
    private fun exempt(path: String?) = PrintlnInProduction.isNonProductionPath(path)

    // Go sees `samples/proj/src/X.kt` for `krit samples/proj` and `demo/X.kt`
    // for `krit .` from the checkout: neither holds `/samples/` or `/demo/`,
    // so Go reports both files.
    @Test fun relativeScanSpellingOnlyCountsMarkersBelowItsFirstComponent() {
        assertFalse(exempt("samples/proj/src/X.kt"))
        assertFalse(exempt("demo/src/main/kotlin/Main.kt"))
        assertTrue(exempt("samples/proj/demo/src/D.kt"))
        assertTrue(exempt("app/Sample/Main.kt"))
    }

    // Go scanned the absolute path (`krit /home/p1/samples/proj`): every
    // directory in it counts, the checkout's own included.
    @Test fun absoluteScanSpellingIsMatchedWhole() {
        assertTrue(exempt("/home/p1/samples/proj/src/X.kt"))
        assertFalse(exempt("/home/p1/proj/src/X.kt"))
    }

    @Test fun onlyWholeDirectoryNamesCount() {
        assertFalse(exempt("app/demonstration/Main.kt"))
        assertFalse(exempt("app/src/Demo.kt"))
        assertFalse(exempt("app/src/sample.kt"))
    }

    @Test fun scriptsAreExempt() {
        assertTrue(exempt("build.gradle"))
        assertTrue(exempt("app/build.gradle.kts"))
        assertTrue(exempt("scripts/report.main.kts"))
        assertFalse(exempt("app/src/Main.kt"))
    }

    // Go matches `/demo/` literally, so a Windows spelling never matches it.
    @Test fun windowsSeparatorsAreNotNormalized() {
        assertFalse(exempt("app\\demo\\Main.kt"))
        assertTrue(exempt("app\\build.gradle.kts"))
    }

    @Test fun missingPathIsNotExempt() {
        assertFalse(exempt(""))
        assertFalse(exempt(null))
    }

    // FirRule.scanPath maps the compiler's spelling of a requested file (as
    // requested, or canonical) to the scan spelling Go sent, falls back to the
    // requested spelling, and outside a check request to the path itself.
    @Test fun scanPathMapsCompilerPathToRequestSpelling(@TempDir dir: Path) {
        val real = Files.createDirectories(dir.resolve("real/samples/proj/src"))
        val source = Files.writeString(real.resolve("X.kt"), "package x\n")
        val other = Files.writeString(real.resolve("Y.kt"), "package y\n")
        val link = Files.createSymbolicLink(dir.resolve("link"), dir.resolve("real"))
        val requested = link.resolve("samples/proj/src/X.kt").toString()
        val requestedOther = link.resolve("samples/proj/src/Y.kt").toString()
        val context = FirRuleCompileContext(
            files = setOf(requested, requestedOther),
            scanPaths = mapOf(requested to "samples/proj/src/X.kt"),
        )
        assertEquals("samples/proj/src/X.kt", context.scanPath(requested))
        assertEquals("samples/proj/src/X.kt", context.scanPath(source.toRealPath().toString()), "canonical spelling")
        assertEquals(requestedOther, context.scanPath(other.toRealPath().toString()), "no scan spelling: as requested")
        assertNull(context.scanPath(real.resolve("Z.kt").toString()), "not a requested file")
        assertNull(context.scanPath(null))

        assertEquals(requested, PrintlnInProduction.scanPath(requested), "no check request: the path itself")
        FirRuleContext.begin(context)
        try {
            assertEquals("samples/proj/src/X.kt", PrintlnInProduction.scanPath(source.toRealPath().toString()))
            assertFalse(exempt(PrintlnInProduction.scanPath(requested)))
            assertTrue(exempt(PrintlnInProduction.scanPath(requestedOther)))
        } finally {
            FirRuleContext.end()
        }
    }
}
