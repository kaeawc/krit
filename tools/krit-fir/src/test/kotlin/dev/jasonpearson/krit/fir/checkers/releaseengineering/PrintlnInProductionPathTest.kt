package dev.jasonpearson.krit.fir.checkers.releaseengineering

import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.nio.file.Files
import java.nio.file.Path
import kotlin.test.assertFalse
import kotlin.test.assertTrue

// The Go rule tests the scan's own spelling of the path, usually relative to
// the working directory; the compiler sees the absolute path. The checker
// cuts a path below the working directory down to Go's relative spelling (no
// leading `/`), so a marker counts only below the first relative component,
// exactly as in Go.
class PrintlnInProductionPathTest {
    private fun exempt(path: String, cwd: String? = "/work/project") =
        PrintlnInProduction.isNonProductionPath(path, cwd)

    @Test fun markerBelowFirstRelativeComponentIsExempt() {
        assertTrue(exempt("/work/project/app/samples/Main.kt"))
        assertTrue(exempt("/work/project/app/Sample/Main.kt"))
        assertTrue(exempt("/work/project/samples/proj/demo/src/D.kt"))
        assertTrue(exempt("/home/p1/samples/proj/demo/X.kt", cwd = "/home/p1"))
    }

    // Go sees `demo/X.kt` for `krit .`, and `samples/proj/src/X.kt` for
    // `krit samples/proj`: neither contains `/demo/` or `/samples/`, so Go
    // reports both files.
    @Test fun markerAsFirstRelativeComponentIsNotExempt() {
        assertFalse(exempt("/work/project/demos/Main.kt"))
        assertFalse(exempt("/work/project/demo/src/main/kotlin/Main.kt"))
        assertFalse(exempt("/home/p1/samples/proj/src/X.kt", cwd = "/home/p1"))
        assertFalse(exempt("/home/p1/samples/proj/src/X.kt", cwd = "/home/p1/"))
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

    // krit spells request paths from $PWD, while the JVM's user.dir is the
    // canonical directory. A working directory reached through a symlink whose
    // logical path contains `/samples/` must still scope to the project.
    @Test fun symlinkedWorkingDirectoryIsCanonicalized(@TempDir dir: Path) {
        val real = Files.createDirectories(dir.resolve("real/proj/src/main/kotlin/com/y"))
        val source = Files.writeString(real.resolve("Y.kt"), "package com.y\n")
        val linkParent = Files.createDirectories(dir.resolve("lnk/samples"))
        val link = Files.createSymbolicLink(linkParent.resolve("proj"), dir.resolve("real/proj"))
        val logicalPath = link.resolve("src/main/kotlin/com/y/Y.kt").toString()
        val canonicalRoot = dir.resolve("real/proj").toRealPath().toString()
        assertTrue(Files.exists(source))
        // The raw spellings do not share a prefix; the canonical ones do.
        assertFalse(exempt(logicalPath, cwd = canonicalRoot))
        // Symmetric case: canonical request path, logical working directory.
        assertFalse(exempt(source.toRealPath().toString(), cwd = link.toString()))
        // A marker below the project still counts after canonicalization.
        val demo = Files.createDirectories(dir.resolve("real/proj/app/demo"))
        Files.writeString(demo.resolve("D.kt"), "package d\n")
        assertTrue(exempt(link.resolve("app/demo/D.kt").toString(), cwd = canonicalRoot))
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
        assertTrue(exempt("C:\\work\\project\\app\\demo\\Main.kt", cwd = "C:\\work\\project"))
        assertFalse(exempt("C:\\work\\project\\demo\\Main.kt", cwd = "C:\\work\\project"))
        assertFalse(exempt("C:\\demo\\project\\Main.kt", cwd = "C:\\demo\\project"))
    }

    @Test fun missingPathIsNotExempt() {
        assertFalse(exempt("", cwd = null))
        assertFalse(PrintlnInProduction.isNonProductionPath(null))
    }
}
