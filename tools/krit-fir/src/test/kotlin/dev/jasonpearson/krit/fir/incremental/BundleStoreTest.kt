package dev.jasonpearson.krit.fir.incremental

import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.io.File
import java.nio.file.Path
import java.util.concurrent.CountDownLatch
import java.util.concurrent.Executors
import java.util.concurrent.TimeUnit
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertFalse
import kotlin.test.assertTrue

/** Publishing, reading, eviction and writer exclusion of the bundle directory (#878). */
class BundleStoreTest {
    @TempDir lateinit var tmp: Path

    private val root get() = tmp.resolve("fir-incremental").toFile()

    private fun keyOf(i: Int) = "%064d".format(i)

    private fun state(key: String, generation: Int = 1) = PersistedState(
        key, "vk", generation, emptyMap(), emptyMap(), emptyMap(), emptyMap(),
        PersistedSnapshot(emptyMap(), emptyMap(), emptySet(), emptySet()),
    )

    /** A valid snapshot id (32 hex digits) standing for the readable name [name]. */
    private fun sid(name: String) = java.security.MessageDigest.getInstance("MD5").digest(name.toByteArray()).joinToString("") { "%02x".format(it) }

    private fun bytes(key: String, id: String, generation: Int = 1) =
        BundleCodec.encode(key, sid(id), state(key, generation), emptyList(), generation)

    private fun BundleStore.pub(key: String, id: String, staged: File?, generation: Int = 1) =
        publish(key, bytes(key, id, generation), sid(id), staged)

    private fun stage(store: BundleStore, key: String, marker: String): File =
        store.newStage(key).also { File(it, "A.class").writeText(marker) }

    @Test fun aPublishedBundleReadsBackWithItsClasses() {
        val store = BundleStore(root)
        val key = keyOf(1)
        assertTrue(store.pub(key, "id1", stage(store, key, "classes-1")))
        val back = store.read(key)
        assertEquals(sid("id1"), back.snapshotId)
        assertEquals("classes-1", File(store.classesDir(key, sid("id1")), "A.class").readText())
        assertEquals(emptyList(), store.dir(key).list()!!.filter { it.startsWith(".") }, "no staging or temp leftovers")
    }

    @Test fun aMissingBundleIsAMissNotAnError() {
        val store = BundleStore(root)
        assertEquals(BundleRejected.Reason.MISSING, assertFailsWith<BundleRejected> { store.read(keyOf(9)) }.reason)
    }

    @Test fun aSlicePublishReusesTheClassesAndRefusesWhenTheyAreGone() {
        val store = BundleStore(root)
        val key = keyOf(1)
        assertTrue(store.pub(key, "id1", stage(store, key, "x")))
        assertTrue(store.pub(key, "id1", null, 2))
        assertEquals(2, store.read(key).state.generation)
        store.classesDir(key, sid("id1")).deleteRecursively()
        assertFalse(store.pub(key, "id1", null, 3), "a bundle must not name classes that are not there")
        assertEquals(2, store.read(key).state.generation, "and the old one stays")
    }

    @Test fun aNewSnapshotKeepsThePreviousClassesForReadersOfTheOldBundle() {
        val store = BundleStore(root)
        val key = keyOf(1)
        store.pub(key, "a", stage(store, key, "a"))
        store.pub(key, "b", stage(store, key, "b"))
        assertTrue(store.classesDir(key, sid("a")).isDirectory && store.classesDir(key, sid("b")).isDirectory)
        store.pub(key, "c", stage(store, key, "c"))
        assertEquals(setOf("classes-${sid("b")}", "classes-${sid("c")}"), store.dir(key).list()!!.filter { it.startsWith("classes-") }.toSet())
    }

    @Test fun onlyTheMostRecentBundlesAreKept() {
        val store = BundleStore(root, keep = 3)
        for (i in 1..6) {
            val key = keyOf(i)
            assertTrue(store.pub(key, "s$i", stage(store, key, "$i")))
            store.bundleFile(key).setLastModified(1_000_000L + i * 10_000L)
        }
        val left = root.listFiles()!!.filter { it.isDirectory }.map { it.name }.toSet()
        assertEquals(setOf(keyOf(4), keyOf(5), keyOf(6)).map { "v${BundleCodec.VERSION}-$it" }.toSet(), left)
        assertEquals(emptyList(), root.listFiles()!!.filter { it.name.endsWith(".lock") && it.name.removeSuffix(".lock") !in left })
        // The newest survive and still read.
        for (i in 4..6) assertEquals(sid("s$i"), store.read(keyOf(i)).snapshotId)
    }

    @Test fun anOlderFormatVersionIsEvictedLikeAnyOtherBundle() {
        val store = BundleStore(root, keep = 1)
        File(root, "v0-${keyOf(0)}").also { it.mkdirs(); File(it, "bundle.bin").writeText("old"); it.setLastModified(1) }
        File(root, "v0-${keyOf(0)}/bundle.bin").setLastModified(1)
        val key = keyOf(1)
        assertTrue(store.pub(key, "s", stage(store, key, "x")))
        assertFalse(File(root, "v0-${keyOf(0)}").exists())
    }

    @Test fun aBundleBeingWrittenIsNotEvicted() {
        val store = BundleStore(root, keep = 1)
        val old = keyOf(1)
        store.pub(old, "s", stage(store, old, "x"))
        store.bundleFile(old).setLastModified(1_000L)
        val held = CountDownLatch(1)
        val release = CountDownLatch(1)
        val holder = Thread {
            BundleStore.withLock(File(root, "v${BundleCodec.VERSION}-$old.lock"), 1000) { held.countDown(); release.await(10, TimeUnit.SECONDS) }
        }.also { it.start() }
        assertTrue(held.await(5, TimeUnit.SECONDS))
        val key = keyOf(2)
        store.pub(key, "t", stage(store, key, "y"))
        assertTrue(store.dir(old).isDirectory, "the bundle a writer holds is left alone")
        release.countDown(); holder.join()
        store.evict()
        assertFalse(store.dir(old).exists(), "and goes once nobody writes it")
    }

    @Test fun concurrentWritersOfOneBundleNeverLeaveItHalfWritten() {
        val store = BundleStore(root)
        val key = keyOf(1)
        val writers = 6
        val rounds = 12
        val pool = Executors.newFixedThreadPool(writers)
        val start = CountDownLatch(1)
        val failures = java.util.concurrent.ConcurrentLinkedQueue<Throwable>()
        val readerStop = java.util.concurrent.atomic.AtomicBoolean()
        val reads = java.util.concurrent.atomic.AtomicInteger()
        val reader = Thread {
            start.await()
            while (!readerStop.get()) {
                try {
                    val b = store.read(key)
                    // Whatever bundle a reader gets, the classes it names are there or the read is a plain miss.
                    reads.incrementAndGet()
                    if (b.state.generation < 1) failures += AssertionError("generation")
                } catch (_: BundleRejected) {
                    // MISSING before the first publish is fine; a half-written file would be CORRUPT/TRUNCATED
                } catch (t: Throwable) { failures += t }
            }
        }.also { it.start() }
        val futures = (0 until writers).map { w ->
            pool.submit {
                start.await()
                for (r in 0 until rounds) {
                    val id = "w$w-r$r"
                    val generation = 1 + w * rounds + r
                    try {
                        store.pub(key, id, stage(store, key, "$w/$r"), generation)
                    } catch (t: Throwable) { failures += t }
                }
            }
        }
        start.countDown()
        futures.forEach { it.get(120, TimeUnit.SECONDS) }
        readerStop.set(true); reader.join()
        pool.shutdown()
        assertEquals(emptyList(), failures.toList())
        val final = store.read(key)
        assertTrue(store.classesDir(key, final.snapshotId).isDirectory, "the published bundle's classes exist")
        assertEquals(emptyList(), store.dir(key).list()!!.filter { it.startsWith(".") }, "no staging or temp leftovers")
        assertTrue(store.dir(key).list()!!.count { it.startsWith("classes-") } <= 2)
    }

    @Test fun aSecondProcessHoldingTheLockMakesAWriterWaitForItNotCorruptTheBundle() {
        val store = BundleStore(root)
        val key = keyOf(1)
        store.pub(key, "a", stage(store, key, "a"))
        val lockFile = File(root, "v${BundleCodec.VERSION}-$key.lock")
        val source = tmp.resolve("Hold.java").toFile()
        source.writeText(
            """
            import java.nio.channels.FileChannel;
            import java.nio.file.Path;
            import java.nio.file.StandardOpenOption;
            public class Hold {
                public static void main(String[] args) throws Exception {
                    FileChannel channel = FileChannel.open(Path.of(args[0]), StandardOpenOption.CREATE, StandardOpenOption.WRITE);
                    channel.lock();
                    System.out.println("locked");
                    System.out.flush();
                    Thread.sleep(Long.parseLong(args[1]));
                }
            }
            """.trimIndent(),
        )
        val java = File(System.getProperty("java.home"), "bin/java").path
        val process = ProcessBuilder(java, source.path, lockFile.path, "1500").redirectError(ProcessBuilder.Redirect.DISCARD).start()
        try {
            assertEquals("locked", process.inputStream.bufferedReader().readLine())
            val t0 = System.nanoTime()
            assertTrue(store.pub(key, "b", stage(store, key, "b"), 2))
            assertTrue((System.nanoTime() - t0) / 1_000_000 >= 1000, "the writer waited for the other process")
            assertEquals(sid("b"), store.read(key).snapshotId)
        } finally {
            process.destroy()
        }
    }
}
