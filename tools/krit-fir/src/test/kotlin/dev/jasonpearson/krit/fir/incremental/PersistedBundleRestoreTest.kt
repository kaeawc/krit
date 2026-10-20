package dev.jasonpearson.krit.fir.incremental

import dev.jasonpearson.krit.fir.runner.AnalysisSession
import dev.jasonpearson.krit.fir.runner.BundleLoad
import dev.jasonpearson.krit.fir.runner.FileRef
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.io.File
import java.nio.file.Path
import kotlin.test.assertEquals
import kotlin.test.assertNotNull
import kotlin.test.assertTrue

/**
 * What a restored state must remember (#878): persist (optionally after a slice merge that leaves `removed` or
 * `binaryStale` behind), change sources while nothing runs, restart, and compare the first check with a fresh full
 * check of the same sources, file by file.
 *
 * The slice-merge scenarios are the ones that need a field of the persisted image to survive: without `removed` the
 * restarted session lets the binary of a deleted file answer for it (a missed unresolved reference); without
 * `binaryStale` it trusts a stale binary of a file whose surface changed.
 */
class PersistedBundleRestoreTest {
    @TempDir lateinit var tmp: Path

    private val src get() = tmp.resolve("src").toFile()
    private val snaps get() = tmp.resolve("snapshots").toFile()
    private val root get() = tmp.resolve("fir-incremental").toFile()
    private val rules = setOf("ProtocolProbe")
    private val sessions = ArrayList<AnalysisSession>()

    @AfterEach fun cleanUp() { sessions.forEach { it.dispose() } }

    private fun w(rel: String, text: String) = src.writeSource(rel, text)
    private fun refs() = kotlinFilesIn(src).map { FileRef(it) }
    private fun session(): AnalysisSession =
        AnalysisSession(listOf(src.path), listOf(testStdlib), incrementalRecording = true, incrementalPlanning = true).also {
            it.snapshotRootOverride = snaps; it.configurePersistence(root); sessions += it
        }

    /**
     * [setup] writes the module; the first session checks it and [between] may change it and check again (a slice
     * merge); the session is persisted and ended; [whileDown] changes the sources; a new session checks them.
     */
    private fun scenario(
        setup: () -> Unit, whileDown: () -> Unit, between: (AnalysisSession) -> Unit = {},
        expect: (AnalysisSession) -> Unit = {},
    ) {
        setup()
        val first = session()
        first.check(1, refs(), rules)
        between(first)
        first.flushPersistence(); first.dispose(); sessions.remove(first)
        whileDown()
        val restarted = session()
        val got = restarted.check(2, refs(), rules)
        val load = restarted.persistence!!.lastLoad
        assertNotNull(restarted.lastPlan)
        val want = plainSession(src).also { sessions += it }.check(3, refs(), rules)
        val diffs = kotlinFilesIn(src).filter { want.verdictOf(it) != got.verdictOf(it) }.map { "$it want=${want.verdictOf(it)} got=${got.verdictOf(it)}" }
        assertTrue(load is BundleLoad.Loaded, "load=$load")
        assertTrue(diffs.isEmpty(), "stale verdicts after the restart: $diffs")
        assertEquals(want.errorFiles.keys, got.errorFiles.keys)
        expect(restarted)
    }

    @Test fun aSealedSubclassAddedWhileDownIsSeenByTheWhenOverTheParent() = scenario(
        setup = {
            w("Shape.kt", "package p\n\nsealed class Shape\nclass Circle : Shape()\n")
            w("Use.kt", "package p\n\nfun area(s: Shape): Int = when (s) { is Circle -> 1 }\nfun protocolProbe() {}\n")
            w("Other.kt", "package p\n\nfun o() { protocolProbe() }\n")
        },
        whileDown = { w("Square.kt", "package p\n\nclass Square : Shape()\n") },
    )

    @Test fun aSealedSubclassRemovedWhileDown() = scenario(
        setup = {
            w("Shape.kt", "package p\n\nsealed class Shape\nclass Circle : Shape()\n")
            w("Square.kt", "package p\n\nclass Square : Shape()\n")
            w("Use.kt", "package p\n\nfun area(s: Shape): Int = when (s) { is Circle -> 1; is Square -> 2 }\nfun protocolProbe() {}\n")
        },
        whileDown = { File(src, "Square.kt").delete() },
    )

    @Test fun aSealedSubclassRemovedByASliceThenARestart() = scenario(
        setup = {
            w("Shape.kt", "package p\n\nsealed class Shape\nclass Circle : Shape()\n")
            w("Square.kt", "package p\n\nclass Square : Shape()\n")
            w("Use.kt", "package p\n\nfun area(s: Shape): Int = when (s) { is Circle -> 1; is Square -> 2 }\nfun protocolProbe() {}\n")
        },
        between = { s ->
            File(src, "Square.kt").delete()
            w("Use.kt", "package p\n\nfun area(s: Shape): Int = when (s) { is Circle -> 1 }\nfun protocolProbe() {}\n")
            s.check(5, refs(), rules)
        },
        // Square comes back while down: the restored session must know its binary was deleted, and Use.kt is no longer exhaustive.
        whileDown = { w("Square.kt", "package p\n\nclass Square : Shape()\n") },
    )

    @Test fun aPackageRemovedWhileDown() = scenario(
        setup = {
            w("Q.kt", "package q\n\nfun qf() {}\n")
            w("Use.kt", "package p\n\nimport q.*\n\nfun u() { qf() }\nfun protocolProbe() {}\n")
        },
        whileDown = { File(src, "Q.kt").delete() },
    )

    @Test fun aRenamedFunctionWhileDown() = scenario(
        setup = {
            w("Api.kt", "package p\n\nfun protocolProbe() {}\nfun base(): Int = 1\n")
            w("Use.kt", "package p\n\nfun u() { protocolProbe(); base() }\n")
            w("Third.kt", "package p\n\nfun t(): Int = base() + 1\n")
        },
        whileDown = { w("Api.kt", "package p\n\nfun protocolProbe() {}\nfun base2(): Int = 1\n") },
    )

    @Test fun aFileRenamedWhileDown() = scenario(
        setup = {
            w("Api.kt", "package p\n\nfun protocolProbe() {}\nfun base(): Int = 1\n")
            w("Use.kt", "package p\n\nfun u() { protocolProbe(); base() }\n")
        },
        whileDown = { File(src, "Api.kt").renameTo(File(src, "Api2.kt")) },
    )

    @Test fun aFileRenamedByASliceThenARestart() = scenario(
        setup = {
            w("Api.kt", "package p\n\nfun protocolProbe() {}\nfun base(): Int = 1\n")
            w("Use.kt", "package p\n\nfun u() { protocolProbe(); base() }\n")
            w("Third.kt", "package p\n\nfun t(): Int = base() + 1\n")
        },
        between = { s ->
            File(src, "Api.kt").renameTo(File(src, "Api2.kt"))
            s.check(5, refs(), rules)
        },
        whileDown = { w("Third.kt", "package p\n\nfun t(): Int = base() + base()\n") },
    )

    @Test fun anAbstractMemberAddedToTheBaseWhileDown() = scenario(
        setup = {
            w("Base.kt", "package p\n\nabstract class Base\n")
            w("Impl.kt", "package p\n\nclass Impl : Base()\nfun protocolProbe() {}\n")
        },
        whileDown = { w("Base.kt", "package p\n\nabstract class Base { abstract fun f() }\n") },
    )

    // A slice merged a surface change: Api.kt is binary-stale (its binary does not describe it), and the state that is
    // persisted says so. A restored session that forgot would let other files resolve against the stale binary.
    @Test fun binaryStaleFilesSurviveARestart() = scenario(
        setup = {
            w("Api.kt", "package p\n\nfun protocolProbe() {}\nfun base(): Int = 1\n")
            w("Use.kt", "package p\n\nfun u() { protocolProbe(); base() }\n")
            w("Third.kt", "package p\n\nfun t(): Int = base() + 1\n")
        },
        between = { s ->
            w("Api.kt", "package p\n\nfun protocolProbe() {}\nfun base(): Int = 1\nfun extra(): Int = 2\n")
            s.check(5, refs(), rules)
            assertTrue(s.incrementalState.binaryStale().isNotEmpty(), "the slice left a binary-stale file to persist")
        },
        whileDown = { w("Third.kt", "package p\n\nfun t(): Int = base() + extra()\n") },
        expect = { assertTrue(it.incrementalState.binaryStale().isNotEmpty(), "and the restored state still has it") },
    )

    // A slice merged a deletion: Gone.kt has a binary in the snapshot but no source, so every slice hides that binary.
    // While down, another file is made to call gone(); the restored session must report it unresolved, as a full check does.
    @Test fun deletedBinaryStaysHiddenAfterARestart() = scenario(
        setup = {
            w("Api.kt", "package p\n\nfun protocolProbe() {}\nfun base(): Int = 1\n")
            w("Gone.kt", "package p\n\nfun gone(): Int = 3\n")
            w("Use.kt", "package p\n\nfun u() { protocolProbe(); base() }\n")
        },
        between = { s ->
            File(src, "Gone.kt").delete()
            s.check(5, refs(), rules)
            assertTrue(s.incrementalState.removedSinceSnapshot().isNotEmpty(), "the slice left a removed binary to persist")
        },
        whileDown = { w("Use.kt", "package p\n\nfun u() { protocolProbe(); base(); gone() }\n") },
        expect = { assertTrue(it.incrementalState.removedSinceSnapshot().isNotEmpty(), "and the restored state still hides it") },
    )

    // The same, with the deleted function declared again elsewhere: the source one must win, not the ghost.
    @Test fun aDeletedFunctionRedeclaredWhileDownResolvesToTheSource() = scenario(
        setup = {
            w("Api.kt", "package p\n\nfun protocolProbe() {}\nfun base(): Int = 1\n")
            w("Gone.kt", "package p\n\nfun gone(): Int = 3\n")
            w("Use.kt", "package p\n\nfun u() { protocolProbe(); base() }\n")
        },
        between = { s ->
            File(src, "Gone.kt").delete()
            s.check(5, refs(), rules)
        },
        whileDown = {
            w("Use.kt", "package p\n\nfun u() { protocolProbe(); base(); gone() }\n")
            w("NewGone.kt", "package p\n\nfun gone(): Int = 4\n")
        },
    )
}
