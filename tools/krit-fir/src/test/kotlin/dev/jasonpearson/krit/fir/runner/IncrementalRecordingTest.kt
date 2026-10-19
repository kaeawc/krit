package dev.jasonpearson.krit.fir.runner

import dev.jasonpearson.krit.fir.CheckRequest
import dev.jasonpearson.krit.fir.checkers.protocol.ThrowingProbe
import dev.jasonpearson.krit.fir.incremental.Sym
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.io.File
import java.nio.file.Path
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNotNull
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

    private fun surfaceChangesAfter(name: String, edit: () -> Unit): Boolean {
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

    @Test fun surfaceChangesOnReturnTypeEdit() = assertTrue(surfaceChangesAfter("return type") { editLib("fun ret(): Int = 1", "fun ret(): Long = 1L") })

    @Test fun surfaceChangesOnParameterEdit() = assertTrue(surfaceChangesAfter("parameter") { editLib("fun param(a: Int) {}", "fun param(a: String) {}") })

    @Test fun surfaceChangesOnNewOverload() = assertTrue(surfaceChangesAfter("overload") { editLib("fun over(a: Int) {}", "fun over(a: Int) {}\nfun over(a: String) {}") })

    @Test fun surfaceChangesOnVisibilityEdit() = assertTrue(surfaceChangesAfter("visibility") { editLib("fun vis() {}", "internal fun vis() {}") })

    @Test fun surfaceChangesOnSupertypeEdit() = assertTrue(surfaceChangesAfter("supertype") { editLib("class Plain\n", "class Plain : I\n") })

    @Test fun surfaceChangesOnNewSealedSubclassInAnotherFile() = assertTrue(
        surfaceChangesAfter("sealed subclass") { write("Sub.kt", "package p\n\nclass S2 : S()\n") },
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
        assertEquals(session.incrementalState.contentHash(lib()), session.incrementalState.contentHash(lib()))
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

    @Test fun aChangedInvocationFingerprintResetsTheState() {
        baseline()
        val session = newSession()
        compile(session, setOf("ProtocolProbe"))
        compile(session, setOf("ProtocolProbe"))
        assertEquals(2, session.incrementalState.generation)
        val fingerprint = session.incrementalState.fingerprint
        assertFalse(session.incrementalState.surfaceChanged(lib()))

        compile(session, setOf("ProtocolProbe", "ThrowingProbe"))
        assertEquals(1, session.incrementalState.generation, "different rules start a new state")
        assertTrue(fingerprint != session.incrementalState.fingerprint)
        assertTrue(session.incrementalState.surfaceChanged(lib()), "nothing to compare with after a reset")
    }

    @Test fun theSharedOracleAndCheckCompileRecordsToo() {
        baseline()
        val session = newSession()
        val paths = allFiles()
        val (_, checked) = session.analyzeWithCheck(
            paths, CheckRequest(id = 1, command = "check", files = paths.map { FileRef(it) }, rules = listOf("ProtocolProbe")),
        )
        assertNotNull(checked)
        assertEquals(1, session.incrementalState.generation)
        for (file in paths) assertTrue(session.incrementalState.lookupsOf(file).isNotEmpty(), file)
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
}
