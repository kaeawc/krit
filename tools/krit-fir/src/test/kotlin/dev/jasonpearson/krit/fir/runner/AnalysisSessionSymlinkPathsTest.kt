package dev.jasonpearson.krit.fir.runner

import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.io.File
import java.nio.file.Files
import java.nio.file.Path
import kotlin.test.assertEquals
import kotlin.test.assertTrue

class AnalysisSessionSymlinkPathsTest {
    @TempDir
    lateinit var tmp: Path

    @Test
    fun symlinkedRootUsesRequestedPathsOnceAcrossOracleAndCheck() {
        val real = Files.createDirectory(tmp.resolve("real"))
        val linked = Files.createSymbolicLink(tmp.resolve("linked"), real)
        Files.writeString(real.resolve("Base.kt"), "package p\nopen class Smoke\n")
        Files.writeString(
            real.resolve("Leaf.kt"),
            "package p\nclass Leaf : Smoke()\nfun redundant(): String { val s: String = \"x\"; return s ?: \"fallback\" }\n",
        )
        val base = linked.resolve("Base.kt").toString()
        val leaf = linked.resolve("Leaf.kt").toString()
        val stdlib = File(Unit::class.java.protectionDomain.codeSource.location.toURI()).path
        val session = AnalysisSession(listOf(linked.toString()), listOf(stdlib))

        val outcome = session.analyzeFull(listOf(leaf, base))
        assertEquals(setOf(base, leaf), outcome.result.files.keys)
        assertTrue(outcome.result.files.getValue(leaf).diagnostics.any { it.factoryName == "USELESS_ELVIS" })
        assertEquals(setOf(leaf), outcome.cacheDeps.depPathsByFile.keys)
        assertEquals(setOf(base), outcome.cacheDeps.depPathsByFile.getValue(leaf))
        assertEquals(setOf(leaf), outcome.cacheDeps.perFileDeps.keys)
        assertTrue(outcome.cacheDeps.crashedFiles.keys.all { it == base || it == leaf })

        val check = session.check(1, listOf(FileRef(base), FileRef(leaf)), setOf("SmokeChecker"))
        assertTrue(check.crashed.isEmpty(), "duplicate compilation errors: ${check.crashed}")
        assertEquals(listOf(base), check.findings.map { it.path })
    }
}
