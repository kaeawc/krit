package dev.jasonpearson.krit.fir.runner

import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.nio.file.Path
import kotlin.test.assertEquals

/**
 * Pins the property the Go oracle cache relies on for krit-fir: one run
 * returns facts for every file in the compilation, whatever subset was
 * requested. The cache refreshes every file's entry from each run, so a
 * dependent file can never keep facts from an earlier compilation. If a
 * future change returns only the requested files, the Go side must track
 * cross-file dependencies instead.
 */
class AnalysisSessionWholeCompilationTest {

    @TempDir
    lateinit var tmp: Path

    @Test
    fun requestingOneFileReturnsEveryCompiledFile() {
        val requested = writeKt("Use.kt", "package p\n\nfun use() = helper()\n")
        val expected = setOf(
            requested,
            writeKt("Lib.kt", "package p\n\nclass R\nfun helper(): R = R()\n"),
            // No class and no call: only a file-level checker sees these.
            writeKt("Alias.kt", "package p\n\ntypealias Id = String\n"),
            writeKt("Const.kt", "package p.consts\n\nconst val LIMIT = 3\n"),
        )

        val result = AnalysisSession(
            sourceDirs = listOf(tmp.toFile().absolutePath),
            classpath = emptyList(),
        ).analyzeFull(listOf(requested)).result

        assertEquals(expected, result.files.keys)
        assertEquals("p.consts", result.files.getValue(tmp.resolve("Const.kt").toFile().canonicalPath).packageName)
    }

    private fun writeKt(name: String, source: String): String {
        val file = tmp.resolve(name).toFile()
        file.writeText(source)
        return file.canonicalPath
    }
}
