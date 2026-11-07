package dev.jasonpearson.krit.fir.runner

import org.junit.jupiter.api.Assumptions.assumeTrue
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.io.File
import java.nio.file.Path
import kotlin.test.assertEquals
import kotlin.test.assertNotNull
import kotlin.test.assertTrue

/**
 * A property access of a call chain starts where the call and its receiver start, so the three share one
 * "line:col" key. The access must still be recorded under its own byte range, because rules read the
 * property's type by range and would otherwise see the receiver's type for `holder.other`.
 */
class AnalysisSessionQualifiedAccessTypesTest {

    @TempDir
    lateinit var tmp: Path

    @Test
    fun propertyAccessSharingItsStartWithACallKeepsItsOwnType() {
        val stdlib = findKotlinStdlib()
        assumeTrue(stdlib != null, "kotlin-stdlib jar not found in the Gradle cache")
        val source = """
            package com.acme.chain

            class Holder(val other: String?)

            fun use(holder: Holder) {
                holder.other.toString()
            }
        """.trimIndent()
        val file = tmp.resolve("Chain.kt").toFile().apply { writeText(source) }

        val payload = AnalysisSession(
            sourceDirs = listOf(tmp.toFile().absolutePath),
            classpath = listOfNotNull(stdlib),
        ).analyze(emptyList()).files[file.absolutePath] ?: error("no file payload")

        val start = source.indexOf("holder.other")
        val end = start + "holder.other".length
        val access = payload.expressions.values.firstOrNull { it.startByte == start && it.endByte == end }
        assertNotNull(access, "no fact for the access `holder.other`, got ${payload.expressions}")
        assertEquals("kotlin.String?", access.type)
        assertTrue(access.nullable, "the declared type of `other` is nullable")
        // The call and the receiver keep their own facts.
        assertTrue(payload.expressions.values.any { it.startByte == start && it.endByte == end + ".toString()".length })
    }

    private fun findKotlinStdlib(): String? {
        System.getenv("KOTLIN_STDLIB_JAR")?.let { if (File(it).isFile) return it }
        val home = System.getProperty("user.home") ?: return null
        val cacheRoot = File(home, ".gradle/caches/modules-2/files-2.1/org.jetbrains.kotlin/kotlin-stdlib")
        if (!cacheRoot.isDirectory) return null
        return cacheRoot.walkTopDown()
            .filter { it.isFile && it.name.startsWith("kotlin-stdlib-") && it.name.endsWith(".jar") }
            .filter { !it.name.contains("sources") && !it.name.contains("javadoc") }
            .sortedByDescending { it.name }
            .firstOrNull()?.absolutePath
    }
}
