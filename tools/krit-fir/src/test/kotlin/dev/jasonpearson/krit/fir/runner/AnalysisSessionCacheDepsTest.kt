package dev.jasonpearson.krit.fir.runner

import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.nio.file.Path
import kotlin.test.assertEquals
import kotlin.test.assertNotNull
import kotlin.test.assertTrue

/**
 * End-to-end coverage for the per-file dependency-closure projection
 * powering the `analyzeWithDeps` envelope's `cacheDeps` field. The
 * tracker records cross-file supertype links so the Go-side cache can
 * invalidate dependent files when a source dependency changes.
 */
class AnalysisSessionCacheDepsTest {

    @TempDir
    lateinit var tmp: Path

    @Test
    fun supertypeFromAnotherSourceFileIsRecordedAsDepPath() {
        val basePath = writeKt(
            "Base.kt",
            """
            package com.acme.deps

            open class Base
            """.trimIndent(),
        )
        val leafPath = writeKt(
            "Leaf.kt",
            """
            package com.acme.deps

            class Leaf : Base()
            """.trimIndent(),
        )

        val outcome = analyze()
        val depPaths = outcome.cacheDeps.depPathsByFile[leafPath]
        assertNotNull(depPaths, "leaf should have dep paths: ${outcome.cacheDeps.depPathsByFile.keys}")
        assertTrue(
            basePath in depPaths,
            "expected $basePath in $depPaths",
        )

        val perFileDeps = outcome.cacheDeps.perFileDeps[leafPath]
        assertNotNull(perFileDeps)
        val base = perFileDeps["com.acme.deps.Base"]
        assertNotNull(base, "Base ClassPayload missing in leaf's perFileDeps: $perFileDeps")
        assertEquals("class", base.kind)
    }

    @Test
    fun supertypeInSameFileIsNotRecordedAsDep() {
        // A class that extends another class in the SAME file should
        // not list itself as a dep path; krit-types' DepTracker skips
        // self-references and we mirror that.
        val path = writeKt(
            "Same.kt",
            """
            package com.acme.same

            open class Parent
            class Child : Parent()
            """.trimIndent(),
        )

        val outcome = analyze()
        val depPaths = outcome.cacheDeps.depPathsByFile[path]
        // Either no entry at all, or an empty set — both mean "no
        // cross-file deps".
        assertTrue(
            depPaths == null || path !in depPaths,
            "self-reference leaked into cacheDeps: $depPaths",
        )
    }

    @Test
    fun librarySupertypeIsNotRecordedAsDep() {
        // `kotlin.Any` (the implicit supertype of every class) is a
        // library class — origin != Source. It must NOT appear in
        // depPathsByFile because the Go-side cache layer keys on
        // source-file paths, not jar paths.
        val path = writeKt(
            "Standalone.kt",
            """
            package com.acme.standalone

            class Standalone
            """.trimIndent(),
        )

        val outcome = analyze()
        val depPaths = outcome.cacheDeps.depPathsByFile[path]
        assertTrue(
            depPaths.isNullOrEmpty(),
            "library supertype leaked into cacheDeps: $depPaths",
        )
    }

    @Test
    fun cleanProjectProducesEmptyCacheDepsView() {
        // Nothing analyzed → the view is empty across the board so
        // the wire payload stays minimal.
        val outcome = analyze()
        assertEquals(emptyMap(), outcome.cacheDeps.depPathsByFile)
        assertEquals(emptyMap(), outcome.cacheDeps.perFileDeps)
        assertEquals(emptyMap(), outcome.cacheDeps.crashedFiles)
    }

    private fun analyze(): AnalyzeOutcome =
        AnalysisSession(
            sourceDirs = listOf(tmp.toFile().absolutePath),
            classpath = emptyList(),
        ).analyzeFull(emptyList())

    private fun writeKt(name: String, source: String): String {
        val file = tmp.resolve(name).toFile()
        file.writeText(source)
        return file.canonicalPath
    }
}
