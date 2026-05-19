package dev.jasonpearson.krit.fir.runner

import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.nio.file.Path
import kotlin.test.assertEquals
import kotlin.test.assertNotNull
import kotlin.test.assertTrue

/**
 * End-to-end coverage for annotation projection on classes and
 * members. Mirrors krit-types' shape: each annotation contributes its
 * containing class FQN to the `annotations` list (no value rendering).
 */
class AnalysisSessionAnnotationsTest {

    @TempDir
    lateinit var tmp: Path

    @Test
    fun classAndMemberAnnotationsRoundTripAsFqnLists() {
        writeKt(
            "Annotated.kt",
            """
            package com.acme.ann

            @Target(AnnotationTarget.CLASS, AnnotationTarget.FUNCTION, AnnotationTarget.PROPERTY)
            @Retention(AnnotationRetention.SOURCE)
            annotation class Tag

            @Tag
            class Box(val label: String) {
                @Tag
                fun describe(): String = label

                @Tag
                val secondary: String = ""
            }
            """.trimIndent(),
        )

        val result = AnalysisSession(
            sourceDirs = listOf(tmp.toFile().absolutePath),
            classpath = emptyList(),
        ).analyze(emptyList())

        val box = result.dependencies["com.acme.ann.Box"]
        assertNotNull(box)
        assertTrue("com.acme.ann.Tag" in box.annotations, "class annotations: ${box.annotations}")

        val describe = box.members.singleOrNull { it.kind == "function" && it.name == "describe" }
        assertNotNull(describe)
        assertEquals(listOf("com.acme.ann.Tag"), describe.annotations)

        val secondary = box.members.singleOrNull { it.kind == "property" && it.name == "secondary" }
        assertNotNull(secondary)
        assertEquals(listOf("com.acme.ann.Tag"), secondary.annotations)
    }

    @Test
    fun unannotatedDeclarationsCarryEmptyAnnotationLists() {
        writeKt(
            "Plain.kt",
            """
            package com.acme.plain

            class Plain {
                fun greet(): String = "hi"
            }
            """.trimIndent(),
        )

        val result = AnalysisSession(
            sourceDirs = listOf(tmp.toFile().absolutePath),
            classpath = emptyList(),
        ).analyze(emptyList())

        val plain = result.dependencies["com.acme.plain.Plain"]
        assertNotNull(plain)
        assertEquals(emptyList(), plain.annotations)
        plain.members.forEach { m ->
            assertEquals(emptyList(), m.annotations, "expected empty annotations on $m")
        }
    }

    private fun writeKt(name: String, source: String) {
        tmp.resolve(name).toFile().writeText(source)
    }
}
