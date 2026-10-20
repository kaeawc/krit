package dev.jasonpearson.krit.fir.runner

import dev.jasonpearson.krit.fir.incremental.BundleCodec
import dev.jasonpearson.krit.fir.incremental.BundleRejected
import dev.jasonpearson.krit.fir.incremental.BundleStore
import dev.jasonpearson.krit.fir.incremental.PersistedState
import dev.jasonpearson.krit.fir.incremental.PersistedVerdict
import dev.jasonpearson.krit.fir.incremental.Snapshot
import dev.jasonpearson.krit.fir.incremental.SnapshotStorage
import dev.jasonpearson.krit.fir.isIsolatable
import java.io.File
import java.util.concurrent.Executors
import java.util.concurrent.ScheduledExecutorService
import java.util.concurrent.ScheduledFuture
import java.util.concurrent.TimeUnit

/** What loading a persisted bundle into a cold session came to. */
internal sealed interface BundleLoad {
    /** The state, the verdicts and the snapshot of the bundle are the session's now. */
    object Loaded : BundleLoad

    /** Nothing was loaded and the session is as it was: the next check is today's cold full check. */
    class Miss(val reason: BundleRejected.Reason, val detail: String? = null) : BundleLoad
}

/**
 * Keeps an [AnalysisSession]'s incremental state warm across processes: loads the bundle of the session's compile key
 * when the session is cold, and writes the bundle after full checks and (debounced) slice merges. See [BundleStore] for
 * the layout and the publish protocol.
 *
 * **Snapshot lifetime.** The session compiles against its own working snapshot (a directory owned like any other, under
 * the swept `fir-snapshots` area, with the same file-lock liveness and shutdown cleanup). A bundle never shares class
 * files with it: writing copies the working classes into the bundle (once per snapshot, after a full check), and loading
 * copies the bundle's classes into a new working snapshot. A slice compile parks (moves) files inside the working
 * snapshot, so a bundle that shared them would be torn mid-slice and a second process loading it would compile against
 * a half-moved directory; with copies, the persisted classes are immutable from the moment they get their name, and the
 * existing cleanup of working snapshots needs no exception for persisted ones.
 *
 * **Write timing.** A full check schedules a write [FULL_DELAY_MILLIS] after it returns; a slice merge schedules one
 * [debounceMillis] after the last merge, and never later than [MAX_DELAY_MILLIS] after the first unwritten one. All run
 * on one background thread. A write holds the session's snapshot lock only to take an immutable image of the state
 * and the verdicts, and to check afterwards that the working snapshot did not change while its classes were copied;
 * encoding and all file I/O happen outside it, so a check never waits for a write. The copy of the classes (once per
 * snapshot, after a full check) is the one thing that reads the working snapshot without the lock: slices park files
 * in it, so [Snapshot.epoch] is read before and after, and a copy that overlapped a change is thrown away and retried
 * ([MAX_UNLOCKED_COPY_ATTEMPTS] times; after that the next attempt copies under the lock, so a check can then wait for one
 * classes copy, which keeps a busy session from starving its own bundle).
 * [flush] (used by [AnalysisSession.dispose]) writes whatever is pending, synchronously.
 *
 * Nothing here throws into a check: a failed read is a [BundleLoad.Miss], a failed write is logged once and dropped.
 */
internal class BundlePersistence(private val session: AnalysisSession, val store: BundleStore) {
    @Volatile var debounceMillis = DEBOUNCE_MILLIS

    /** The outcome of the load this session attempted; null before one. */
    @Volatile var lastLoad: BundleLoad? = null
        private set

    /** How long the latest load took, read to attached; -1 before one. For telemetry and benches. */
    @Volatile var lastLoadMillis = -1L
        private set

    /** Test seams on the writer thread: after the classes were copied (before the copy is checked), and before publishing. */
    @Volatile internal var testHookAfterCopy: (() -> Unit)? = null
    @Volatile internal var testHookBeforePublish: (() -> Unit)? = null

    /** Bundles published by this session. */
    @Volatile var writes = 0
        private set

    private var loadAttempted = false

    // The working snapshot whose classes are in the bundle under [persistedId]. Guarded by the session's snapshot lock.
    private var persistedFor: Snapshot? = null
    private var persistedId: String? = null

    // Copies of the working snapshot that overlapped a change and were thrown away. Guarded by [writeLock].
    private var tornCopies = 0

    private val lock = Any()
    private var pending: ScheduledFuture<*>? = null
    private var dirtySince = 0L
    @Volatile private var dirty = false
    @Volatile private var closed = false
    private var failureLogged = false

    // ---- load ---------------------------------------------------------------------------------------------------

    /**
     * Loads the bundle of this session's compile key into the session, once. The caller holds the session's snapshot
     * lock and has seen that the session has no state. The planner then runs as it does after a long pause: whatever
     * changed on disk since the bundle was written is found by content hash.
     */
    fun loadOnce(): BundleLoad {
        if (loadAttempted) return lastLoad ?: BundleLoad.Miss(BundleRejected.Reason.MISSING)
        loadAttempted = true
        val t0 = System.nanoTime()
        val outcome = load()
        lastLoadMillis = (System.nanoTime() - t0) / 1_000_000
        lastLoad = outcome
        if (DEBUG) System.err.println("krit-fir: persisted bundle: " + (if (outcome is BundleLoad.Miss) "miss (${outcome.reason} ${outcome.detail.orEmpty()})" else "loaded"))
        return outcome
    }

    private fun load(): BundleLoad {
        val state = session.incrementalState
        var working: Snapshot? = null
        var attached = false
        try {
            val key = session.compileKeyNow()
            val bundle = store.read(key)
            if (bundle.snapshotId.isEmpty()) throw BundleRejected(BundleRejected.Reason.SNAPSHOT_MISSING, "bundle has no snapshot")
            // A bundle is a file in the project tree: a path in it that is not one of this session's sources is not ours.
            // <repo>/.krit/fir-incremental names its repository; any other root trusts only the source directories.
            val repoDir = store.root.parentFile?.takeIf { store.root.name == "fir-incremental" && it.name == ".krit" }?.parentFile
            session.pathOutsideSources(bundle.state, bundle.verdicts, repoDir)?.let {
                throw BundleRejected(BundleRejected.Reason.STATE_MISMATCH, "the bundle names a file outside the source directories and the repository: $it")
            }
            // The state and the verdicts are one file and were written together; this guards a writer bug, not a race.
            if (bundle.verdictGeneration != bundle.state.generation) {
                throw BundleRejected(BundleRejected.Reason.STATE_MISMATCH, "verdicts at generation ${bundle.verdictGeneration}, state at ${bundle.state.generation}")
            }
            val classes = store.readableClassesDir(key, bundle.snapshotId)
                ?: throw BundleRejected(BundleRejected.Reason.SNAPSHOT_MISSING, "no class directory for ${bundle.snapshotId}")
            working = Snapshot(SnapshotStorage.newSnapshotRoot(session.snapshotRootOverride))
            try {
                BundleStore.copyTree(classes, working.dir)
            } catch (e: java.io.IOException) {
                throw BundleRejected(BundleRejected.Reason.SNAPSHOT_MISSING, "copying the classes failed: ${e.message}")
            }
            if (!working.restore(bundle.state.snapshot.outputs)) {
                throw BundleRejected(BundleRejected.Reason.SNAPSHOT_MISSING, "the class files do not match the state")
            }
            if (!state.restoreImage(bundle.state, working)) {
                throw BundleRejected(BundleRejected.Reason.STATE_MISMATCH, "the state is not one a commit produces")
            }
            attached = true
            val verdicts = session.verdicts
            verdicts.clear()
            for (v in bundle.verdicts) verdicts.put(v.file, v.entry)
            verdicts.sync(state.generation)
            persistedFor = working
            persistedId = bundle.snapshotId
            return BundleLoad.Loaded
        } catch (r: BundleRejected) {
            discard(working, attached)
            return BundleLoad.Miss(r.reason, r.message)
        } catch (_: OutOfMemoryError) {
            // A bundle is untrusted input: running out of heap on it is a miss, never a failed check.
            discard(working, attached)
            return BundleLoad.Miss(BundleRejected.Reason.IO, "out of memory loading the bundle")
        } catch (e: Throwable) {
            if (!isIsolatable(e)) throw e
            discard(working, attached)
            return BundleLoad.Miss(BundleRejected.Reason.IO, "${e.javaClass.simpleName}: ${e.message}")
        }
    }

    // A half-loaded session is a cold one: the state (and with it the working snapshot it owns) is reset.
    private fun discard(working: Snapshot?, attached: Boolean) {
        session.incrementalState.reset()
        session.verdicts.clear()
        persistedFor = null; persistedId = null
        if (working != null) try { working.delete() } catch (_: Exception) {}
    }

    // ---- write --------------------------------------------------------------------------------------------------

    /** The session's state moved; write it [FULL_DELAY_MILLIS] from now (after a full check) or debounced (slice merge). */
    fun markDirty(afterFullCheck: Boolean) {
        if (closed) return
        synchronized(lock) {
            val now = System.currentTimeMillis()
            if (!dirty) { dirty = true; dirtySince = now }
            val delay = if (afterFullCheck) FULL_DELAY_MILLIS else minOf(debounceMillis, maxOf(0L, dirtySince + MAX_DELAY_MILLIS - now))
            pending?.cancel(false)
            pending = scheduler.schedule({ runPending() }, delay, TimeUnit.MILLISECONDS)
        }
    }

    /** Writes the pending state now, on the calling thread. */
    fun flush() {
        synchronized(lock) { pending?.cancel(false); pending = null }
        runPending()
    }

    /** Flushes and stops: later [markDirty] calls do nothing. */
    fun close() {
        closed = true
        flush()
    }

    /**
     * Stops without writing what is pending: the caller no longer wants this root written (`--no-cache`). A write that
     * is in flight is stopped too: when this returns, nothing more is published (a publish that was already
     * committing is waited for; it finishes before this returns).
     */
    fun discard() {
        closed = true
        synchronized(commitGate) { discarded = true }
        synchronized(lock) { pending?.cancel(false); pending = null; dirty = false }
    }

    private val writeLock = Any()

    // Held by [BundleStore.publish] while it commits, and by [discard] while it flips [discarded].
    private val commitGate = Any()
    @Volatile private var discarded = false

    private fun runPending() {
        synchronized(writeLock) {
            if (!dirty) return
            dirty = false
            try {
                if (!writeBundle()) return
            } catch (e: Throwable) {
                if (!isIsolatable(e)) throw e
                logFailure(e)
            }
        }
    }

    private fun logFailure(e: Throwable) {
        if (failureLogged) return
        failureLogged = true
        System.err.println("krit-fir: persisting the incremental bundle failed (${e.javaClass.name}: ${e.message})")
    }

    /** Encodes the session's current state and publishes it; false when there was nothing to write. */
    private fun writeBundle(): Boolean {
        if (session.persistence !== this || discarded) return false
        var stage: File? = null
        try {
            val key: String
            val id: String
            val image: PersistedState
            val entries: List<PersistedVerdict>
            val verdictGeneration: Int
            val snapshot: Snapshot
            var copyEpoch = -1L // >= 0: the classes still have to be copied, and this is the snapshot's epoch at the image
            synchronized(session.snapshotLock) {
                val state = session.incrementalState
                image = state.exportImage() ?: return false
                key = session.compileKeyNow()
                if (image.compileKey != key) return false
                val verdicts = session.verdicts
                // A cache that fell out of step with the state is not trusted by the planner, so it is not saved.
                if (verdicts.generation != state.generation) return false
                snapshot = state.snapshot ?: return false
                entries = verdicts.files().mapNotNull { f -> verdicts.get(f)?.let { PersistedVerdict(f, it) } }
                verdictGeneration = verdicts.generation
                if (persistedFor === snapshot && persistedId != null) {
                    id = persistedId!!
                } else {
                    id = BundleCodec.newSnapshotId()
                    if (tornCopies >= MAX_UNLOCKED_COPY_ATTEMPTS) {
                        val staged = store.newStage(key)
                        stage = staged // before the copy, so that a copy that throws is cleaned up
                        BundleStore.copyTree(snapshot.dir, staged)
                    } else {
                        copyEpoch = snapshot.epoch
                    }
                }
            }
            if (copyEpoch >= 0) {
                val staged = store.newStage(key).also { stage = it }
                // The working snapshot is not locked: if a slice parks files meanwhile the copy is torn (or throws).
                val failure = try { BundleStore.copyTree(snapshot.dir, staged); null } catch (e: java.io.IOException) { e }
                testHookAfterCopy?.invoke()
                val stable = synchronized(session.snapshotLock) {
                    session.incrementalState.snapshot === snapshot && snapshot.epoch == copyEpoch && !snapshot.corrupt
                }
                if (!stable) {
                    tornCopies++
                    if (!discarded) markDirty(afterFullCheck = false)
                    return false
                }
                if (failure != null) throw failure
            }
            tornCopies = 0
            val bytes = BundleCodec.encode(key, id, image, entries, verdictGeneration)
            testHookBeforePublish?.invoke()
            val published = store.publish(key, bytes, id, stage, commitGate) { !discarded }
            if (published) {
                synchronized(session.snapshotLock) { persistedFor = snapshot; persistedId = id }
                writes++
            } else if (!discarded) {
                // The lock was busy, or the classes of a reused id are gone: stage them afresh next time.
                if (stage == null) synchronized(session.snapshotLock) { persistedFor = null; persistedId = null }
                markDirty(afterFullCheck = false)
            }
            return published
        } finally {
            stage?.takeIf { it.exists() }?.let { BundleStore.deleteTree(it.toPath(), it.parentFile.toPath()) }
        }
    }

    companion object {
        const val DEBOUNCE_MILLIS = 2_000L
        const val MAX_DELAY_MILLIS = 30_000L
        const val FULL_DELAY_MILLIS = 50L
        const val MAX_UNLOCKED_COPY_ATTEMPTS = 2
        private val DEBUG = System.getenv("KRIT_FIR_INCREMENTAL_DEBUG") == "1"

        // One daemon thread for every session's writes; a session's writes are serialised by its own lock.
        private val scheduler: ScheduledExecutorService by lazy {
            Executors.newSingleThreadScheduledExecutor { r -> Thread(r, "krit-fir-bundle-writer").also { it.isDaemon = true } }
        }
    }
}
