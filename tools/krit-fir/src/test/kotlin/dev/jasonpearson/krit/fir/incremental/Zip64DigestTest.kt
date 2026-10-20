package dev.jasonpearson.krit.fir.incremental

import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.io.File
import java.nio.ByteBuffer
import java.nio.ByteOrder
import java.nio.file.Path
import java.util.zip.ZipEntry
import java.util.zip.ZipOutputStream
import kotlin.test.assertEquals
import kotlin.test.assertNotEquals
import kotlin.test.assertTrue

/**
 * The zip64 branch of [ZipDigest]: an archive with more than 65,535 entries gets a zip64 end record and locator,
 * and the 32-bit fields of the plain end record are saturated. Same size and mtime, other contents, must digest
 * differently - with and without data in front of the archive and a comment behind the directory.
 */
class Zip64DigestTest {
    @TempDir lateinit var tmp: Path

    private val stamp = 1_700_000_000_000L
    private val entries = 70_000

    private fun zip64(name: String, lastContent: String, comment: String? = null, firstContent: String = "x"): File {
        val file = tmp.resolve(name).toFile()
        ZipOutputStream(file.outputStream().buffered()).use { zip ->
            comment?.let { zip.setComment(it) }
            for (i in 0 until entries) {
                zip.putNextEntry(ZipEntry("e/E$i").also { it.time = stamp })
                zip.write((if (i == entries - 1) lastContent else if (i == 0) firstContent else "x").toByteArray())
                zip.closeEntry()
            }
        }
        file.setLastModified(stamp)
        ZipDigest.clearCache()
        return file
    }

    private fun isZip64(file: File): Boolean {
        val b = file.readBytes()
        val buf = ByteBuffer.wrap(b).order(ByteOrder.LITTLE_ENDIAN)
        // the plain end record is the last 22 bytes when there is no comment
        var i = b.size - 22
        while (i >= 0 && buf.getInt(i) != 0x06054b50) i--
        return i >= 20 && buf.getInt(i - 20) == 0x07064b50 && (buf.getShort(i + 10).toInt() and 0xFFFF) == 0xFFFF
    }

    private fun prepended(name: String, of: File, junk: Int): File {
        val out = tmp.resolve(name).toFile()
        out.writeBytes("#!/bin/sh\n".repeat(junk).toByteArray() + of.readBytes())
        out.setLastModified(stamp)
        ZipDigest.clearCache()
        return out
    }

    @Test fun twoZip64ArchivesOfTheSameSizeAndTimeButOtherContentsDigestDifferently() {
        val a = zip64("a.jar", "AAAAAAAA")
        val b = zip64("b.jar", "BBBBBBBB")
        assertTrue(isZip64(a) && isZip64(b), "the fixture must be zip64")
        assertEquals(a.length(), b.length())
        val da = ZipDigest.of(a)
        val db = ZipDigest.of(b)
        listOf("not-a-zip", "bad-directory", "short-directory", "unreadable").forEach { assertNotEquals(it, da) }
        assertNotEquals(da, db)
        assertEquals(da, ZipDigest.of(zip64("a2.jar", "AAAAAAAA")), "same contents, same digest")
    }

    @Test fun zip64WithDataPrependedStillDigestsTheDirectory() {
        val a = zip64("a.jar", "AAAAAAAA")
        val b = zip64("b.jar", "BBBBBBBB")
        val pa = prepended("pa.jar", a, 7)
        val pb = prepended("pb.jar", b, 7)
        assertEquals(ZipDigest.of(a), ZipDigest.of(pa), "the same directory wherever the archive starts")
        assertNotEquals(ZipDigest.of(pa), ZipDigest.of(pb))
        assertNotEquals("bad-directory", ZipDigest.of(pa))
    }

    // The first entry's record is the first bytes of the directory: a digest window that starts too late misses it.
    @Test fun aChangeInTheFirstEntryOfAZip64DirectoryIsSeenWithAndWithoutPrependedData() {
        val a = zip64("a.jar", "AAAAAAAA", firstContent = "1111")
        val b = zip64("b.jar", "AAAAAAAA", firstContent = "2222")
        assertEquals(a.length(), b.length())
        assertNotEquals(ZipDigest.of(a), ZipDigest.of(b))
        assertNotEquals(ZipDigest.of(prepended("pa.jar", a, 5)), ZipDigest.of(prepended("pb.jar", b, 5)))
    }

    @Test fun zip64WithACommentDigestsTheDirectory() {
        val a = zip64("a.jar", "AAAAAAAA", comment = "c".repeat(3000))
        val b = zip64("b.jar", "BBBBBBBB", comment = "c".repeat(3000))
        assertNotEquals(ZipDigest.of(a), ZipDigest.of(b))
        assertNotEquals("bad-directory", ZipDigest.of(a))
        assertNotEquals("not-a-zip", ZipDigest.of(a))
    }

    // --- a zip64 archive whose plain end record is saturated (what an archive over 4 GiB or a tool that always writes
    // zip64 produces): cdSize and cdOffset exist only in the zip64 end record, so this is the case that needs the branch.
    // The JDK's ZipOutputStream saturates only the entry count, which the plain end record can still carry.

    private fun small(name: String, content: String): File {
        val file = tmp.resolve(name).toFile()
        ZipOutputStream(file.outputStream()).use { zip ->
            zip.putNextEntry(ZipEntry("a/A.class").also { it.time = stamp }); zip.write(content.toByteArray()); zip.closeEntry()
            zip.putNextEntry(ZipEntry("a/B.class").also { it.time = stamp }); zip.write("same".toByteArray()); zip.closeEntry()
        }
        return file
    }

    private fun saturated(name: String, plain: File, extensible: Int = 0): File {
        val b = plain.readBytes()
        val bb = ByteBuffer.wrap(b).order(ByteOrder.LITTLE_ENDIAN)
        val e = b.size - 22
        check(bb.getInt(e) == 0x06054b50) { "plain jar has no comment" }
        val count = bb.getShort(e + 10).toLong(); val cdSize = bb.getInt(e + 12).toLong(); val cdOffset = bb.getInt(e + 16).toLong()
        val rec = ByteBuffer.allocate(56 + extensible).order(ByteOrder.LITTLE_ENDIAN)
            .putInt(0x06064b50).putLong(44L + extensible).putShort(45).putShort(45).putInt(0).putInt(0)
            .putLong(count).putLong(count).putLong(cdSize).putLong(cdOffset)
        val loc = ByteBuffer.allocate(20).order(ByteOrder.LITTLE_ENDIAN).putInt(0x07064b50).putInt(0).putLong(e.toLong()).putInt(1)
        val end = ByteBuffer.allocate(22).order(ByteOrder.LITTLE_ENDIAN)
            .putInt(0x06054b50).putShort(0).putShort(0).putShort(-1).putShort(-1).putInt(-1).putInt(-1).putShort(0)
        val out = tmp.resolve(name).toFile()
        out.writeBytes(b.copyOfRange(0, e) + rec.array() + loc.array() + end.array())
        out.setLastModified(stamp); ZipDigest.clearCache()
        return out
    }

    @Test fun aSaturatedZip64EndRecordIsReadFromTheZip64Record() {
        val a = saturated("a.jar", small("pa.jar", "AAAAAAAA"))
        val b = saturated("b.jar", small("pb.jar", "BBBBBBBB"))
        assertEquals(a.length(), b.length())
        val da = ZipDigest.of(a)
        listOf("not-a-zip", "bad-directory", "short-directory", "unreadable").forEach { assertNotEquals(it, da) }
        assertNotEquals(da, ZipDigest.of(b))
        // and it is the digest of the same directory the plain archive has
        assertEquals(ZipDigest.of(small("plain.jar", "AAAAAAAA").also { it.setLastModified(stamp); ZipDigest.clearCache() }), da)
    }

    @Test fun aSaturatedZip64ArchiveWithDataPrependedIsReadFromTheZip64Record() {
        val a = saturated("a.jar", small("pa.jar", "AAAAAAAA"))
        val b = saturated("b.jar", small("pb.jar", "BBBBBBBB"))
        val pa = prepended("pra.jar", a, 9); val pb = prepended("prb.jar", b, 9)
        assertEquals(ZipDigest.of(a), ZipDigest.of(pa))
        assertNotEquals(ZipDigest.of(pa), ZipDigest.of(pb))
    }

    // Valid per APPNOTE 4.3.14: the zip64 end record may carry an extensible data sector after the fixed 56 bytes.
    @Test fun aZip64EndRecordWithAnExtensibleDataSectorIsStillRead() {
        val a = saturated("a.jar", small("pa.jar", "AAAAAAAA"), extensible = 16)
        val b = saturated("b.jar", small("pb.jar", "BBBBBBBB"), extensible = 16)
        assertNotEquals("bad-directory", ZipDigest.of(a))
        assertNotEquals(ZipDigest.of(a), ZipDigest.of(b))
    }
}
