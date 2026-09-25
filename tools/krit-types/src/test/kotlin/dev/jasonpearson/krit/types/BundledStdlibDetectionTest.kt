package dev.jasonpearson.krit.types

import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.io.File
import java.nio.file.Path
import java.util.zip.ZipEntry
import java.util.zip.ZipOutputStream
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

class BundledStdlibDetectionTest {
    @TempDir
    lateinit var tmp: Path

    private fun jar(name: String, vararg entries: String): String {
        val file = tmp.resolve(name).toFile()
        ZipOutputStream(file.outputStream()).use { zip ->
            for (entry in entries) {
                zip.putNextEntry(ZipEntry(entry))
                zip.closeEntry()
            }
        }
        return file.path
    }

    @Test
    fun jdkShimAloneStillGetsTheBundledStdlib() {
        // kotlin-stdlib-jdk7/8 are empty shims since Kotlin 1.8; they can't
        // resolve listOf, so their name must not suppress the bundle.
        val shim = jar("kotlin-stdlib-jdk8-2.1.0.jar", "META-INF/MANIFEST.MF")
        val effective = effectiveClasspath(listOf(shim))
        assertEquals(2, effective.size, "bundled stdlib not appended: $effective")
        assertEquals(shim, effective.first())
        assertTrue(File(effective.last()).name.startsWith("kotlin-stdlib-"), effective.last())
    }

    @Test
    fun standardStdlibJarLeavesClasspathUnchanged() {
        val stdlib = jar("kotlin-stdlib-2.1.0.jar", "kotlin/collections/CollectionsKt.class")
        assertEquals(listOf(stdlib), effectiveClasspath(listOf(stdlib)))
    }

    @Test
    fun renamedJarContainingTheStdlibIsRecognized() {
        val renamed = jar("stdlib.jar", "kotlin/collections/CollectionsKt.class")
        assertEquals(listOf(renamed), effectiveClasspath(listOf(renamed)))
    }

    @Test
    fun classesDirectoryContainingTheStdlibIsRecognized() {
        val dir = tmp.resolve("classes").toFile()
        File(dir, "kotlin/collections").mkdirs()
        File(dir, "kotlin/collections/CollectionsKt.class").writeText("")
        assertEquals(listOf(dir.path), effectiveClasspath(listOf(dir.path)))
    }

    @Test
    fun unreadableJarIsNotTakenForTheStdlib() {
        val corrupt = tmp.resolve("broken.jar").toFile().apply { writeText("not a zip") }
        assertFalse(BundledStdlib.containsStdlib(corrupt.path))
    }
}
