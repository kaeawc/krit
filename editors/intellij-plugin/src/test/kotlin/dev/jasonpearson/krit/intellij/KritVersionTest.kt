package dev.jasonpearson.krit.intellij

import java.io.File
import java.nio.file.Files
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNotNull
import kotlin.test.assertNull
import kotlin.test.assertTrue

class KritVersionTest {
    @Test
    fun `parses tags, bare versions, and krit --version output`() {
        assertEquals(KritVersion(0, 3, 0), KritVersion.parse("v0.3.0"))
        assertEquals(KritVersion(1, 2, 3), KritVersion.parse("1.2.3"))
        assertEquals(KritVersion(0, 3, 1), KritVersion.parse("krit 0.3.1\n"))
        assertEquals(KritVersion(0, 4, 0, "rc1"), KritVersion.parse("krit 0.4.0-rc1"))
    }

    @Test
    fun `a source build has no version`() {
        assertNull(KritVersion.parse("krit dev"))
        assertNull(KritVersion.parse(""))
        assertNull(KritVersion.parse(null))
    }

    @Test
    fun `orders by number and puts a pre-release before its release`() {
        val ordered = listOf("0.10.0", "0.3.0", "0.3.0-rc2", "0.3.0-rc1", "1.0.0", "0.9.9")
            .map { KritVersion.parse(it)!! }
            .sorted()
            .map { it.toString() }

        assertEquals(listOf("0.3.0-rc1", "0.3.0-rc2", "0.3.0", "0.9.9", "0.10.0", "1.0.0"), ordered)
    }

    @Test
    fun `the compatible range includes its minimum and excludes its maximum`() {
        assertTrue(KritCompatibility.isCompatible(KritCompatibility.MIN_INCLUSIVE))
        assertTrue(KritCompatibility.isCompatible(KritVersion(0, 99, 0)))
        assertFalse(KritCompatibility.isCompatible(KritVersion(0, 2, 0)))
        assertFalse(KritCompatibility.isCompatible(KritVersion(0, 3, 0, "rc1")))
        assertFalse(KritCompatibility.isCompatible(KritCompatibility.MAX_EXCLUSIVE))
    }

    @Test
    fun `warns outside the range, and not for an unknown version`() {
        val old = KritCompatibility.warningFor(KritVersion(0, 2, 0), "/opt/krit")
        assertNotNull(old)
        assertTrue(old.contains("krit 0.2.0 at /opt/krit"))
        assertTrue(old.contains("Update krit"))
        assertTrue(old.contains(KritCompatibility.rangeText))

        val newer = assertNotNull(KritCompatibility.warningFor(KritVersion(1, 0, 0), "/opt/krit"))
        assertTrue(newer.contains("Update the Krit plugin"))
        assertNull(KritCompatibility.warningFor(KritVersion(0, 3, 0), "/opt/krit"))
        assertNull(KritCompatibility.warningFor(null, "/opt/krit"))
    }

    @Test
    fun `runs krit --version once per binary`() {
        val dir = Files.createTempDirectory("krit-version").toFile()
        try {
            val binary = File(dir, "krit").apply { writeText("v1") }
            var runs = 0
            val versions = KritBinaryVersions { runs++; "krit 0.3.0" }

            assertEquals(KritVersion(0, 3, 0), versions.versionOf(binary))
            assertEquals(KritVersion(0, 3, 0), versions.versionOf(binary))
            assertEquals(1, runs)

            binary.writeText("v2, an upgrade")
            versions.versionOf(binary)
            assertEquals(2, runs)
        } finally {
            dir.deleteRecursively()
        }
    }
}
