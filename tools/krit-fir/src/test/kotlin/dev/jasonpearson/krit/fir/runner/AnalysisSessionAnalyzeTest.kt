package dev.jasonpearson.krit.fir.runner

import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.io.File
import java.nio.file.Path
import kotlin.test.assertEquals
import kotlin.test.assertNotNull
import kotlin.test.assertTrue

/**
 * End-to-end coverage for [AnalysisSession.analyze] — drives a real K2
 * compilation against a small Kotlin source tree and confirms the FIR
 * class projection feeds the resulting `AnalyzeResult` with the
 * expected per-class data.
 *
 * Each test creates a fresh `AnalysisSession` so the source-file walk
 * picks up the tempdir state. K2 cold-start is ~1-3s per test — fine
 * at this scale.
 */
class AnalysisSessionAnalyzeTest {

    @TempDir
    lateinit var tmp: Path

    @Test
    fun analyzeProjectsTopLevelClassDeclarations() {
        val file = writeKt(
            "Foo.kt",
            """
            package com.acme

            class Bar {
                fun greet() = "hi"
            }
            """.trimIndent(),
        )

        val result = AnalysisSession(
            sourceDirs = listOf(tmp.toFile().absolutePath),
            classpath = emptyList(),
        ).analyze(emptyList())

        val payload = result.files[file]
        assertNotNull(payload, "file payload missing: files=${result.files.keys}")
        assertEquals("com.acme", payload.packageName)
        val bar = payload.declarations.singleOrNull { it.fqn == "com.acme.Bar" }
        assertNotNull(bar, "declaration missing: ${payload.declarations.map { it.fqn }}")
        assertEquals("class", bar.kind)
        assertEquals("public", bar.visibility)
        assertEquals(emptyList<String>(), bar.typeParameters)
        // Cross-file index agrees with the per-file declaration.
        assertEquals(bar, result.dependencies["com.acme.Bar"])
    }

    @Test
    fun analyzeCapturesClassKindAndModalityFlags() {
        writeKt(
            "Mixed.kt",
            """
            package com.acme.mixed

            sealed class Tree
            class Leaf : Tree()
            interface Shape
            object Singleton
            enum class Color { RED, GREEN }
            data class Point(val x: Int, val y: Int)
            abstract class Base
            open class Extensible
            """.trimIndent(),
        )

        val result = AnalysisSession(
            sourceDirs = listOf(tmp.toFile().absolutePath),
            classpath = emptyList(),
        ).analyze(emptyList())

        val byFqn = result.dependencies.mapValues { it.value }
        // Every class kind maps to the wire string krit-types uses.
        assertEquals("class", byFqn["com.acme.mixed.Tree"]?.kind)
        assertEquals("interface", byFqn["com.acme.mixed.Shape"]?.kind)
        assertEquals("object", byFqn["com.acme.mixed.Singleton"]?.kind)
        assertEquals("enum", byFqn["com.acme.mixed.Color"]?.kind)

        // Modality flags map to the right boolean fields.
        assertTrue(byFqn["com.acme.mixed.Tree"]?.isSealed == true, "Tree.isSealed")
        assertTrue(byFqn["com.acme.mixed.Point"]?.isData == true, "Point.isData")
        assertTrue(byFqn["com.acme.mixed.Base"]?.isAbstract == true, "Base.isAbstract")
        assertTrue(byFqn["com.acme.mixed.Extensible"]?.isOpen == true, "Extensible.isOpen")
        // Non-data, final classes carry NO modifier flags so the wire
        // payload stays minimal — krit-types' `appendClass` follows the
        // same omit-when-false rule.
        val leaf = byFqn["com.acme.mixed.Leaf"]
        assertNotNull(leaf)
        assertEquals(false, leaf.isSealed)
        assertEquals(false, leaf.isOpen)
        assertEquals(false, leaf.isData)
        assertEquals(false, leaf.isAbstract)
    }

    @Test
    fun analyzeCapturesSupertypesAndTypeParameters() {
        writeKt(
            "Generic.kt",
            """
            package com.acme.generic

            interface Container<T>
            abstract class StringContainer<T : CharSequence> : Container<T>
            """.trimIndent(),
        )

        val result = AnalysisSession(
            sourceDirs = listOf(tmp.toFile().absolutePath),
            classpath = emptyList(),
        ).analyze(emptyList())

        val container = result.dependencies["com.acme.generic.Container"]
        assertNotNull(container)
        assertEquals(listOf("T"), container.typeParameters)

        val stringContainer = result.dependencies["com.acme.generic.StringContainer"]
        assertNotNull(stringContainer)
        assertEquals(listOf("T"), stringContainer.typeParameters)
        // Supertype list captures the FIR-rendered cone type. Exact
        // rendering can vary across compiler versions; just confirm the
        // declared supertype name appears somewhere.
        assertTrue(
            stringContainer.supertypes.any { "Container" in it },
            "expected Container supertype, got ${stringContainer.supertypes}",
        )
    }

    @Test
    fun packageNameRecoveredFromClassDeclaration() {
        writeKt(
            "WithPackage.kt",
            """
            package com.acme.nested.deep

            class Marker
            """.trimIndent(),
        )

        val result = AnalysisSession(
            sourceDirs = listOf(tmp.toFile().absolutePath),
            classpath = emptyList(),
        ).analyze(emptyList())

        val payload = result.files.values.firstOrNull()
        assertNotNull(payload)
        assertEquals("com.acme.nested.deep", payload.packageName)
    }

    private fun writeKt(name: String, source: String): String {
        val file = tmp.resolve(name).toFile()
        file.writeText(source)
        // K2's source-file path goes through canonicalization (macOS
        // symlinks `/var` → `/private/var`); return the canonical path
        // so test assertions against the `files` map line up with what
        // the projection layer captured.
        return file.canonicalPath
    }
}
