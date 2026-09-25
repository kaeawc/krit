package dev.jasonpearson.krit.fir

import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.io.File
import java.nio.file.Path
import kotlin.test.assertTrue

/**
 * The Go cache runs large miss sets through the one-shot CLI with
 * `--cache-deps-out`. Without that file the cache is not written, so every
 * later run misses again.
 */
class OneShotCacheDepsTest {

    @TempDir
    lateinit var tmp: Path

    @Test
    fun oneShotWritesTheCacheDepsFile() {
        val src = tmp.resolve("src").toFile().apply { mkdirs() }
        File(src, "Base.kt").writeText("package p\n\nopen class Base\n")
        File(src, "Leaf.kt").writeText("package p\n\nclass Leaf : Base()\n")
        val output = tmp.resolve("types.json").toFile()
        val deps = tmp.resolve("deps.json").toFile()

        runOneShot(
            sources = listOf(src.absolutePath),
            outputPath = output.absolutePath,
            filesListPath = null,
            classpath = emptyList(),
            cacheDepsOutPath = deps.absolutePath,
        )

        assertTrue(output.length() > 0, "no --output written")
        val body = deps.readText()
        assertTrue(""""approximation":"fir-whole-compilation"""" in body, body)
        assertTrue(""""depPaths":["${File(src, "Base.kt").absolutePath}"]""" in body, body)
    }
}
