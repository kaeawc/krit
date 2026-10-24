package dev.jasonpearson.krit.intellij

import java.nio.file.Files
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNotEquals
import kotlin.test.assertNull

class KritBinaryHashTest {
    @Test
    fun `hashes file contents as lower-case sha256 hex`() {
        val f = Files.createTempFile("krit-hash", ".bin").toFile()
        f.deleteOnExit()
        f.writeText("abc")
        assertEquals("ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad", KritBinaryHash.of(f))
    }

    @Test
    fun `an upgraded binary at the same path gets a new hash`() {
        val f = Files.createTempFile("krit-hash", ".bin").toFile()
        f.deleteOnExit()
        f.writeText("v1")
        val first = KritBinaryHash.of(f)
        f.writeText("version two")
        f.setLastModified(f.lastModified() + 2_000)
        assertNotEquals(first, KritBinaryHash.of(f))
    }

    @Test
    fun `an unreadable file has no hash`() {
        val dir = Files.createTempDirectory("krit-hash").toFile()
        dir.deleteOnExit()
        assertNull(KritBinaryHash.of(dir.resolve("missing")))
    }
}
