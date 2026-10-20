package dev.jasonpearson.krit.fir.incremental

import dev.jasonpearson.krit.fir.runner.AnalysisSession
import dev.jasonpearson.krit.fir.runner.BatchResult
import dev.jasonpearson.krit.fir.runner.BundleLoad
import dev.jasonpearson.krit.fir.runner.FileRef
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.io.File
import java.nio.file.Path
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNotNull
import kotlin.test.assertNull
import kotlin.test.assertTrue

/**
 * Persisting the incremental bundle (#878): a new session on the same root starts warm, and everything that can go
 * wrong with a bundle (corrupt, truncated, another version or key, a missing class directory) is a plain cold check.
 */
class PersistedBundleTest {
    @TempDir lateinit var tmp: Path

    private val src get() = tmp.resolve("src").toFile()
    private val snaps get() = tmp.resolve("snapshots").toFile()
    private val root get() = tmp.resolve("repo/.krit/fir-incremental").toFile()
    private val rules = setOf("ProtocolProbe")
    private val sessions = ArrayList<AnalysisSession>()

    @AfterEach fun cleanUp() {
        IncrementalTestHook.onStage = null
        sessions.forEach { it.dispose() }
    }

    private fun write(rel: String, text: String) = src.writeSource(rel, text)
    private fun refs() = kotlinFilesIn(src).map { FileRef(it) }

    private fun probeModule() {
        write("Api.kt", "package p\n\nfun protocolProbe() {}\nfun base(): Int = 1\n")
        write("Use.kt", "package p\n\nfun u() { protocolProbe(); base() }\n")
        write("Other.kt", "package p\n\nfun o() { protocolProbe() }\n")
        write("Third.kt", "package p\n\nfun t(): Int = base() + 1\n")
    }

    private fun session(
        persistRoot: File? = root, classpath: List<String> = listOf(testStdlib), debounce: Long? = null,
    ): AnalysisSession =
        AnalysisSession(listOf(src.path), classpath, incrementalRecording = true, incrementalPlanning = true)
            .also {
                it.snapshotRootOverride = snaps
                it.configurePersistence(persistRoot)
                debounce?.let { d -> it.persistence?.debounceMillis = d }
                sessions += it
            }

    private fun AnalysisSession.checkAll(files: List<FileRef> = refs()) = check(1, files, rules)

    private val AnalysisSession.plan get() = assertNotNull(lastPlan)

    private fun fullCheck(): BatchResult =
        plainSession(src).also { sessions += it }.check(99, refs(), rules)

    private fun assertSameAsFull(got: BatchResult, want: BatchResult = fullCheck()) {
        for (f in kotlinFilesIn(src)) assertEquals(want.verdictOf(f), got.verdictOf(f), f)
        assertEquals(want.errorFiles, got.errorFiles)
        assertEquals(want.succeeded, got.succeeded)
        assertEquals(want.findings.size, got.findings.size)
        assertEquals(want.firstCompilerError == null, got.firstCompilerError == null)
    }

    /** A cold session writes its bundle, which a restart then finds. Returns the compile key. */
    private fun persistAColdCheck(): String {
        val first = session()
        first.checkAll()
        first.flushPersistence()
        first.dispose()
        val bundles = root.listFiles()!!.filter { it.isDirectory }
        assertEquals(1, bundles.size, "one bundle: ${root.list()?.toList()}")
        return bundles.single().name.removePrefix("v${BundleCodec.VERSION}-")
    }

    private fun bundleFile(key: String) = File(root, "v${BundleCodec.VERSION}-$key/bundle.bin")

    private fun assertColdFullCheckAfter(reason: BundleRejected.Reason?) {
        val restarted = session()
        val result = restarted.checkAll()
        val load = restarted.persistence!!.lastLoad
        if (reason != null) assertEquals(reason, (load as BundleLoad.Miss).reason, "load: $load")
        with(restarted.plan) {
            assertEquals(PlanPath.FULL, path)
            assertEquals(FullReason.COLD, this.reason)
        }
        assertSameAsFull(result)
        // The cold check replaces the bad bundle with a good one for the next restart.
        restarted.flushPersistence()
        val next = session()
        next.checkAll()
        assertTrue(next.persistence!!.lastLoad is BundleLoad.Loaded, "the repaired bundle loads: ${next.persistence!!.lastLoad}")
    }

    // ---- the round trip ---------------------------------------------------------------------------------------------

    @Test fun aRestartedSessionServesTheFirstCheckAfterEditsIncrementally() {
        probeModule()
        persistAColdCheck()

        // Edits made while nothing was running.
        write("Use.kt", "package p\n\nfun u() { protocolProbe(); base(); base() }\n")
        write("Fresh.kt", "package p\n\nfun fresh() { protocolProbe() }\n")
        File(src, "Third.kt").delete()

        val restarted = session()
        assertNull(restarted.incrementalState.compileKey, "the new session starts cold")
        val result = restarted.checkAll()
        assertTrue(restarted.persistence!!.lastLoad is BundleLoad.Loaded)
        with(restarted.plan) {
            assertEquals(PlanPath.INCREMENTAL, path)
            assertNull(reason)
            assertEquals(1, changedFiles); assertEquals(1, newFiles); assertEquals(1, deletedFiles)
        }
        assertSameAsFull(result)
        assertEquals(restarted.incrementalState.generation, restarted.verdicts.generation)
        assertNotNull(restarted.incrementalState.snapshot)
    }

    @Test fun theLoadedStateEqualsTheStateThatWasPersisted() {
        probeModule()
        write("Gone.kt", "package p\n\nfun gone(): Int = 3\n")
        val first = session()
        first.checkAll()
        // Slice merges leave state behind that a full check never has: a deleted file's binary (removed) and a file
        // whose binary no longer describes it (binaryStale). The image must carry both.
        File(src, "Gone.kt").delete()
        write("Api.kt", "package p\n\nfun protocolProbe() {}\nfun base(): Int = 1\nfun extra(): Int = 2\n")
        first.checkAll()
        assertEquals(PlanPath.INCREMENTAL, first.plan.path)
        val written = first.incrementalState.exportImage()!!
        assertTrue(written.snapshot.removed.isNotEmpty() && written.snapshot.binaryStale.isNotEmpty(), "precondition: ${written.snapshot.removed} ${written.snapshot.binaryStale}")
        val declaring = first.incrementalState.filesDeclaring(Sym("p", "base"))
        val declared = first.incrementalState.packageDeclared("p")
        val verdictsWritten = first.verdicts.files().associateWith { first.verdicts.get(it)!! }
        first.flushPersistence()
        first.dispose()

        val restarted = session()
        restarted.checkAll() // loads, then serves
        val loaded = restarted.incrementalState.exportImage()!!
        assertEquals(written.generation, loaded.generation)
        assertEquals(written.surfaces, loaded.surfaces)
        assertEquals(written.contentHashes, loaded.contentHashes)
        assertEquals(written.classFacts, loaded.classFacts)
        assertEquals(written.lookups, loaded.lookups)
        assertEquals(written.snapshot.outputs, loaded.snapshot.outputs)
        assertEquals(written.snapshot.surfaceHashes, loaded.snapshot.surfaceHashes)
        assertEquals(written.snapshot.binaryStale, loaded.snapshot.binaryStale)
        assertEquals(written.snapshot.removed, loaded.snapshot.removed)
        assertEquals(written.verdictKey, loaded.verdictKey)
        for ((file, entry) in verdictsWritten) {
            val got = restarted.verdicts.get(file)!!
            assertEquals(entry.contentHash, got.contentHash)
            assertEquals(entry.verdictKey, got.verdictKey)
            assertEquals(entry.verdict.findings, got.verdict.findings)
        }
        // The declaration indexes were rebuilt from the surfaces.
        val api = canon(File(src, "Api.kt").path)
        assertEquals(declaring, restarted.incrementalState.filesDeclaring(Sym("p", "base")))
        assertEquals(setOf(api), restarted.incrementalState.filesDeclaring(Sym("p", "base")))
        assertEquals(declared, restarted.incrementalState.packageDeclared("p"))
    }

    @Test fun theWorkingSnapshotOfARestartedSessionIsItsOwnCopy() {
        probeModule()
        val key = persistAColdCheck()
        val restarted = session()
        restarted.checkAll()
        val working = restarted.incrementalState.snapshot!!
        val persisted = File(root, "v${BundleCodec.VERSION}-$key").listFiles()!!.single { it.name.startsWith("classes-") }
        assertTrue(working.dir.canonicalPath != persisted.canonicalPath && !working.root.canonicalPath.startsWith(root.canonicalPath))
        // Slices park files in the working snapshot; the persisted classes are never touched.
        val before = filesUnder(persisted)
        write("Use.kt", "package p\n\nfun u() { protocolProbe() }\n")
        restarted.checkAll()
        restarted.flushPersistence()
        restarted.dispose()
        assertEquals(before, filesUnder(persisted))
        assertTrue(persisted.isDirectory, "disposing the session deletes its working snapshot, not the bundle's")
        assertFalse(working.root.exists())
    }

    @Test fun editsAfterTheLastWriteAreFoundByContentHash() {
        probeModule()
        val first = session(debounce = 60_000)
        first.checkAll()
        first.flushPersistence()
        // A slice merge the debounce has not written yet; the process "dies" without flushing.
        write("Use.kt", "package p\n\nfun u() { protocolProbe(); base(); base() }\n")
        first.checkAll()
        val writes = first.persistence!!.writes
        sessions.remove(first) // not disposed: a crash
        write("Other.kt", "package p\n\nfun o() { protocolProbe(); protocolProbe() }\n")

        val restarted = session()
        val result = restarted.checkAll()
        assertTrue(restarted.persistence!!.lastLoad is BundleLoad.Loaded)
        assertEquals(PlanPath.INCREMENTAL, restarted.plan.path)
        assertEquals(2, restarted.plan.changedFiles, "both edits are newer than the bundle")
        assertEquals(1, writes)
        assertSameAsFull(result)
        first.dispose()
    }

    // ---- error paths -------------------------------------------------------------------------------------------------

    @Test fun aCorruptBundleFallsBackToAColdFullCheck() {
        probeModule()
        val key = persistAColdCheck()
        val file = bundleFile(key)
        val bytes = file.readBytes()
        bytes[bytes.size / 2] = (bytes[bytes.size / 2] + 1).toByte()
        file.writeBytes(bytes)
        assertColdFullCheckAfter(BundleRejected.Reason.CORRUPT)
    }

    @Test fun aTruncatedBundleFallsBackToAColdFullCheck() {
        probeModule()
        val key = persistAColdCheck()
        val file = bundleFile(key)
        file.writeBytes(file.readBytes().copyOf(file.length().toInt() / 3))
        assertColdFullCheckAfter(BundleRejected.Reason.TRUNCATED)
    }

    @Test fun anEmptyOrGarbageBundleFallsBackToAColdFullCheck() {
        probeModule()
        val key = persistAColdCheck()
        bundleFile(key).writeBytes(ByteArray(0))
        assertColdFullCheckAfter(null)
        bundleFile(key).writeText("this is not a bundle at all, just text".repeat(10))
        assertColdFullCheckAfter(null)
    }

    @Test fun aBundleOfAnotherFormatVersionFallsBackToAColdFullCheck() {
        probeModule()
        val key = persistAColdCheck()
        val file = bundleFile(key)
        val bytes = file.readBytes()
        bytes[7] = (bytes[7] + 1).toByte()
        file.writeBytes(bytes)
        assertColdFullCheckAfter(BundleRejected.Reason.VERSION)
    }

    @Test fun aBundleFiledUnderAnotherCompileKeyIsNotUsed() {
        probeModule()
        val key = persistAColdCheck()
        // The same bytes under the key a changed classpath would give.
        val changedClasspath = listOf(testStdlib, File(tmp.toFile(), "extra.jar").path)
        val other = session(classpath = changedClasspath)
        val otherKey = other.compileKeyNow()
        assertTrue(otherKey != key)
        File(root, "v${BundleCodec.VERSION}-$key").copyRecursively(File(root, "v${BundleCodec.VERSION}-$otherKey"))
        val result = other.checkAll()
        assertEquals(BundleRejected.Reason.COMPILE_KEY, ((other.persistence!!.lastLoad) as BundleLoad.Miss).reason)
        assertEquals(PlanPath.FULL, other.plan.path)
        assertEquals(FullReason.COLD, other.plan.reason)
        assertSameAsFull(result)
    }

    @Test fun aChangedClasspathHasNoBundleAndChecksCold() {
        probeModule()
        persistAColdCheck()
        val other = session(classpath = listOf(testStdlib, File(tmp.toFile(), "extra.jar").path))
        val result = other.checkAll()
        assertEquals(BundleRejected.Reason.MISSING, ((other.persistence!!.lastLoad) as BundleLoad.Miss).reason)
        assertEquals(PlanPath.FULL, other.plan.path)
        assertSameAsFull(result)
    }

    @Test fun aMissingSnapshotDirectoryFallsBackToAColdFullCheck() {
        probeModule()
        val key = persistAColdCheck()
        File(root, "v${BundleCodec.VERSION}-$key").listFiles()!!.filter { it.name.startsWith("classes-") }.forEach { it.deleteRecursively() }
        assertColdFullCheckAfter(BundleRejected.Reason.SNAPSHOT_MISSING)
    }

    @Test fun aSnapshotDirectoryMissingAClassFileFallsBackToAColdFullCheck() {
        probeModule()
        val key = persistAColdCheck()
        val classes = File(root, "v${BundleCodec.VERSION}-$key").listFiles()!!.single { it.name.startsWith("classes-") }
        classes.walkTopDown().first { it.isFile && it.extension == "class" }.delete()
        assertColdFullCheckAfter(BundleRejected.Reason.SNAPSHOT_MISSING)
    }

    @Test fun aBundleWhosePathsEscapeTheSnapshotIsRejected() {
        probeModule()
        val key = persistAColdCheck()
        val file = bundleFile(key)
        val decoded = BundleCodec.decode(file.readBytes(), key)
        val evil = PersistedState(
            decoded.state.compileKey, decoded.state.verdictKey, decoded.state.generation, decoded.state.surfaces,
            decoded.state.contentHashes, decoded.state.classFacts, decoded.state.lookups,
            PersistedSnapshot(decoded.state.snapshot.outputs.mapValues { listOf("../../../etc/passwd") }, decoded.state.snapshot.surfaceHashes, decoded.state.snapshot.binaryStale, decoded.state.snapshot.removed),
        )
        file.writeBytes(BundleCodec.encode(key, decoded.snapshotId, evil, decoded.verdicts, decoded.verdictGeneration))
        assertColdFullCheckAfter(BundleRejected.Reason.SNAPSHOT_MISSING)
    }

    @Test fun anOversizedBundleFileIsAColdCheckNotAnError() {
        probeModule()
        val key = persistAColdCheck()
        val file = bundleFile(key)
        file.delete()
        // Sparse, so it costs nothing on disk; reading it whole would be an OutOfMemoryError out of `check`.
        java.io.RandomAccessFile(file, "rw").use { it.setLength(3L * 1024 * 1024 * 1024) }
        assertColdFullCheckAfter(BundleRejected.Reason.IO)
    }

    @Test fun aBundleNamingFilesOutsideTheSourcesIsRejected() {
        probeModule()
        val key = persistAColdCheck()
        val file = bundleFile(key)
        val decoded = BundleCodec.decode(file.readBytes(), key)
        val st = decoded.state
        val sn = st.snapshot
        fun rewrite(state: PersistedState) = file.writeBytes(BundleCodec.encode(key, decoded.snapshotId, state, decoded.verdicts, decoded.verdictGeneration))
        fun variant(
            hashes: Map<String, String> = st.contentHashes, outputs: Map<String, List<String>> = sn.outputs,
            removed: Set<String> = sn.removed, stale: Set<String> = sn.binaryStale,
        ) = PersistedState(st.compileKey, st.verdictKey, st.generation, st.surfaces, hashes, st.classFacts, st.lookups,
            PersistedSnapshot(outputs, sn.surfaceHashes, stale, removed))
        val outside = tmp.resolve("elsewhere/Evil.kt").toString()
        for (evil in listOf(
            variant(hashes = st.contentHashes + (outside to "h")),
            variant(outputs = sn.outputs + (outside to emptyList())),
            variant(removed = setOf(outside)),
            variant(stale = setOf("/etc/passwd")),
            variant(removed = setOf("${src.path}/../elsewhere/Evil.kt")),
        )) {
            rewrite(evil)
            assertColdFullCheckAfter(BundleRejected.Reason.STATE_MISMATCH)
        }
    }

    @Test fun aVerdictForAFileOutsideTheStateAndTheRootsIsRejected() {
        probeModule()
        val key = persistAColdCheck()
        val file = bundleFile(key)
        val decoded = BundleCodec.decode(file.readBytes(), key)
        val real = decoded.verdicts.first()
        val outside = tmp.resolve("elsewhere/Evil.kt").toString()
        val forged = decoded.verdicts + PersistedVerdict(outside, real.entry)
        file.writeBytes(BundleCodec.encode(key, decoded.snapshotId, decoded.state, forged, decoded.verdictGeneration))
        assertColdFullCheckAfter(BundleRejected.Reason.STATE_MISMATCH)
        // A finding whose path is outside is rejected the same way.
        val f = real.entry.verdict.findings.firstOrNull() ?: dev.jasonpearson.krit.fir.runner.Finding(real.file, 1, 1, 0, 0, "R", "warning", "m", 1.0)
        val moved = VerdictCache.Entry(real.entry.contentHash, real.entry.suppliedHash, real.entry.verdictKey,
            CachedVerdict(listOf(dev.jasonpearson.krit.fir.runner.Finding(outside, f.line, f.col, f.startByte, f.endByte, f.rule, f.severity, f.message, f.confidence)), null, emptyMap()))
        file.writeBytes(BundleCodec.encode(key, decoded.snapshotId, decoded.state, decoded.verdicts.drop(1) + PersistedVerdict(real.file, moved), decoded.verdictGeneration))
        assertColdFullCheckAfter(BundleRejected.Reason.STATE_MISMATCH)
    }

    @Test fun aBundleDirectoryThatIsALinkOnReadIsAColdCheck() {
        probeModule()
        val key = persistAColdCheck()
        val dir = File(root, "v${BundleCodec.VERSION}-$key")
        val moved = File(root, "moved-away")
        assertTrue(dir.renameTo(moved))
        java.nio.file.Files.createSymbolicLink(dir.toPath(), moved.toPath())
        val restarted = session()
        restarted.checkAll()
        assertTrue((restarted.persistence!!.lastLoad as BundleLoad.Miss).reason == BundleRejected.Reason.IO, "${restarted.persistence!!.lastLoad}")
        assertEquals(FullReason.COLD, restarted.plan.reason)
        restarted.dispose(); sessions.remove(restarted)
        assertTrue(moved.isDirectory)
    }

    @Test fun verdictsOutOfStepWithTheStateAreNotTrusted() {
        probeModule()
        val key = persistAColdCheck()
        val file = bundleFile(key)
        val decoded = BundleCodec.decode(file.readBytes(), key)
        file.writeBytes(BundleCodec.encode(key, decoded.snapshotId, decoded.state, decoded.verdicts, decoded.verdictGeneration + 1))
        assertColdFullCheckAfter(BundleRejected.Reason.STATE_MISMATCH)
    }

    // ---- no root, --no-cache ---------------------------------------------------------------------------------------------

    @Test fun withoutARootNothingIsReadOrWritten() {
        probeModule()
        val s = session(persistRoot = null)
        assertNull(s.persistence)
        s.checkAll()
        write("Use.kt", "package p\n\nfun u() { protocolProbe() }\n")
        s.checkAll()
        s.flushPersistence()
        s.dispose()
        assertFalse(root.exists(), "no bundle directory is created")
    }

    @Test fun noCacheNeitherReadsNorWritesTheBundle() {
        probeModule()
        val key = persistAColdCheck()
        val file = bundleFile(key)
        val before = file.readBytes()
        val stamp = file.lastModified()
        val dirListing = root.walkTopDown().map { it.relativeTo(root).path + ":" + it.lastModified() }.toList().sorted()

        // --no-cache: the request carries no incrementalDir, i.e. configurePersistence(null).
        val bypass = session(persistRoot = null)
        val result = bypass.checkAll()
        assertEquals(PlanPath.FULL, bypass.plan.path)
        assertEquals(FullReason.COLD, bypass.plan.reason, "nothing was read")
        assertSameAsFull(result)
        write("Use.kt", "package p\n\nfun u() { protocolProbe(); protocolProbe() }\n")
        bypass.checkAll()
        bypass.flushPersistence()
        bypass.dispose()
        assertTrue(before.contentEquals(file.readBytes()), "nothing was written")
        assertEquals(stamp, file.lastModified())
        assertEquals(dirListing, root.walkTopDown().map { it.relativeTo(root).path + ":" + it.lastModified() }.toList().sorted())
    }

    @Test fun turningPersistenceOffStopsAWriteThatIsAlreadyInFlight() {
        probeModule()
        val s = session()
        val persistence = s.persistence!!
        val inFlight = java.util.concurrent.CountDownLatch(1)
        val release = java.util.concurrent.CountDownLatch(1)
        persistence.testHookBeforePublish = { inFlight.countDown(); release.await(30, java.util.concurrent.TimeUnit.SECONDS) }
        s.checkAll() // schedules the write of the full check
        assertTrue(inFlight.await(30, java.util.concurrent.TimeUnit.SECONDS), "the write reached the publish step")
        s.configurePersistence(null) // --no-cache on the next request, while the writer is between encoding and publishing
        release.countDown()
        persistence.flush() // waits for the writer
        assertEquals(0, persistence.writes)
        assertFalse(root.walkTopDown().any { it.name == "bundle.bin" || it.name.startsWith("classes-") || it.name.startsWith(".stage-") || it.name.startsWith(".bundle-") }, "nothing was published: ${root.walkTopDown().toList()}")
    }

    @Test fun aCheckNeverWaitsForABundleWriteAndATornClassesCopyIsRetried() {
        probeModule()
        val s = session(debounce = 10)
        val persistence = s.persistence!!
        val ran = java.util.concurrent.atomic.AtomicInteger()
        var checkFinished = false
        persistence.testHookAfterCopy = {
            if (ran.incrementAndGet() == 1) {
                // The writer stands between copying the classes and checking the copy, with the snapshot lock not held:
                // a check that slices (parking files of the working snapshot) must not wait for it.
                write("Use.kt", "package p\n\nfun u() { protocolProbe(); base(); base(); base() }\n")
                val t = Thread { s.checkAll() }.also { it.start() }
                t.join(60_000)
                checkFinished = !t.isAlive
            }
        }
        s.checkAll()
        val deadline = System.nanoTime() + 60_000_000_000L
        while (persistence.writes == 0 && System.nanoTime() < deadline) Thread.sleep(25)
        assertTrue(checkFinished, "the check waited for the writer")
        assertTrue(ran.get() >= 2, "the first copy overlapped the slice and was thrown away, a second one was made (${ran.get()})")
        assertEquals(1, persistence.writes)
        s.flushPersistence()
        s.dispose(); sessions.remove(s)

        val restarted = session()
        val result = restarted.checkAll()
        assertTrue(restarted.persistence!!.lastLoad is BundleLoad.Loaded, "${restarted.persistence!!.lastLoad}")
        assertSameAsFull(result)
    }

    @Test fun turningPersistenceOffOnALiveSessionDropsItsPendingWrite() {
        probeModule()
        val s = session(debounce = 60_000)
        s.checkAll()
        s.flushPersistence()
        val key = root.listFiles()!!.single { it.isDirectory }.name.removePrefix("v${BundleCodec.VERSION}-")
        val stamp = bundleFile(key).lastModified()
        val bytes = bundleFile(key).readBytes()
        write("Use.kt", "package p\n\nfun u() { protocolProbe(); protocolProbe() }\n")
        s.checkAll() // schedules a debounced write
        s.configurePersistence(null) // --no-cache on the next request
        s.dispose()
        assertTrue(bytes.contentEquals(bundleFile(key).readBytes()))
        assertEquals(stamp, bundleFile(key).lastModified())
    }

    // ---- write timing ---------------------------------------------------------------------------------------------------

    @Test fun aSliceMergeIsNotWrittenOnTheCriticalPathAndIsWrittenOnFlush() {
        probeModule()
        val s = session(debounce = 60_000)
        s.checkAll()
        s.flushPersistence()
        val key = root.listFiles()!!.single { it.isDirectory }.name.removePrefix("v${BundleCodec.VERSION}-")
        val generation = BundleCodec.decode(bundleFile(key).readBytes(), key).state.generation
        val stamp = bundleFile(key).lastModified()

        write("Use.kt", "package p\n\nfun u() { protocolProbe(); base(); base() }\n")
        s.checkAll()
        assertEquals(PlanPath.INCREMENTAL, s.plan.path)
        assertEquals(stamp, bundleFile(key).lastModified(), "the check returned without touching the bundle")
        assertEquals(generation, BundleCodec.decode(bundleFile(key).readBytes(), key).state.generation)

        s.flushPersistence()
        val after = BundleCodec.decode(bundleFile(key).readBytes(), key)
        assertEquals(s.incrementalState.generation, after.state.generation)
        assertTrue(after.state.generation > generation)
        // Only the state and verdicts were rewritten: the snapshot's classes were not copied again.
        assertEquals(1, File(root, "v${BundleCodec.VERSION}-$key").list()!!.count { it.startsWith("classes-") })
    }

    @Test fun theDebouncedWriteHappensOnItsOwnThread() {
        probeModule()
        val s = session(debounce = 100)
        s.checkAll()
        write("Use.kt", "package p\n\nfun u() { protocolProbe() }\n")
        s.checkAll()
        val key = s.compileKeyNow()
        val want = s.incrementalState.generation
        // The check returned; the bundle catches up with the merge without anyone calling flush. (A merge that follows
        // the full check closely may replace the full check's pending write: one write then covers both.)
        val deadline = System.nanoTime() + 20_000_000_000L
        fun written(): Int = try { BundleCodec.decode(bundleFile(key).readBytes(), key).state.generation } catch (_: Exception) { -1 }
        while (written() != want && System.nanoTime() < deadline) Thread.sleep(50)
        assertEquals(want, written(), "the slice merge reached the bundle (writes=${s.persistence!!.writes})")
    }

    @Test fun aFullCheckAfterANewSnapshotRewritesTheClassesOnce() {
        probeModule()
        val s = session()
        s.checkAll()
        s.flushPersistence()
        val key = root.listFiles()!!.single { it.isDirectory }.name.removePrefix("v${BundleCodec.VERSION}-")
        val first = File(root, "v${BundleCodec.VERSION}-$key").list()!!.single { it.startsWith("classes-") }
        // A new snapshot: the planner's resnapshot or a state reset ends in a full check.
        s.resetIncremental()
        s.checkAll()
        s.flushPersistence()
        val names = File(root, "v${BundleCodec.VERSION}-$key").list()!!.filter { it.startsWith("classes-") }
        assertEquals(2, names.size, "the previous classes stay for readers of the previous bundle")
        assertTrue(first in names)
        assertEquals(names.single { it != first }.removePrefix("classes-"), BundleCodec.decode(bundleFile(key).readBytes(), key).snapshotId)
    }

    // ---- eviction and concurrency, through sessions -------------------------------------------------------------------

    @Test fun olderCompileKeysAreEvictedBeyondTheNewestThree() {
        probeModule()
        for (i in 1..5) {
            val s = session(classpath = listOf(testStdlib, File(tmp.toFile(), "extra$i.jar").path))
            s.checkAll()
            s.flushPersistence()
            s.dispose()
            val bundle = root.listFiles()!!.filter { it.isDirectory }.maxByOrNull { it.lastModified() }!!
            File(bundle, "bundle.bin").setLastModified(1_000_000L + i * 60_000L)
        }
        assertEquals(3, root.listFiles()!!.count { it.isDirectory })
    }

    @Test fun twoSessionsWritingTheSameBundleLeaveItReadable() {
        probeModule()
        val a = session(debounce = 10)
        val b = session(debounce = 10)
        val ta = Thread { a.checkAll(); a.flushPersistence() }
        val tb = Thread { b.checkAll(); b.flushPersistence() }
        ta.start(); tb.start(); ta.join(); tb.join()
        a.dispose(); b.dispose()
        val bundles = root.listFiles()!!.filter { it.isDirectory }
        assertEquals(1, bundles.size)
        val restarted = session()
        val result = restarted.checkAll()
        assertTrue(restarted.persistence!!.lastLoad is BundleLoad.Loaded, "${restarted.persistence!!.lastLoad}")
        assertEquals(PlanPath.INCREMENTAL, restarted.plan.path)
        assertSameAsFull(result)
    }

    @Test fun aSessionWithoutThePlannerIgnoresARoot() {
        probeModule()
        val s = AnalysisSession(listOf(src.path), listOf(testStdlib), incrementalRecording = true, incrementalPlanning = false)
            .also { sessions += it; it.configurePersistence(root) }
        assertNull(s.persistence)
        s.checkAll()
        assertFalse(root.exists())
    }
}
