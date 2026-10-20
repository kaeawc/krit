package dev.jasonpearson.krit.fir.incremental

import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.io.File
import java.nio.file.Path
import java.util.zip.ZipEntry
import java.util.zip.ZipOutputStream
import kotlin.test.assertEquals
import kotlin.test.assertNotEquals

/** Probes of the end-record search that the delta's own tests do not reach. */
class ZipDigestEdgeTest {
    @TempDir lateinit var tmp: Path
    private val stamp = 1_700_000_000_000L

    private fun jar(name: String, content: String, comment: String? = null): File {
        val file = tmp.resolve(name).toFile()
        ZipOutputStream(file.outputStream()).use { zip ->
            comment?.let { zip.setComment(it) }
            zip.putNextEntry(ZipEntry("a/A.class").also { it.time = stamp })
            zip.write(content.toByteArray()); zip.closeEntry()
        }
        file.setLastModified(stamp); ZipDigest.clearCache()
        return file
    }

    // A comment that ENDS in a complete 22-byte end record with an empty comment: its comment length reaches the end
    // of the file, so the delta's rule picks it over the real one.
    @Test fun aCommentEndingInAWellFormedFakeEndRecordDoesNotHideTheDirectory() {
        val fake = "PK\u0005\u0006" + "\u0000".repeat(18)
        val a = jar("a.jar", "AAAAAAAA", comment = "c".repeat(100) + fake)
        val b = jar("b.jar", "BBBBBBBB", comment = "c".repeat(100) + fake)
        assertNotEquals(ZipDigest.of(a), ZipDigest.of(b))
    }
}
