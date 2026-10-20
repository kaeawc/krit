package dev.jasonpearson.krit.fir.incremental

import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.io.File
import java.io.RandomAccessFile
import java.nio.file.Files
import java.nio.file.LinkOption
import java.nio.file.Path
import java.nio.file.attribute.BasicFileAttributeView
import java.nio.file.attribute.FileTime
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertFalse
import kotlin.test.assertNotNull
import kotlin.test.assertNull
import kotlin.test.assertTrue

/**
 * The bundle directory is in the project tree, so it is not trusted (#878 review): entries planted in it must never make
 * the store delete, copy or read anything outside it, delete a directory it did not create, or load a huge file.
 */
class BundleStoreHostileDirTest {
    @TempDir lateinit var tmp: Path

    private val root get() = tmp.resolve("fir-incremental").toFile()

    private fun keyOf(i: Int) = "%064d".format(i)

    private fun sid(name: String) = java.security.MessageDigest.getInstance("MD5").digest(name.toByteArray()).joinToString("") { "%02x".format(it) }

    private fun state(key: String) = PersistedState(
        key, "vk", 1, emptyMap(), emptyMap(), emptyMap(), emptyMap(),
        PersistedSnapshot(emptyMap(), emptyMap(), emptySet(), emptySet()),
    )

    private fun publish(store: BundleStore, key: String, id: String, marker: String = id): Boolean {
        val stage = store.newStage(key).also { File(it, "A.class").writeText(marker) }
        return store.publish(key, BundleCodec.encode(key, sid(id), state(key), emptyList(), 1), sid(id), stage)
    }

    private fun old(path: File) {
        Files.getFileAttributeView(path.toPath(), BasicFileAttributeView::class.java, LinkOption.NOFOLLOW_LINKS)
            .setTimes(FileTime.fromMillis(1000), null, null)
    }

    /** A directory outside the bundle root with files in it, one nested; old, so any age-based rule would take it. */
    private fun victim(): File = tmp.resolve("victim").toFile().also {
        File(it, "sub").mkdirs()
        File(it, "precious.txt").writeText("do not delete")
        File(it, "sub/more.txt").writeText("x")
        it.setLastModified(1000)
    }

    private fun assertIntact(victim: File) {
        assertEquals("do not delete", File(victim, "precious.txt").readText())
        assertEquals("x", File(victim, "sub/more.txt").readText())
    }

    private fun link(at: File, to: File) {
        at.parentFile.mkdirs()
        Files.createSymbolicLink(at.toPath(), to.toPath())
    }

    // ---- symbolic links ---------------------------------------------------------------------------------------------

    @Test fun evictionNeverFollowsALinkedBundleDirectory() {
        val store = BundleStore(root, keep = 1)
        val v = victim()
        link(File(root, "v1-evil"), v)
        link(File(root, "v1-${keyOf(7)}"), v) // the right name, still a link
        for (i in 1..3) publish(store, keyOf(i), "s$i")
        assertIntact(v)
        assertTrue(Files.isSymbolicLink(File(root, "v1-${keyOf(7)}").toPath()), "a link is left alone, not even unlinked")
    }

    @Test fun pruningNeverFollowsALinkedClassesDirectory() {
        val store = BundleStore(root)
        val v = victim()
        val k = keyOf(1)
        publish(store, k, "a")
        link(File(store.dir(k), "classes-evil"), v)
        link(File(store.dir(k), "classes-${sid("evil")}"), v)
        publish(store, k, "b")
        publish(store, k, "c")
        assertIntact(v)
    }

    @Test fun sweepingNeverFollowsALinkedLeftover() {
        val store = BundleStore(root)
        val v = victim()
        val k = keyOf(1)
        publish(store, k, "a")
        for (name in listOf(".stage-evil", ".stage-${java.util.UUID.randomUUID()}", ".bundle-${java.util.UUID.randomUUID()}")) {
            val l = File(store.dir(k), name)
            link(l, v)
            old(l)
        }
        publish(store, k, "b")
        assertIntact(v)
    }

    @Test fun aLinkInsideADoomedTreeIsUnlinkedNotFollowed() {
        val store = BundleStore(root)
        val v = victim()
        val k = keyOf(1)
        publish(store, k, "a")
        val inner = File(store.classesDir(k, sid("a")), "pkg")
        link(inner, v)
        link(File(store.classesDir(k, sid("a")), "file-link"), File(v, "precious.txt"))
        publish(store, k, "b")
        publish(store, k, "c") // classes-a is doomed now
        assertFalse(store.classesDir(k, sid("a")).exists(), "the tree itself is deleted")
        assertIntact(v)
    }

    @Test fun anEvictedBundleWithALinkInsideIsDeletedWithoutFollowingIt() {
        val store = BundleStore(root, keep = 1)
        val v = victim()
        val doomed = File(root, "v1-${keyOf(9)}")
        File(doomed, "classes-${sid("x")}").mkdirs()
        link(File(doomed, "classes-${sid("x")}/escape"), v)
        old(doomed)
        publish(store, keyOf(1), "s")
        assertFalse(doomed.exists())
        assertIntact(v)
    }

    @Test fun copyingATreeSkipsLinksAndNeverCopiesTheirTargets() {
        val v = victim()
        val from = tmp.resolve("from").toFile()
        File(from, "real").mkdirs()
        File(from, "real/A.class").writeText("a")
        link(File(from, "dir-link"), v)
        link(File(from, "file-link"), File(v, "precious.txt"))
        val to = tmp.resolve("to").toFile()
        BundleStore.copyTree(from, to)
        assertEquals("a", File(to, "real/A.class").readText())
        assertFalse(File(to, "dir-link").exists() || Files.isSymbolicLink(File(to, "dir-link").toPath()))
        assertFalse(File(to, "file-link").exists() || Files.isSymbolicLink(File(to, "file-link").toPath()))
        val linkedRoot = tmp.resolve("linked-root").toFile()
        link(linkedRoot, from)
        assertFailsWith<java.io.IOException> { BundleStore.copyTree(linkedRoot, tmp.resolve("to2").toFile()) }
    }

    @Test fun aBundleDirectoryThatIsALinkIsNeverWrittenThrough() {
        val store = BundleStore(root)
        val v = victim()
        val k = keyOf(1)
        link(store.dir(k), v)
        assertFailsWith<java.io.IOException> { store.newStage(k) }
        assertFailsWith<java.io.IOException> { publish(store, k, "a") }
        assertIntact(v)
        assertEquals(setOf("precious.txt", "sub"), v.list()!!.toSet(), "nothing was added to the target")
    }

    @Test fun aBundleFileThatIsALinkIsNotRead() {
        val store = BundleStore(root)
        val k = keyOf(1)
        publish(store, k, "a")
        val real = tmp.resolve("elsewhere.bin").toFile().also { it.writeBytes(store.bundleFile(k).readBytes()) }
        store.bundleFile(k).delete()
        link(store.bundleFile(k), real)
        assertEquals(BundleRejected.Reason.IO, assertFailsWith<BundleRejected> { store.read(k) }.reason)
    }

    // ---- names ----------------------------------------------------------------------------------------------------------

    @Test fun evictionOnlyTouchesDirectoriesNamedLikeBundles() {
        val store = BundleStore(root, keep = 1)
        root.mkdirs()
        val others = listOf("vendor", "venv", "videos", "v1-evil", "v1-${"a".repeat(63)}", "v-${keyOf(3)}", "v1-${keyOf(3)}x", "V1-${keyOf(3)}")
        for (n in others) File(root, n).also { it.mkdirs(); File(it, "keep.txt").writeText("x"); old(it) }
        for (i in 1..3) publish(store, keyOf(i), "s$i")
        for (n in others) assertTrue(File(File(root, n), "keep.txt").exists(), "$n survives")
        assertEquals(emptyList(), root.list()!!.filter { it.endsWith(".lock") && it.removeSuffix(".lock") !in root.list()!! }, "no lock files are left behind")
        for (n in others) assertFalse(File(root, "$n.lock").exists(), "$n got no lock file")
    }

    @Test fun onlyTheNamesTheStoreCreatesAreSweptFromABundleDirectory() {
        val store = BundleStore(root)
        val k = keyOf(1)
        publish(store, k, "a")
        val keep = listOf(".stage-notauuid", ".bundle-x", "notes.txt", "classes-notahexid").map { File(store.dir(k), it) }
        for (f in keep) { f.mkdirs(); File(f, "k").writeText("k"); old(f) }
        val sweep = File(store.dir(k), ".stage-${java.util.UUID.randomUUID()}").also { it.mkdirs(); old(it) }
        publish(store, k, "b")
        for (f in keep) assertTrue(File(f, "k").exists(), "${f.name} survives")
        assertFalse(sweep.exists(), "a dead writer's staging directory goes")
    }

    @Test fun aSnapshotIdOrCompileKeyIsNeverUsedAsAPathUnlessItIsExactlyTheFormat() {
        val store = BundleStore(root)
        val k = keyOf(1)
        publish(store, k, "x")
        for (bad in listOf("x/../../../outside", "..", "", "ABCDEF0123456789ABCDEF0123456789", sid("x") + "0", sid("x").dropLast(1))) {
            assertFailsWith<IllegalArgumentException>(bad) { store.classesDir(k, bad) }
        }
        for (bad in listOf("../x", "", "g".repeat(64), keyOf(1) + "0")) assertFailsWith<IllegalArgumentException>(bad) { store.dir(bad) }
        assertFailsWith<IllegalArgumentException> { store.publish(k, ByteArray(0), "../../x", null) }
    }

    // ---- size ----------------------------------------------------------------------------------------------------------

    @Test fun aHugeSparseBundleFileIsRejectedBeforeItIsRead() {
        val store = BundleStore(root)
        val k = keyOf(1)
        store.dir(k).mkdirs()
        RandomAccessFile(store.bundleFile(k), "rw").use { it.setLength(3L * 1024 * 1024 * 1024) }
        val rejected = assertFailsWith<BundleRejected> { store.read(k) }
        assertEquals(BundleRejected.Reason.IO, rejected.reason)
    }

    @Test fun theSizeLimitFollowsTheHeapAndTheConstructor() {
        assertTrue(BundleStore.defaultMaxBundleBytes() <= 512L * 1024 * 1024)
        assertTrue(BundleStore.defaultMaxBundleBytes() <= Runtime.getRuntime().maxMemory() / 4)
        val store = BundleStore(root, maxBundleBytes = 64)
        val k = keyOf(1)
        store.dir(k).mkdirs()
        store.bundleFile(k).writeBytes(ByteArray(65))
        assertEquals(BundleRejected.Reason.IO, assertFailsWith<BundleRejected> { store.read(k) }.reason)
        store.bundleFile(k).writeBytes(ByteArray(64))
        assertEquals(BundleRejected.Reason.CORRUPT, assertFailsWith<BundleRejected> { store.read(k) }.reason, "at the limit it is read, and is not a bundle")
    }

    // ---- which classes survive a publish -----------------------------------------------------------------------------

    @Test fun thePreviousBundlesClassesSurviveWhateverTheTimestampsSay() {
        repeat(40) { round ->
            val store = BundleStore(tmp.resolve("r$round").toFile())
            val k = keyOf(1)
            publish(store, k, "a")
            publish(store, k, "b")
            // Timestamps that tie, or that put the referenced directory behind an older one, as a coarse filesystem clock does.
            val all = listOf("a", "b").map { store.classesDir(k, sid(it)) }
            val stamps = if (round % 2 == 0) listOf(5_000L, 5_000L) else listOf(9_000L, 1_000L)
            all.zip(stamps).forEach { (d, t) -> d.setLastModified(t) }
            publish(store, k, "c")
            assertEquals(
                setOf("classes-${sid("b")}", "classes-${sid("c")}"),
                store.dir(k).list()!!.filter { it.startsWith("classes-") }.toSet(), "round $round",
            )
            assertEquals(sid("c"), store.read(k).snapshotId)
        }
    }

    @Test fun theClassesTheOldBundleNamesAreKeptNotTheNewestByTime() {
        val store = BundleStore(root)
        val k = keyOf(1)
        publish(store, k, "a")
        publish(store, k, "b")
        // A third directory that is newer than both but that no bundle names (a dead writer's): it goes, "b" stays.
        val stray = File(store.dir(k), "classes-${sid("stray")}").also { it.mkdirs(); File(it, "A.class").writeText("s") }
        stray.setLastModified(System.currentTimeMillis() + 60_000)
        publish(store, k, "c")
        assertFalse(stray.exists())
        assertTrue(store.classesDir(k, sid("b")).isDirectory && store.classesDir(k, sid("c")).isDirectory)
        assertFalse(store.classesDir(k, sid("a")).exists())
    }

    // ---- a publish that must not happen ------------------------------------------------------------------------------

    @Test fun aPublishThatMayNotCommitLeavesNothingBehind() {
        val store = BundleStore(root)
        val k = keyOf(1)
        val stage = store.newStage(k).also { File(it, "A.class").writeText("a") }
        assertFalse(store.publish(k, BundleCodec.encode(k, sid("a"), state(k), emptyList(), 1), sid("a"), stage, mayCommit = { false }))
        assertFalse(store.bundleFile(k).exists())
        assertFalse(store.classesDir(k, sid("a")).exists())
        assertFalse(stage.exists(), "the staged copy is removed")
    }

    // ---- the directory a request may name ----------------------------------------------------------------------------

    @Test fun onlyAKritFirIncrementalDirectoryIsTrustedFromARequest() {
        val warnings = ArrayList<String>()
        val repo = tmp.resolve("repo").toFile().also { it.mkdirs() }
        val good = File(repo, ".krit/fir-incremental")
        assertEquals(good.canonicalFile, BundleStore.trustedRoot(good.path) { warnings += it })
        assertEquals(good.canonicalFile, BundleStore.trustedRoot(File(repo, ".krit/other/../fir-incremental").path) { warnings += it })
        assertTrue(warnings.isEmpty())
        assertNull(BundleStore.trustedRoot(null) { warnings += it })
        assertNull(BundleStore.trustedRoot("  ") { warnings += it })
        assertTrue(warnings.isEmpty(), "absent is not a warning")
        for (bad in listOf("/", "/etc", repo.path, File(repo, ".krit").path, File(repo, "fir-incremental").path, File(repo, ".krit/fir-incremental/x").path, "relative/.krit/fir-incremental", File(repo, ".krit/fir-incremental/..").path)) {
            warnings.clear()
            assertNull(BundleStore.trustedRoot(bad) { warnings += it }, bad)
            assertEquals(1, warnings.size, "a warning for $bad")
        }
        // A link that makes the directory something else once resolved.
        val elsewhere = tmp.resolve("elsewhere").toFile().also { it.mkdirs() }
        link(File(repo, ".krit2"), elsewhere)
        warnings.clear()
        assertNull(BundleStore.trustedRoot(File(repo, ".krit2/fir-incremental").path) { warnings += it })
        assertNotNull(warnings.singleOrNull())
    }
}
