package dev.jasonpearson.krit.fir.checkers.releaseengineering

import org.junit.jupiter.api.Test
import kotlin.test.assertFalse
import kotlin.test.assertTrue

// The Go rule tests the scan's own spelling of the path, usually relative to
// the working directory; the compiler sees the absolute path. Directory
// markers above the working directory must not exempt a whole project, and a
// marker directory at the top of the scanned tree still counts.
class PrintlnInProductionPathTest {
    private fun exempt(path: String, cwd: String? = "/work/project") =
        PrintlnInProduction.isNonProductionPath(path, cwd)

    @Test fun markerBelowWorkingDirectoryIsExempt() {
        assertTrue(exempt("/work/project/app/samples/Main.kt"))
        assertTrue(exempt("/work/project/app/Sample/Main.kt"))
        assertTrue(exempt("/work/project/demos/Main.kt"))
        assertTrue(exempt("/work/project/demo/src/main/kotlin/Main.kt"))
    }

    @Test fun markerAtOrAboveWorkingDirectoryIsNotExempt() {
        assertFalse(exempt("/home/samples/project/src/Main.kt", cwd = "/home/samples/project"))
        assertFalse(exempt("/home/demo/src/Main.kt", cwd = "/home/demo"))
        assertFalse(exempt("/home/demo/src/Main.kt", cwd = "/home/demo/"))
    }

    @Test fun pathOutsideWorkingDirectoryIsMatchedWhole() {
        assertTrue(exempt("/elsewhere/demo/Main.kt"))
        assertFalse(exempt("/elsewhere/app/Main.kt"))
        assertTrue(exempt("/elsewhere/demo/Main.kt", cwd = null))
        assertTrue(exempt("/work/projectdemo/demo/Main.kt"))
    }

    @Test fun onlyWholeDirectoryNamesCount() {
        assertFalse(exempt("/work/project/demonstration/Main.kt"))
        assertFalse(exempt("/work/project/src/Demo.kt"))
        assertFalse(exempt("/work/project/src/sample.kt"))
    }

    @Test fun scriptsAreExempt() {
        assertTrue(exempt("/work/project/build.gradle.kts"))
        assertTrue(exempt("/work/project/scripts/report.main.kts"))
        assertFalse(exempt("/work/project/src/Main.kt"))
    }

    @Test fun windowsSeparatorsAreNormalized() {
        assertTrue(exempt("C:\\work\\project\\demo\\Main.kt", cwd = "C:\\work\\project"))
        assertFalse(exempt("C:\\demo\\project\\Main.kt", cwd = "C:\\demo\\project"))
    }

    @Test fun missingPathIsNotExempt() {
        assertFalse(exempt("", cwd = null))
        assertFalse(PrintlnInProduction.isNonProductionPath(null))
    }
}
