package dev.jasonpearson.krit.fir.incremental

import dev.jasonpearson.krit.fir.checkers.protocol.ThrowingProbe
import dev.jasonpearson.krit.fir.runner.AnalysisSession
import dev.jasonpearson.krit.fir.runner.BatchResult
import dev.jasonpearson.krit.fir.runner.FileRef
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.io.File
import java.nio.file.Path
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNotEquals
import kotlin.test.assertNotNull
import kotlin.test.assertNull
import kotlin.test.assertTrue
import kotlin.test.fail

/** The snapshot, the slice compile and its merge into [IncrementalState] (#865). */
class SliceCompileTest {
    @TempDir lateinit var tmp: Path

    private val src get() = tmp.resolve("src").toFile()
    private val snaps get() = tmp.resolve("snapshots").toFile()
    private val rules = setOf("ProtocolProbe", "ThrowingProbe")

    @AfterEach fun clearHook() {
        IncrementalTestHook.onStage = null
        IncrementalTestHook.hideMessageBuffer = false
    }

    private fun write(rel: String, text: String) = src.writeSource(rel, text)
    private fun path(rel: String) = canon(File(src, rel).path)

    private val libText = "package p\n\nfun base(): Int = 1\nfun body(): Int { return 1 }\nclass Box { fun size(): Int = 1 }\n"

    private fun baseline() {
        write("Lib.kt", libText)
        write("Mid.kt", "package p\n\nfun mid() = base()\n")
        write("Use.kt", "package q\n\nimport p.Box\n\nfun use(b: Box) = b.size()\n")
        write("Other.kt", "package p\n\nfun other(): String = \"x\"\n")
    }

    private fun refs(vararg rel: String) = rel.map { FileRef(path(it)) }

    private fun full(session: AnalysisSession, ruleIds: Set<String> = rules): BatchResult =
        session.check(1, kotlinFilesIn(src).map { FileRef(it) }, ruleIds)

    private fun slice(session: AnalysisSession, vararg rel: String, ruleIds: Set<String> = rules): BatchResult {
        val outcome = session.sliceCheck(2, refs(*rel), ruleIds)
        return (outcome as? SliceOutcome.Checked)?.result ?: fail("fell back: ${(outcome as SliceOutcome.Fallback).reason} ${outcome.cause}")
    }

    private fun recording() = recordingSession(src, snaps)

    // ---- the snapshot ---------------------------------------------------------------------------------------------

    @Test fun aCleanCheckBuildsTheSnapshotUnderTheKritCacheDirNotTheProject() {
        baseline()
        val session = recording()
        full(session)
        val snapshot = assertNotNull(session.incrementalState.snapshot)
        assertEquals(null, session.sliceFallback())
        assertTrue(snapshot.dir.toPath().startsWith(snaps.toPath()), snapshot.dir.toString())
        assertTrue(filesUnder(snapshot.dir).any { it.endsWith("LibKt.class") }, filesUnder(snapshot.dir).toString())
        assertTrue(File(snapshot.dir, "META-INF/$SNAPSHOT_MODULE_NAME.kotlin_module").isFile)
        assertEquals(4, snapshot.fileCount())
        assertFalse(src.walkTopDown().any { it.extension == "class" }, "nothing is written into the project")
        session.dispose()
        assertFalse(snapshot.root.exists(), "dispose deletes the snapshot")
    }

    @Test fun theCheckThatBuildsTheSnapshotGivesTheVerdictsOfAPlainCheck() {
        write("Api.kt", "package p\n\nfun protocolProbe() {}\nfun throwingProbeReport() {}\nfun throwingProbeCrash() {}\n")
        write("ThrowsInCall.kt", "package p\n\nfun a() { throwingProbeReport(); throwingProbeCrash(); protocolProbe() }\n")
        write("Clean.kt", "package p\n\nfun e() { throwingProbeReport(); protocolProbe() }\n")
        ThrowingProbe.reset()
        val plain = full(plainSession(src))
        ThrowingProbe.reset()
        val session = recording()
        val on = full(session)
        assertNotNull(session.incrementalState.snapshot)
        assertTrue(plain.findings.isNotEmpty() && plain.ruleErrors.isNotEmpty(), plain.toString())
        for (f in kotlinFilesIn(src)) assertEquals(plain.verdictOf(f), on.verdictOf(f), f)
        assertEquals(plain.errorFiles, on.errorFiles)
        session.dispose()
    }

    @Test fun aModuleWithACompilerErrorGetsNoSnapshotAndTheSameVerdict() {
        baseline()
        write("Broken.kt", "package p\n\nfun broken() { missingFunction() }\n")
        val plain = full(plainSession(src))
        val session = recording()
        val on = full(session)
        assertTrue(plain.errorFiles.isNotEmpty())
        assertEquals(plain.errorFiles, on.errorFiles)
        assertNull(session.incrementalState.snapshot)
        assertEquals(SliceFallback.COMPILE_ERRORS, session.sliceFallback(), "the compile errors are the reason, not a missing build")
        assertEquals("COMPILE_ERRORS: compile errors in 1 file", session.sliceFallbackDescription())
        assertEquals(1, session.incrementalState.generation, "the recording is still committed")
        assertTrue(snaps.resolve("fir-snapshots").listFiles().orEmpty().isEmpty(), "no directory is left behind")
    }

    @Test fun aSnapshotThatCannotBeCreatedChangesNothingButTheSnapshot() {
        baseline()
        val plain = full(plainSession(src))
        val blocked = tmp.resolve("not-a-directory").toFile().apply { writeText("x") }
        val session = recordingSession(src, blocked)
        val on = full(session)
        assertEquals(plain.findings, on.findings)
        assertEquals(plain.errorFiles, on.errorFiles)
        assertNull(session.incrementalState.snapshot)
        assertEquals(SliceFallback.NO_SNAPSHOT, session.sliceFallback())
        assertEquals(1, session.incrementalState.generation)
    }

    @Test fun aBackendFailureAfterACleanFrontendChangesNothingButTheSnapshot() {
        write("Api.kt", "package p\n\nfun protocolProbe() {}\n")
        write("Use.kt", "package p\n\nfun u() { protocolProbe() }\n")
        val plain = full(plainSession(src))
        IncrementalTestHook.onStage = { if (it == IncrementalStage.SNAPSHOT_BACKEND) error("backend boom") }
        val session = recording()
        val on = full(session)
        assertTrue(plain.findings.isNotEmpty())
        assertEquals(plain.findings.sortedBy { it.path + it.line }, on.findings.sortedBy { it.path + it.line })
        assertEquals(plain.errorFiles, on.errorFiles)
        assertEquals(plain.crashed, on.crashed)
        assertEquals(null, on.firstCompilerError)
        assertNull(session.incrementalState.snapshot)
        assertEquals(1, session.incrementalState.generation)
        assertTrue(snaps.resolve("fir-snapshots").listFiles().orEmpty().isEmpty())
    }

    @Test fun withoutTheCompilersMessageBufferNoBackendRunsAndNothingChanges() {
        write("Api.kt", "package p\n\nfun protocolProbe() {}\n")
        write("Use.kt", "package p\n\nfun u() { protocolProbe() }\nfun v(): Int = \"x\"\n")
        val plain = full(plainSession(src))
        IncrementalTestHook.hideMessageBuffer = true
        val session = recording()
        val on = full(session)
        assertTrue(plain.errorFiles.isNotEmpty())
        assertEquals(plain.errorFiles, on.errorFiles)
        assertEquals(plain.findings, on.findings)
        assertEquals(plain.firstCompilerError, on.firstCompilerError)
        IncrementalTestHook.hideMessageBuffer = false
        // A clean module: the marker is reached, fails closed, and the result is the plain one.
        write("Use.kt", "package p\n\nfun u() { protocolProbe() }\n")
        val cleanPlain = full(plainSession(src))
        IncrementalTestHook.hideMessageBuffer = true
        val clean = full(session)
        assertEquals(cleanPlain.findings, clean.findings)
        assertEquals(cleanPlain.errorFiles, clean.errorFiles)
        assertEquals(null, clean.firstCompilerError)
        assertTrue(clean.crashed.isEmpty())
        assertNull(session.incrementalState.snapshot)
        assertTrue(snaps.resolve("fir-snapshots").listFiles().orEmpty().isEmpty(), "no directory is left behind")
        session.dispose()
    }

    @Test fun aSharedOracleAndCheckCompileKeepsTheSnapshot() {
        baseline()
        val session = recording()
        full(session)
        val snapshot = assertNotNull(session.incrementalState.snapshot)
        val before = filesUnder(snapshot.dir)
        val paths = kotlinFilesIn(src)
        val (_, checked) = session.analyzeWithCheck(
            paths, dev.jasonpearson.krit.fir.CheckRequest(id = 9, command = "check", files = paths.map { FileRef(it) }, rules = listOf("ProtocolProbe")),
        )
        assertNotNull(checked)
        assertEquals(null, session.sliceFallback())
        assertTrue(snapshot === session.incrementalState.snapshot)
        assertEquals(before, filesUnder(snapshot.dir))
        assertEquals(emptySet(), slice(session, "Lib.kt").errorFiles.keys)
        session.dispose()
    }

    // ---- sealed hierarchies and what a slice adds on its own -------------------------------------------------

    private fun sealedModule() {
        write("S.kt", "package p\n\nsealed interface U\nclass D1 : U\nfun bodyOnly(): Int = 1\n")
        write("S2.kt", "package p\n\nclass D2 : U\n")
        write("W.kt", "package p\n\nfun h(u: U): Int = when (u) { is D1 -> 1; is D2 -> 2 }\n")
    }

    @Test fun aBodyOnlyEditOfASealedParentsFileChangesNeitherItsSurfaceNorItsBinary() {
        sealedModule()
        val session = recording()
        full(session)
        val state = session.incrementalState
        val before = state.surfaceHash(path("S.kt"))
        write("S.kt", "package p\n\nsealed interface U\nclass D1 : U\nfun bodyOnly(): Int = 2\n")
        val outcome = session.sliceCheck(2, refs("S.kt"), rules) as SliceOutcome.Checked
        assertEquals(setOf(path("S2.kt")), outcome.added, "the inheritor in the snapshot is compiled with its sealed parent")
        assertEquals(before, state.surfaceHash(path("S.kt")), "the inheritor list is complete, so the surface is stable")
        assertFalse(state.surfaceChanged(path("S.kt")))
        assertFalse(state.surfaceChanged(path("S2.kt")))
        assertEquals(emptySet(), state.binaryStale())
        assertEquals(emptySet(), state.dependentsOf(listOf(path("S.kt"))))
        // And it stays that way for the next slice.
        write("S.kt", "package p\n\nsealed interface U\nclass D1 : U\nfun bodyOnly(): Int = 3\n")
        assertEquals(emptySet(), (session.sliceCheck(3, refs("S.kt"), rules) as SliceOutcome.Checked).result.errorFiles.keys)
        assertEquals(emptySet(), state.binaryStale())
        session.dispose()
    }

    @Test fun aClassThatBecomesSealedInTheEditBringsItsInheritorsToo() {
        write("S.kt", "package p\n\ninterface U\nclass D1 : U\n")
        write("S2.kt", "package p\n\nclass D2 : U\n")
        val session = recording()
        full(session)
        write("S.kt", "package p\n\nsealed interface U\nclass D1 : U\n")
        val outcome = session.sliceCheck(2, refs("S.kt"), rules) as SliceOutcome.Checked
        assertEquals(setOf(path("S.kt"), path("S2.kt")), outcome.compiled)
        assertEquals(setOf(path("S2.kt")), outcome.added)
        assertEquals(emptySet(), outcome.result.errorFiles.keys)
        session.dispose()
    }

    @Test fun aSliceAlwaysCompilesTheBinaryStaleFiles() {
        baseline()
        val session = recording()
        full(session)
        write("Lib.kt", libText.replace("fun base(): Int = 1", "fun base(): Long = 1L"))
        slice(session, "Lib.kt")
        assertEquals(setOf(path("Lib.kt")), session.incrementalState.binaryStale())
        // The caller asks for Mid.kt alone: the stale Lib.kt comes along, so Mid.kt sees the new base().
        val outcome = session.sliceCheck(3, refs("Mid.kt"), rules) as SliceOutcome.Checked
        assertEquals(setOf(path("Mid.kt"), path("Lib.kt")), outcome.compiled)
        assertEquals(setOf(path("Lib.kt")), outcome.added)
        assertEquals(setOf(path("Lib.kt"), path("Mid.kt")), session.incrementalState.binaryStale())
        session.dispose()
    }

    // ---- the slice check ---------------------------------------------------------------------------------------

    @Test fun aSliceReturnsTodaysPerFileShapeWithTheRuleErrorRecorderArmed() {
        write("Api.kt", "package p\n\nfun protocolProbe() {}\nfun throwingProbeReport() {}\nfun throwingProbeCrash() {}\n")
        write("ThrowsInCall.kt", "package p\n\nfun a() { throwingProbeReport(); throwingProbeCrash(); protocolProbe() }\n")
        write("Clean.kt", "package p\n\nfun e() { throwingProbeReport(); protocolProbe() }\n")
        ThrowingProbe.reset()
        val session = recording()
        val whole = full(session)
        ThrowingProbe.reset()
        val sliced = slice(session, "ThrowsInCall.kt", "Clean.kt")
        for (f in listOf("ThrowsInCall.kt", "Clean.kt")) assertEquals(whole.verdictOf(path(f)), sliced.verdictOf(path(f)), f)
        assertTrue(sliced.ruleErrors.getValue("ThrowingProbe").containsKey(path("ThrowsInCall.kt")), sliced.ruleErrors.toString())
        assertTrue(sliced.findings.isNotEmpty())
        assertEquals(2, sliced.succeeded)
        session.dispose()
    }

    @Test fun aSliceWithoutASnapshotIsAFallback() {
        baseline()
        val session = plainSession(src)
        full(session)
        assertEquals(SliceFallback.NO_SNAPSHOT, session.sliceFallback())
        assertEquals(SliceFallback.NO_SNAPSHOT, (session.sliceCheck(1, refs("Lib.kt"), rules) as SliceOutcome.Fallback).reason)
    }

    @Test fun aChangedClasspathEntryMakesTheSnapshotUnusable() {
        write("A.kt", "package a\nfun a() = 1\n")
        val extra = tmp.resolve("extra-classes").toFile().apply { mkdirs() }
        val session = AnalysisSession(listOf(src.path), listOf(testStdlib, extra.path), incrementalRecording = true)
        session.snapshotRootOverride = snaps
        session.check(1, refs("A.kt"), emptySet())
        assertEquals(null, session.sliceFallback())
        File(extra, "x.txt").writeText("changed")
        assertEquals(SliceFallback.STATE_CHANGED, session.sliceFallback())
        session.dispose()
    }

    @Test fun parkingRestoresTheSnapshotWhenTheSliceCompileThrows() {
        baseline()
        val session = recording()
        full(session)
        val snapshot = assertNotNull(session.incrementalState.snapshot)
        val before = filesUnder(snapshot.dir)
        val libClass = before.first { it.endsWith("LibKt.class") }
        var parkedDuring: Boolean? = null
        IncrementalTestHook.onStage = {
            if (it == IncrementalStage.SLICE_COMPILE) {
                parkedDuring = !File(snapshot.dir, libClass).exists()
                error("slice boom")
            }
        }
        val generation = session.incrementalState.generation
        val lookups = session.incrementalState.lookupsOf(path("Lib.kt"))
        val outcome = session.sliceCheck(2, refs("Lib.kt"), rules) as SliceOutcome.Fallback
        assertEquals(SliceFallback.SLICE_FAILED, outcome.reason)
        assertEquals("slice boom", outcome.cause?.message)
        assertEquals(true, parkedDuring, "the stale output is out of the way during the compile")
        assertEquals(before, filesUnder(snapshot.dir), "every class file is back")
        assertFalse(snapshot.corrupt)
        // The state is untouched and the next slice works.
        assertEquals(generation, session.incrementalState.generation)
        assertEquals(lookups, session.incrementalState.lookupsOf(path("Lib.kt")))
        assertNotNull(session.incrementalState.lastSliceFailure)
        IncrementalTestHook.onStage = null
        assertEquals(emptySet(), slice(session, "Lib.kt").errorFiles.keys)
        assertEquals(before, filesUnder(snapshot.dir))
        session.dispose()
    }

    @Test fun parkedPutsFilesBackWhenTheBlockThrows() {
        val root = tmp.resolve("snap-root").toFile()
        val snapshot = Snapshot(root)
        File(snapshot.dir, "p").mkdirs()
        val a = File(snapshot.dir, "p/AKt.class").apply { writeText("a") }
        File(snapshot.dir, "p/BKt.class").writeText("b")
        snapshot.adopt(mapOf(canon(File(tmp.toFile(), "A.kt").path) to listOf(a.path)))
        try {
            snapshot.parked(listOf(File(tmp.toFile(), "A.kt").path)) {
                assertFalse(a.exists())
                throw IllegalStateException("boom")
            }
            fail("should throw")
        } catch (e: IllegalStateException) {
            assertEquals("boom", e.message)
        }
        assertEquals("a", a.readText())
        assertFalse(snapshot.corrupt)
        snapshot.delete()
        assertFalse(root.exists())
    }

    // ---- merge-commit ---------------------------------------------------------------------------------------------

    @Test fun aSliceMergesIntoTheStateAndKeepsEverythingElse() {
        baseline()
        val session = recording()
        full(session)
        val state = session.incrementalState
        val useLookups = state.lookupsOf(path("Use.kt"))
        val otherLookups = state.lookupsOf(path("Other.kt"))
        val midLookups = state.lookupsOf(path("Mid.kt"))
        val otherHash = state.contentHash(path("Other.kt"))
        val libHash = state.contentHash(path("Lib.kt"))
        val oldLibLookups = state.lookupsOf(path("Lib.kt"))
        val allFiles = state.files()
        val compileKey = state.compileKey

        // Lib.kt now calls something else (a body edit) and gains a comment.
        write("Lib.kt", libText.replace("fun body(): Int { return 1 }", "fun body(): Int { return other().length }"))
        slice(session, "Lib.kt")

        assertEquals(2, state.generation)
        assertEquals(compileKey, state.compileKey)
        assertEquals(allFiles, state.files())
        // Compiled file: replaced.
        assertNotEquals(libHash, state.contentHash(path("Lib.kt")))
        assertTrue(Sym("p", "other") in state.lookupsOf(path("Lib.kt")), state.lookupsOf(path("Lib.kt")).toString())
        assertFalse(Sym("p", "other") in oldLibLookups)
        // Everything else: kept.
        assertEquals(useLookups, state.lookupsOf(path("Use.kt")))
        assertEquals(otherLookups, state.lookupsOf(path("Other.kt")))
        assertEquals(midLookups, state.lookupsOf(path("Mid.kt")))
        assertEquals(otherHash, state.contentHash(path("Other.kt")))
        // Changes: unchanged for the files that were not compiled; a body edit changed no surface.
        assertFalse(state.surfaceChanged(path("Use.kt")))
        assertFalse(state.surfaceChanged(path("Lib.kt")))
        assertEquals(emptySet(), state.dependentsOf(listOf(path("Lib.kt"))))
        session.dispose()
    }

    @Test fun aSliceKeepsThePreviousSurfaceOnTheChangeAndTheDeclarationIndexes() {
        baseline()
        val session = recording()
        full(session)
        val state = session.incrementalState
        assertEquals(path("Lib.kt"), state.fileDeclaring("p.Box"))
        write("Lib.kt", libText.replace("class Box { fun size(): Int = 1 }", "class Box { fun size(): Long = 1L }"))
        slice(session, "Lib.kt")
        val change = assertNotNull(state.changeOf(path("Lib.kt")))
        assertEquals(setOf(Sym("p.Box", "size")), change.symbols)
        assertNotNull(change.previous, "the surface before the slice is kept")
        assertTrue(state.surfaceChanged(path("Lib.kt")))
        assertEquals(setOf(path("Use.kt")), state.dependentsOf(listOf(path("Lib.kt"))))
        assertEquals(path("Lib.kt"), state.fileDeclaring("p.Box"))
        assertEquals(setOf(path("Lib.kt")), state.filesDeclaring(Sym("p.Box", "size")))
        session.dispose()
    }

    @Test fun aFullCompileStillReplacesTheWholeState() {
        baseline()
        val session = recording()
        full(session)
        write("Lib.kt", libText.replace("fun base(): Int = 1", "fun base(): Long = 1L"))
        slice(session, "Lib.kt")
        assertEquals(setOf(path("Lib.kt")), session.incrementalState.binaryStale())
        val oldRoot = assertNotNull(session.incrementalState.snapshot).root
        full(session)
        assertEquals(emptySet(), session.incrementalState.binaryStale())
        assertFalse(oldRoot.exists(), "the replaced snapshot is deleted")
        assertNotEquals(oldRoot, session.incrementalState.snapshot?.root)
        assertEquals(4, session.incrementalState.snapshot?.fileCount())
        assertTrue(session.incrementalState.files().containsAll(kotlinFilesIn(src)))
        session.dispose()
    }

    // ---- binary staleness --------------------------------------------------------------------------------------

    @Test fun aSignatureEditMakesTheFileBinaryStaleAndABodyEditDoesNot() {
        baseline()
        val session = recording()
        full(session)
        val state = session.incrementalState
        assertEquals(emptySet(), state.binaryStale())

        write("Other.kt", "package p\n\nfun other(): String = \"a different body\"\n")
        slice(session, "Other.kt")
        assertEquals(emptySet(), state.binaryStale(), "a body-only edit leaves the binary valid")

        write("Lib.kt", libText.replace("fun base(): Int = 1", "fun base(): Long = 1L"))
        slice(session, "Lib.kt")
        assertEquals(setOf(path("Lib.kt")), state.binaryStale())

        // Mid.kt's inferred type follows base(): compiled together with the binary-stale Lib.kt it becomes stale too.
        slice(session, "Lib.kt", "Mid.kt")
        assertEquals(setOf(path("Lib.kt"), path("Mid.kt")), state.binaryStale())

        // Restoring the signature makes the snapshot's binary valid again.
        write("Lib.kt", libText)
        slice(session, "Lib.kt", "Mid.kt")
        assertEquals(emptySet(), state.binaryStale())
        session.dispose()
    }

    @Test fun filesWithCompilerErrorsAndNewFilesAreBinaryStale() {
        baseline()
        val session = recording()
        full(session)
        val state = session.incrementalState
        write("Other.kt", "package p\n\nfun other(): String = missing()\n")
        val result = slice(session, "Other.kt")
        assertTrue(path("Other.kt") in result.errorFiles)
        assertEquals(setOf(path("Other.kt")), state.binaryStale())

        write("Fresh.kt", "package p\n\nfun fresh() = 1\n")
        slice(session, "Fresh.kt")
        assertEquals(setOf(path("Other.kt"), path("Fresh.kt")), state.binaryStale())

        write("Other.kt", "package p\n\nfun other(): String = \"x\"\n")
        slice(session, "Other.kt")
        assertEquals(setOf(path("Fresh.kt")), state.binaryStale(), "fixing the error with the same surface frees the file")
        session.dispose()
    }

    @Test fun aDeletedFileIsHiddenFromEverySliceUntilTheNextSnapshot() {
        baseline()
        val session = recording()
        full(session)
        val state = session.incrementalState
        File(path("Lib.kt")).delete()
        // Mid.kt used base() from the deleted file: the slice must not find the snapshot's binary.
        val result = slice(session, "Mid.kt", "Lib.kt")
        assertTrue(path("Mid.kt") in result.errorFiles, result.errorFiles.toString())
        assertEquals(setOf(path("Lib.kt")), state.removedSinceSnapshot())
        assertFalse(path("Lib.kt") in state.files())
        assertNull(state.contentHash(path("Lib.kt")))
        assertEquals(emptySet(), state.lookupsOf(path("Lib.kt")))
        // Still hidden in a later slice that does not mention it.
        assertTrue(path("Mid.kt") in slice(session, "Mid.kt").errorFiles)
        session.dispose()
    }

    @Test fun theStaleBound() {
        baseline()
        val session = recording()
        full(session)
        val state = session.incrementalState
        write("Lib.kt", libText.replace("fun base(): Int = 1", "fun base(): Long = 1L"))
        slice(session, "Lib.kt")
        assertEquals(1, state.binaryStale().size)
        assertFalse(session.needsResnapshot(), "one file of four is under the default minimum")
        assertTrue(session.needsResnapshot(maxFraction = 0.1, minFiles = 1))
        assertFalse(session.needsResnapshot(maxFraction = 0.5, minFiles = 1))
        // Resnapshot: a full check builds a fresh snapshot and replaces the state.
        session.check(3, kotlinFilesIn(src).map { FileRef(it) }, rules)
        assertEquals(emptySet(), state.binaryStale())
        assertFalse(session.needsResnapshot(0.0, 1))
        session.dispose()
    }
}
