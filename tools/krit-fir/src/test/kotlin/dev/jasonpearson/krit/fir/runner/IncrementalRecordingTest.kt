package dev.jasonpearson.krit.fir.runner

import dev.jasonpearson.krit.fir.CheckRequest
import dev.jasonpearson.krit.fir.checkers.protocol.ThrowingProbe
import dev.jasonpearson.krit.fir.incremental.CompileRecording
import dev.jasonpearson.krit.fir.incremental.CompileRecordingRegistry
import dev.jasonpearson.krit.fir.incremental.FileSurface
import dev.jasonpearson.krit.fir.incremental.IncrementalState
import dev.jasonpearson.krit.fir.incremental.LookupIndex
import dev.jasonpearson.krit.fir.incremental.SurfaceTestHook
import dev.jasonpearson.krit.fir.incremental.Sym
import dev.jasonpearson.krit.fir.incremental.sha1Hex
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
 * Incremental recording (#864): the lookups and declaration surfaces a whole-module check compile records
 * into [AnalysisSession.incrementalState], and the guarantee that recording changes no verdict.
 */
class IncrementalRecordingTest {
    @TempDir lateinit var tmp: Path

    private val stdlib = File(Unit::class.java.protectionDomain.codeSource.location.toURI()).absolutePath

    private fun write(name: String, text: String): File = tmp.resolve(name).toFile().apply { writeText(text) }

    private fun newSession(record: Boolean = true) =
        AnalysisSession(listOf(tmp.toString()), listOf(stdlib), incrementalRecording = record)

    private fun allFiles(): List<String> =
        tmp.toFile().listFiles { f -> f.extension == "kt" }!!.map { it.absolutePath }.sorted()

    private fun compile(
        session: AnalysisSession, rules: Set<String> = setOf("ProtocolProbe"),
    ): BatchResult = session.check(1, allFiles().map { FileRef(it) }, rules)

    private val baseLib = """
        package p

        interface I
        class Plain
        sealed class S
        class S1 : S()
        class Box { fun size(): Int = 1 }

        fun make() = Box()
        fun makePlain() = Plain()
        fun base() = 1
        fun ret(): Int = 1
        fun param(a: Int) {}
        fun over(a: Int) {}
        fun vis() {}
        fun body(): Int { val x = 1; return x }
        fun expr(): Int = 1
    """.trimIndent() + "\n"

    private fun baseline() {
        write("Lib.kt", baseLib)
        // `mid` has an inferred type that follows `base`; `use` reaches `size` through an inferred receiver.
        write("Mid.kt", "package p\n\nfun mid() = base()\n")
        write("Other.kt", "package p\n\nfun other(): String = \"x\"\n")
    }

    @AfterEach fun clearHook() { SurfaceTestHook.beforeFile = null }

    private fun lib() = tmp.resolve("Lib.kt").toString()
    private fun mid() = tmp.resolve("Mid.kt").toString()

    // ---- stability ----------------------------------------------------------------------------------------

    @Test fun surfaceHashesAreStableAcrossIdenticalCompiles() {
        baseline()
        val first = newSession()
        compile(first)
        val second = newSession()
        compile(second)
        for (file in allFiles()) {
            val hash = assertNotNull(first.incrementalState.surfaceHash(file), file)
            assertEquals(hash, second.incrementalState.surfaceHash(file), file)
        }
        // The same session compiling the same sources again: nothing changed.
        assertTrue(allFiles().all { first.incrementalState.surfaceChanged(it) }, "first compile has nothing to compare with")
        compile(first)
        assertEquals(2, first.incrementalState.generation)
        for (file in allFiles()) assertFalse(first.incrementalState.surfaceChanged(file), file)
        assertEquals(emptySet(), first.incrementalState.dependentsOf(allFiles()))
    }

    // ---- surface changes ----------------------------------------------------------------------------------

    private fun surfaceChangesAfter(edit: () -> Unit): Boolean {
        baseline()
        val session = newSession()
        compile(session)
        edit()
        compile(session)
        return session.incrementalState.surfaceChanged(lib())
    }

    private fun editLib(from: String, to: String) {
        assertTrue(from in baseLib, from)
        write("Lib.kt", baseLib.replace(from, to))
    }

    @Test fun surfaceChangesOnReturnTypeEdit() = assertTrue(surfaceChangesAfter { editLib("fun ret(): Int = 1", "fun ret(): Long = 1L") })

    @Test fun surfaceChangesOnParameterEdit() = assertTrue(surfaceChangesAfter { editLib("fun param(a: Int) {}", "fun param(a: String) {}") })

    @Test fun surfaceChangesOnNewOverload() = assertTrue(surfaceChangesAfter { editLib("fun over(a: Int) {}", "fun over(a: Int) {}\nfun over(a: String) {}") })

    @Test fun surfaceChangesOnVisibilityEdit() = assertTrue(surfaceChangesAfter { editLib("fun vis() {}", "internal fun vis() {}") })

    @Test fun surfaceChangesOnSupertypeEdit() = assertTrue(surfaceChangesAfter { editLib("class Plain\n", "class Plain : I\n") })

    @Test fun surfaceChangesOnNewSealedSubclassInAnotherFile() = assertTrue(
        surfaceChangesAfter { write("Sub.kt", "package p\n\nclass S2 : S()\n") },
    )

    @Test fun surfaceChangesOnInferredTypeChangePropagatedFromAnotherFile() {
        baseline()
        val session = newSession()
        compile(session)
        val before = session.incrementalState.surfaceHash(mid())
        // Only Lib.kt is edited; Mid.kt's `mid()` has no written type, so its resolved signature moves.
        write("Lib.kt", baseLib.replace("fun base() = 1", "fun base() = 1L"))
        compile(session)
        assertTrue(session.incrementalState.surfaceChanged(lib()))
        assertTrue(session.incrementalState.surfaceChanged(mid()), "inferred return type of mid() changed")
        assertTrue(session.incrementalState.surfaceHash(mid()) != before)
        assertEquals(setOf(mid()), session.incrementalState.dependentsOf(listOf(lib())))
    }

    @Test fun surfaceIsUnchangedByBodyAndCommentEdits() {
        baseline()
        val session = newSession()
        compile(session)
        val before = session.incrementalState.surfaceHash(lib())
        write(
            "Lib.kt",
            baseLib.replace("fun body(): Int { val x = 1; return x }", "fun body(): Int { val y = 40 + 2; return y }")
                .replace("fun expr(): Int = 1", "fun expr(): Int = 2 * 3")
                .replace("class Plain\n", "// a comment\n/** KDoc */\nclass Plain\n"),
        )
        compile(session)
        assertFalse(session.incrementalState.surfaceChanged(lib()))
        assertEquals(before, session.incrementalState.surfaceHash(lib()))
        assertEquals(emptySet(), session.incrementalState.dependentsOf(listOf(lib())))
    }

    // ---- lookups and the reverse index -------------------------------------------------------------------

    @Test fun lookupsAreRecordedForEveryRequestedFile() {
        baseline()
        val session = newSession()
        compile(session)
        assertEquals(allFiles().map { File(it).canonicalPath }.toSet(), session.incrementalState.files())
        for (file in allFiles()) assertTrue(session.incrementalState.lookupsOf(file).isNotEmpty(), "no lookups for $file")
        assertTrue(Sym("p", "base") in session.incrementalState.lookupsOf(mid()), session.incrementalState.lookupsOf(mid()).toString())
        for (file in allFiles()) assertNotNull(session.incrementalState.contentHash(file), file)
        assertEquals(sha1Hex(File(lib()).readBytes()), session.incrementalState.contentHash(lib()))
        val before = session.incrementalState.contentHash(lib())
        write("Lib.kt", baseLib + "// edited\n")
        compile(session)
        assertEquals(sha1Hex(File(lib()).readBytes()), session.incrementalState.contentHash(lib()))
        assertTrue(before != session.incrementalState.contentHash(lib()))
    }

    @Test fun dependentsIncludeAMemberReachedThroughAnInferredReceiver() {
        baseline()
        // App never names Box: `make()` is inferred as Box and `.size()` is a member of that inferred type.
        val app = write("App.kt", "package q\n\nimport p.make\n\nfun use() = make().size()\n").absolutePath
        val unrelated = write("Unrelated.kt", "package q\n\nfun unrelated(): Int = 3\n").absolutePath
        // Box lives in Lib.kt; changing Box.size's signature must reach App.
        val session = newSession()
        compile(session)
        assertTrue(Sym("p.Box", "size") in session.incrementalState.lookupsOf(app), session.incrementalState.lookupsOf(app).toString())
        assertFalse(Sym("p.Box", "size") in session.incrementalState.lookupsOf(unrelated))

        write("Lib.kt", baseLib.replace("class Box { fun size(): Int = 1 }", "class Box { fun size(): Long = 1L }"))
        compile(session)
        assertTrue(session.incrementalState.surfaceChanged(lib()))
        assertEquals(setOf(app), session.incrementalState.dependentsOf(listOf(lib())))
    }

    @Test fun bodyOnlyEditHasNoDependents() {
        baseline()
        write("App.kt", "package q\n\nimport p.make\n\nfun use() = make().size()\n")
        val session = newSession()
        compile(session)
        write("Lib.kt", baseLib.replace("class Box { fun size(): Int = 1 }", "class Box { fun size(): Int = 2 }"))
        compile(session)
        assertFalse(session.incrementalState.surfaceChanged(lib()))
        assertEquals(emptySet(), session.incrementalState.dependentsOf(listOf(lib())))
    }

    // ---- invocation fingerprint --------------------------------------------------------------------------

    @Test fun differentRulesKeepTheCompilerFactsAndChangeOnlyTheVerdictKey() {
        baseline()
        val session = newSession()
        compile(session, setOf("ProtocolProbe"))
        compile(session, setOf("ProtocolProbe"))
        assertEquals(2, session.incrementalState.generation)
        val compileKey = session.incrementalState.compileKey
        val verdictKey = session.incrementalState.verdictKey
        assertFalse(session.incrementalState.surfaceChanged(lib()))

        compile(session, setOf("ProtocolProbe", "ThrowingProbe"))
        assertEquals(3, session.incrementalState.generation, "different rules keep the state")
        assertEquals(compileKey, session.incrementalState.compileKey)
        assertTrue(verdictKey != session.incrementalState.verdictKey)
        assertFalse(session.incrementalState.surfaceChanged(lib()), "surfaces still compare with the previous commit")
        assertTrue(session.incrementalState.lookupsOf(mid()).isNotEmpty())
    }

    @Test fun aRequestForASubsetOfFilesKeepsTheState() {
        baseline()
        val session = newSession()
        compile(session)
        session.check(2, listOf(FileRef(mid())), setOf("ProtocolProbe"))
        assertEquals(2, session.incrementalState.generation)
        assertEquals(allFiles().size, session.incrementalState.files().size)
    }

    @Test fun aChangedClasspathEntryResetsTheState() {
        baseline()
        val extra = tmp.resolve("extra-classes").toFile().apply { mkdirs() }
        val entry = File(extra, "B.class").apply { writeText("x") }
        val session = AnalysisSession(listOf(tmp.resolve("src").toString()), listOf(stdlib, extra.path), incrementalRecording = true)
        File(tmp.resolve("src").toFile().apply { mkdirs() }, "A.kt").writeText("package a\nfun a() = 1\n")
        val files = listOf(FileRef(tmp.resolve("src/A.kt").toString()))
        session.check(1, files, emptySet())
        session.check(1, files, emptySet())
        assertEquals(2, session.incrementalState.generation)
        entry.setLastModified(entry.lastModified() + 5000)
        session.check(1, files, emptySet())
        assertEquals(1, session.incrementalState.generation, "a classpath entry that changed starts a new state")
    }

    @Test fun aRewrittenNestedClasspathFileResetsTheStateEvenWhenTheRootMtimeIsUnchanged() {
        val extra = tmp.resolve("extra-classes").toFile().apply { mkdirs() }
        val nested = File(extra, "pkg/deep/A.class").apply { parentFile.mkdirs(); writeText("v1") }
        val src = tmp.resolve("src").toFile().apply { mkdirs() }
        File(src, "A.kt").writeText("package a\nfun a() = 1\n")
        val session = AnalysisSession(listOf(src.path), listOf(stdlib, extra.path), incrementalRecording = true)
        val files = listOf(FileRef(File(src, "A.kt").path))
        session.check(1, files, emptySet())
        val before = session.incrementalState.compileKey
        session.check(1, files, emptySet())
        assertEquals(2, session.incrementalState.generation)

        val rootMtime = extra.lastModified()
        nested.writeText("version-two")
        nested.setLastModified(nested.lastModified() + 5000)
        extra.setLastModified(rootMtime)
        assertEquals(rootMtime, extra.lastModified())
        session.check(1, files, emptySet())
        assertTrue(before != session.incrementalState.compileKey)
        assertEquals(1, session.incrementalState.generation, "a rewritten nested class file starts a new state")
    }

    @Test fun theSharedOracleAndCheckCompileLeavesTheRecordedStateAlone() {
        baseline()
        val session = newSession()
        val paths = allFiles()
        val request = CheckRequest(id = 1, command = "check", files = paths.map { FileRef(it) }, rules = listOf("ProtocolProbe"))
        // On its own the shared compile records nothing: it has no snapshot to go with a recording.
        val (_, checked) = session.analyzeWithCheck(paths, request)
        assertNotNull(checked)
        assertEquals(0, session.incrementalState.generation)
        // After a recorded check it changes nothing in the state.
        compile(session)
        val lookups = paths.associateWith { session.incrementalState.lookupsOf(it) }
        val files = session.incrementalState.files()
        write("Other.kt", "package p\n\nfun other(): String = \"changed\"\n")
        session.analyzeWithCheck(paths, request)
        assertEquals(1, session.incrementalState.generation)
        assertEquals(files, session.incrementalState.files())
        for (file in paths) assertEquals(lookups[file], session.incrementalState.lookupsOf(file), file)
    }

    // ---- recording changes no verdict --------------------------------------------------------------------

    @Test fun recordingDoesNotChangeFindingsErrorFilesOrRuleErrors() {
        write("Api.kt", "package p\n\nfun protocolProbe() {}\nfun throwingProbeReport() {}\nfun throwingProbeCrash() {}\n")
        write("ThrowsInCall.kt", "package p\n\nfun a() { throwingProbeReport(); throwingProbeCrash(); protocolProbe() }\n")
        write("Clean.kt", "package p\n\nfun e() { throwingProbeReport(); protocolProbe() }\n")
        write("Broken.kt", "package p\n\nfun broken() { protocolProbe(); missingFunction() }\n")
        val rules = setOf("ThrowingProbe", "ProtocolProbe")

        ThrowingProbe.reset()
        val plain = newSession(record = false)
        val off = compile(plain, rules)
        ThrowingProbe.reset()
        val recording = newSession(record = true)
        val on = compile(recording, rules)

        assertTrue(off.findings.isNotEmpty() && off.errorFiles.isNotEmpty() && off.ruleErrors.isNotEmpty(), off.toString())
        fun BatchResult.normalized() = copy(
            findings = findings.sortedWith(compareBy({ it.path }, { it.line }, { it.col }, { it.rule })),
        )
        assertEquals(off.normalized(), on.normalized())
        assertEquals(off.errorFiles, on.errorFiles)
        assertEquals(off.ruleErrors, on.ruleErrors)
        // Off records nothing; on records every file.
        assertEquals(emptySet(), plain.incrementalState.files())
        assertEquals(allFiles().size, recording.incrementalState.files().size)
    }

    // ---- structural facts the lookups cannot see ---------------------------------------------------------

    @Test fun aClassHeaderChangeReachesAFileThatOnlyLooksUpMembersInThatScope() {
        baseline()
        // `makePlain()` is inferred as Plain: the file looks up toString in scope p.Plain and never names Plain.
        val user = write("PlainUser.kt", "package q\n\nimport p.makePlain\n\nfun u() = makePlain().toString()\n").absolutePath
        val session = newSession()
        compile(session)
        val lookups = session.incrementalState.lookupsOf(user)
        assertTrue(lookups.any { it.scope == "p.Plain" }, lookups.toString())

        editLib("class Plain\n", "class Plain : I\n")
        compile(session)
        assertTrue(session.incrementalState.surfaceChanged(lib()))
        assertTrue(user in session.incrementalState.dependentsOf(listOf(lib())))
    }

    @Test fun aClassHeaderChangeIsReportedAsTheWholeClassScope() {
        fun surface(header: String) = FileSurface(
            mapOf(Sym("p", "class:Plain") to header, Sym("p.Plain", "size") to "s", Sym("", "class:Top") to "t"), emptyMap(),
        )
        val (symbols, scopes) = FileSurface.diff(surface("a"), surface("b"))
        assertEquals(setOf(Sym("p", "Plain")), symbols)
        assertEquals(setOf("p.Plain"), scopes)
        assertEquals(setOf("Top"), FileSurface.diff(null, FileSurface(mapOf(Sym("", "class:Top") to "t"), emptyMap())).second)
        // A member change alone is only that symbol.
        val member = FileSurface.diff(surface("a"), surface("a").copy(decls = surface("a").decls + (Sym("p.Plain", "size") to "s2")))
        assertEquals(setOf(Sym("p.Plain", "size")), member.first)
        assertEquals(emptySet(), member.second)

        val index = LookupIndex()
        index.update("/a.kt", setOf(Sym("p.Plain", "toString")))
        index.update("/b.kt", setOf(Sym("p", "other")))
        assertEquals(setOf("/a.kt"), index.dependents(symbols, scopes))
        assertEquals(emptySet(), index.dependents(symbols, emptySet()))
    }

    @Test fun aNewSealedSubclassInAnotherFileReachesTheWhenFile() {
        baseline()
        val whenFile = write("WhenUser.kt", "package q\n\nimport p.S\nimport p.S1\n\nfun w(s: S): Int = when (s) { is S1 -> 1 }\n").absolutePath
        val session = newSession()
        compile(session)
        write("Sub.kt", "package p\n\nclass S2 : S()\n")
        compile(session)
        assertTrue(session.incrementalState.surfaceChanged(lib()))
        assertTrue(whenFile in session.incrementalState.dependentsOf(listOf(lib())))
    }

    @Test fun renamingABaseMemberIsSeenThroughTheSubclassIndex() {
        baseline()
        val base = write("Base.kt", "package p\n\nopen class Base { open fun m() {} }\n").absolutePath
        val sub = write("SubOfBase.kt", "package p\n\nclass SubOfBase : Base()\n").absolutePath
        val session = newSession()
        compile(session)
        assertEquals(base, session.incrementalState.fileDeclaring("p.Base"))
        assertEquals(setOf(base), session.incrementalState.filesDeclaring(Sym("p", "Base")))
        assertEquals(setOf(sub), session.incrementalState.subclassFilesOf("p.Base"))

        write("Base.kt", "package p\n\nopen class Base { open fun n() {} }\n")
        compile(session)
        val change = assertNotNull(session.incrementalState.changeOf(base))
        assertEquals(setOf(Sym("p.Base", "m"), Sym("p.Base", "n")), change.symbols)
        // Lookups alone miss the subclass: SubOfBase never looked up Base#m.
        assertFalse(sub in session.incrementalState.dependentsOf(listOf(base)))
        assertEquals(setOf(sub), session.incrementalState.subclassFilesOf("p.Base"))
        assertEquals(setOf(base), session.incrementalState.filesDeclaring(Sym("p.Base", "n")))
        assertEquals(emptySet(), session.incrementalState.filesDeclaring(Sym("p.Base", "m")))
    }

    @Test fun aRemovedSupertypeIsKeptOnTheChange() {
        baseline()
        val session = newSession()
        write("Lib.kt", baseLib.replace("class Plain\n", "class Plain : I\n"))
        compile(session)
        assertEquals(setOf(lib()), session.incrementalState.subclassFilesOf("p.I"))
        write("Lib.kt", baseLib)
        compile(session)
        assertEquals(emptySet(), session.incrementalState.subclassFilesOf("p.I"))
        val previous = assertNotNull(session.incrementalState.changeOf(lib())?.previous)
        assertEquals(listOf("p.I"), previous.supers["p.Plain"])
    }

    // ---- the index follows the latest compile ------------------------------------------------------------

    @Test fun aFileThatLosesAllItsLookupsKeepsNone() {
        baseline()
        val empty = write("Empty.kt", "package q\n\nfun u() {}\nfun h() = u()\n").absolutePath
        val session = newSession()
        compile(session)
        assertTrue(session.incrementalState.lookupsOf(empty).isNotEmpty())
        write("Empty.kt", "package q\n")
        compile(session)
        assertEquals(emptySet(), session.incrementalState.lookupsOf(empty))
        assertTrue(File(empty).canonicalPath in session.incrementalState.files())
    }

    @Test fun aDeletedFileDisappearsFromTheState() {
        baseline()
        val other = tmp.resolve("Other.kt").toString()
        val session = newSession()
        compile(session)
        assertTrue(File(other).canonicalPath in session.incrementalState.files())
        File(other).delete()
        compile(session)
        assertFalse(File(other).canonicalPath in session.incrementalState.files())
        assertEquals(emptySet(), session.incrementalState.lookupsOf(other))
        assertNull(session.incrementalState.contentHash(other))
        assertNull(session.incrementalState.surfaceHash(other))
    }

    // ---- recording is best-effort ------------------------------------------------------------------------

    private fun failureFixture(): Set<String> {
        write("Api.kt", "package p\n\nfun protocolProbe() {}\nfun throwingProbeReport() {}\nfun throwingProbeCrash() {}\n")
        write("ThrowsInCall.kt", "package p\n\nfun a() { throwingProbeReport(); throwingProbeCrash(); protocolProbe() }\n")
        write("Clean.kt", "package p\n\nfun e() { throwingProbeReport(); protocolProbe() }\n")
        write("Broken.kt", "package p\n\nfun broken() { protocolProbe(); missingFunction() }\n")
        return setOf("ThrowingProbe", "ProtocolProbe")
    }

    private fun BatchResult.normalized() = copy(
        findings = findings.sortedWith(compareBy({ it.path }, { it.line }, { it.col }, { it.rule })),
    )

    private fun assertSameVerdict(off: BatchResult, on: BatchResult) {
        assertTrue(off.findings.isNotEmpty() && off.errorFiles.isNotEmpty() && off.ruleErrors.isNotEmpty(), off.toString())
        assertEquals(off.normalized(), on.normalized())
        assertEquals(off.errorFiles, on.errorFiles)
        assertEquals(off.ruleErrors, on.ruleErrors)
        assertTrue(on.crashed.isEmpty(), on.crashed.toString())
    }

    private fun verdictWithRecordingOff(rules: Set<String>): BatchResult {
        ThrowingProbe.reset()
        return compile(newSession(record = false), rules)
    }

    @Test fun aThrowingSurfaceCheckerDoesNotChangeTheVerdict() {
        val rules = failureFixture()
        val off = verdictWithRecordingOff(rules)
        SurfaceTestHook.beforeFile = { path -> if (path.endsWith("Clean.kt")) error("surface boom") }
        ThrowingProbe.reset()
        val session = newSession()
        val on = compile(session, rules)
        assertSameVerdict(off, on)
        assertEquals(0, session.incrementalState.generation, "a poisoned recording is not committed")
        assertEquals(emptySet(), session.incrementalState.files())
        assertNull(CompileRecordingRegistry.current())
    }

    @Test fun aThrowingCommitDoesNotChangeTheVerdictOrTheSharedCompile() {
        val rules = failureFixture()
        val off = verdictWithRecordingOff(rules)
        class ThrowingState : IncrementalState() {
            override fun commit(recording: CompileRecording) = error("commit boom")
        }
        ThrowingProbe.reset()
        val session = newSession()
        session.incrementalState = ThrowingState()
        assertSameVerdict(off, compile(session, rules))
        assertNull(CompileRecordingRegistry.current())

        // The shared oracle/check compile must stay shared: a recording failure is not a compile failure.
        ThrowingProbe.reset()
        val paths = allFiles()
        val (_, checked) = session.analyzeWithCheck(
            paths, CheckRequest(id = 1, command = "check", files = paths.map { FileRef(it) }, rules = rules.toList()),
        )
        assertNotNull(checked, "the check should still share the oracle's compilation")
        assertSameVerdict(off, checked)
    }

    @Test fun theRecordingRegistryIsClearAfterACompileWhoseCheckerThrew() {
        val rules = failureFixture()
        SurfaceTestHook.beforeFile = { _ -> throw StackOverflowError() }
        val session = newSession()
        compile(session, rules)
        assertNull(CompileRecordingRegistry.current())
        assertEquals(0, session.incrementalState.generation)
        // The next compile, with the fault gone, records again.
        SurfaceTestHook.beforeFile = null
        compile(session, rules)
        assertEquals(1, session.incrementalState.generation)
        assertNull(CompileRecordingRegistry.current())
    }
}
