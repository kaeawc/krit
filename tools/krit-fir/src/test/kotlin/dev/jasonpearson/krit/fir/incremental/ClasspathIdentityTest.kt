package dev.jasonpearson.krit.fir.incremental

import dev.jasonpearson.krit.fir.runner.AnalysisSession
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.io.File
import java.nio.file.Path
import java.util.zip.ZipEntry
import java.util.zip.ZipOutputStream
import kotlin.test.assertEquals
import kotlin.test.assertNotEquals

/**
 * What identifies a classpath entry in the compile key (#878 review). A persisted bundle outlives the process, so a jar
 * that is rebuilt with other contents but the same size and timestamp (reproducible builds, `cp -p`, a tool that
 * restores mtimes) must not keep a stale bundle alive.
 */
class ClasspathIdentityTest {
    @TempDir lateinit var tmp: Path

    private val sessions = ArrayList<AnalysisSession>()

    @AfterEach fun cleanUp() { sessions.forEach { it.dispose() } }

    private val stamp = 1_700_000_000_000L

    private fun jar(name: String, content: String, comment: String? = null): File {
        val file = tmp.resolve(name).toFile()
        ZipOutputStream(file.outputStream()).use { zip ->
            comment?.let { zip.setComment(it) }
            zip.putNextEntry(ZipEntry("a/A.class").also { it.time = stamp })
            zip.write(content.toByteArray())
            zip.closeEntry()
        }
        file.setLastModified(stamp)
        // A new process has not read it yet.
        ZipDigest.clearCache()
        return file
    }

    private fun key(path: File) = FingerprintBuilder().addFileIdentity("classpath", path.path).build()

    @Test fun aJarReplacedByOneOfTheSameSizeAndTimestampChangesTheKey() {
        val file = jar("lib.jar", "AAAAAAAA")
        val size = file.length()
        val before = key(file)
        jar("lib.jar", "BBBBBBBB")
        assertEquals(size, file.length())
        assertEquals(stamp, file.lastModified())
        assertNotEquals(before, key(file), "same path, size and mtime, other contents")
    }

    @Test fun anUnchangedJarKeepsItsKeyAndTheDigestIsCached() {
        val file = jar("lib.jar", "AAAAAAAA")
        val first = key(file)
        assertEquals(first, key(file))
        assertEquals(first, FingerprintBuilder().addFileIdentity("classpath", file.path).build())
        // A rebuilt copy elsewhere is another entry (the path is part of the identity), but the digest itself is content.
        assertEquals(ZipDigest.of(file), ZipDigest.of(jar("copy.jar", "AAAAAAAA").also { it.setLastModified(stamp) }))
    }

    @Test fun aJarWithATrailingCommentIsDigestedToo() {
        val a = jar("a.jar", "AAAAAAAA", comment = "c".repeat(3000))
        val b = jar("b.jar", "BBBBBBBB", comment = "c".repeat(3000))
        assertNotEquals(ZipDigest.of(a), ZipDigest.of(b))
        assertNotEquals("not-a-zip", ZipDigest.of(a))
    }

    @Test fun somethingThatIsNotAZipStillHasAKeyAndNeverThrows() {
        val bogus = tmp.resolve("bogus.jar").toFile().also { it.writeText("not a zip at all, just text".repeat(10)); it.setLastModified(stamp) }
        val tiny = tmp.resolve("tiny.jar").toFile().also { it.writeText("x"); it.setLastModified(stamp) }
        val missing = tmp.resolve("missing.jar").toFile()
        assertEquals("not-a-zip", ZipDigest.of(bogus))
        assertEquals("not-a-zip", ZipDigest.of(tiny))
        key(bogus); key(tiny); key(missing)
        assertNotEquals(key(missing), key(bogus))
    }

    @Test fun aSessionsCompileKeyFollowsTheContentsOfAClasspathJar() {
        val src = tmp.resolve("src").toFile().also { it.mkdirs(); File(it, "A.kt").writeText("package p\n\nfun a() {}\n") }
        val lib = jar("dep.jar", "AAAAAAAA")
        fun keyOfNewSession() = AnalysisSession(listOf(src.path), listOf(lib.path), incrementalRecording = true, incrementalPlanning = true)
            .also { sessions += it }.compileKeyNow()
        val before = keyOfNewSession()
        assertEquals(before, keyOfNewSession())
        jar("dep.jar", "BBBBBBBB") // while no process ran
        assertNotEquals(before, keyOfNewSession())
    }

    @Test fun theContentsOfFilesInsideAClassDirectoryAreNotReadOnlyTheirIdentityCounts() {
        // Documented limit: a directory is walked for names, sizes and timestamps (hashing every class file of an
        // upstream module on each key computation would cost more than a bundle saves).
        val dir = tmp.resolve("classes").toFile().also { File(it, "p").mkdirs() }
        val f = File(dir, "p/A.class").also { it.writeText("AAAA"); it.setLastModified(stamp) }
        val before = key(dir)
        f.writeText("BBBB"); f.setLastModified(stamp)
        assertEquals(before, key(dir))
        f.writeText("BBBBB"); f.setLastModified(stamp)
        assertNotEquals(before, key(dir))
    }
}
