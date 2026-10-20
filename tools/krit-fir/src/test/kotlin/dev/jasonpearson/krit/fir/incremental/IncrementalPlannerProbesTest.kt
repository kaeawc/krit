package dev.jasonpearson.krit.fir.incremental

import dev.jasonpearson.krit.fir.runner.AnalysisSession
import dev.jasonpearson.krit.fir.runner.BatchResult
import dev.jasonpearson.krit.fir.runner.FileRef
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.io.File
import java.nio.file.Path
import kotlin.test.assertEquals
import kotlin.test.assertNotNull

/**
 * Planner against a fresh full check on the constructs the lookup index and the structural rules have to get right:
 * subset requests, star imports, shadowing, visibility, const values, deprecations, inheritance, package moves,
 * files that leave the module, cancellation between waves, and a file whose dependency only changes in wave 2.
 */
class IncrementalPlannerProbesTest {
    @TempDir lateinit var tmp: Path
    private val src get() = tmp.resolve("src").toFile()
    private val out get() = tmp.resolve("outside").toFile()
    private val rules = setOf("ProtocolProbe")
    private val sessions = ArrayList<AnalysisSession>()
    @AfterEach fun cleanUp() { sessions.forEach { it.dispose() } }

    private fun w(rel: String, text: String) = src.writeSource(rel, text)
    private fun p(rel: String) = canon(File(src, rel).path)
    private fun planned() = AnalysisSession(listOf(src.path), listOf(testStdlib), incrementalRecording = true, incrementalPlanning = true)
        .also { it.snapshotRootOverride = tmp.resolve("snaps").toFile(); sessions += it }
    private fun full(files: List<FileRef>) = plainSession(src).also { sessions += it }.check(99, files, rules)

    private fun same(label: String, s: AnalysisSession, files: List<FileRef>, expectIncremental: Boolean = true): BatchResult {
        val got = s.check(1, files, rules)
        val want = full(files)
        val plan = assertNotNull(s.lastPlan)
        for (f in files.map { it.path }) assertEquals(want.verdictOf(f).copy(errorMessage = null), got.verdictOf(f).copy(errorMessage = null), "$label: ${File(f).name}")
        assertEquals(want.errorFiles.keys, got.errorFiles.keys, label)
        if (expectIncremental) assertEquals(PlanPath.INCREMENTAL, plan.path, "$label: $plan")
        return got
    }

    private fun refs(vararg rel: String) = rel.map { FileRef(p(it)) }

    @Test fun inferredChainWithSubsetRequest() {
        w("C.kt", "package p\n\nobject C { fun h(): Int = 1 }\n")
        w("B.kt", "package p\n\nobject B { fun g() = C.h() }\n")
        w("A.kt", "package p\n\nfun a() { val x: Int = B.g(); protocolProbe() }\n")
        w("Api.kt", "package p\n\nfun protocolProbe() {}\n")
        w("Z.kt", "package p\n\nfun z() = 1\n")
        val s = planned()
        val req = refs("A.kt")
        same("cold", s, req, false)
        w("C.kt", "package p\n\nobject C { fun h(): Long = 1L }\n")
        val r = same("inferred", s, req)
        assert(p("A.kt") in r.errorFiles) { "A should error: ${r.errorFiles}" }
        w("C.kt", "package p\n\nobject C { fun h(): Int = 1 }\n")
        same("revert", s, req)
    }

    @Test fun nonRequestedChangedFileThenRequested() {
        w("B.kt", "package p\n\nfun b(): Int = 1\n")
        w("A.kt", "package p\n\nfun a() { val x: Int = b(); protocolProbe() }\n")
        w("Api.kt", "package p\n\nfun protocolProbe() {}\n")
        val s = planned()
        same("cold", s, refs("A.kt"), false)
        w("B.kt", "package p\n\nfun b(): String = \"\"\n")
        same("B changed, only A requested", s, refs("A.kt"))
        same("now B requested", s, refs("A.kt", "B.kt"))
    }

    @Test fun starImportAmbiguity() {
        w("q/Foo.kt", "package q\n\nclass Foo\n")
        w("r/Other.kt", "package r\n\nclass Other\n")
        w("A.kt", "package p\n\nimport q.*\nimport r.*\n\nfun a(): Foo? = null\n")
        val s = planned()
        same("cold", s, refs("A.kt"), false)
        w("r/Foo.kt", "package r\n\nclass Foo\n")
        same("new ambiguous Foo in r", s, refs("A.kt"))
    }

    @Test fun samePackageShadowsDefaultImport() {
        w("A.kt", "package p\n\nfun a(): List<Int> = emptyList()\n")
        val s = planned()
        same("cold", s, refs("A.kt"), false)
        w("L.kt", "package p\n\nclass List<T>\n")
        same("class List in package p", s, refs("A.kt"))
        File(p("L.kt")).delete()
        same("removed", s, refs("A.kt"))
    }

    @Test fun deletedImportedFileSubsetRequest() {
        w("d/D.kt", "package d\n\nfun dd() = 1\n")
        w("A.kt", "package p\n\nimport d.dd\n\nfun a() = dd()\n")
        w("U.kt", "package p\n\nimport d.dd\n")
        val s = planned()
        same("cold", s, refs("A.kt", "U.kt"), false)
        File(p("d/D.kt")).delete()
        val r = same("deleted", s, refs("A.kt", "U.kt"), false)
        assertEquals(FullReason.PACKAGE_REMOVED, s.lastPlan!!.reason)
        assert(r.errorFiles.size == 2) { r.errorFiles.toString() }
    }

    @Test fun requestedFileOutsideModule() {
        out.mkdirs()
        val x = File(out, "X.kt"); x.writeText("package x\n\nfun xx() = 1\n")
        w("A.kt", "package p\n\nimport x.xx\n\nfun a() = xx()\n")
        val s = planned()
        val withX = refs("A.kt") + FileRef(canon(x.path))
        same("cold with X", s, withX, false)
        val r = same("without X", s, refs("A.kt"), false)
        assert(p("A.kt") in r.errorFiles) { "X is no longer in the module: ${r.errorFiles}" }
        // The module had an error (no snapshot), so the request that brings X back is a full check.
        same("with X again", s, withX, false)
        x.writeText("package x\n\nfun xx(): String = \"\"\n")
        same("X edited", s, withX)
    }

    @Test fun aFileThatLeavesTheModuleButStaysOnDiskIsNotCompiledAsPresent() {
        out.mkdirs()
        val x = File(out, "X.kt"); x.writeText("package x\n\nfun xx() = 1\n")
        w("x/Peer.kt", "package x\n\nfun peer() = 2\n")
        w("A.kt", "package p\n\nimport x.xx\n\nfun a() = xx()\n")
        val s = planned()
        val withX = refs("A.kt", "x/Peer.kt") + FileRef(canon(x.path))
        same("cold with X", s, withX, false)
        val r = same("without X", s, refs("A.kt", "x/Peer.kt"))
        assert(p("A.kt") in r.errorFiles) { "X is no longer in the module: ${r.errorFiles}" }
        same("with X again", s, withX)
        x.writeText("package x\n\nfun xx(): String = \"\"\n")
        same("X edited", s, withX)
    }

    @Test fun constValueChange() {
        w("K.kt", "package p\n\nobject K { const val V = 1 }\n")
        w("A.kt", "package p\n\nconst val B: Int = 10 / K.V\n")
        w("Z.kt", "package p\n\nfun z() = 1\n")
        val s = planned()
        same("cold", s, refs("A.kt"), false)
        w("K.kt", "package p\n\nobject K { const val V = 0 }\n")
        val r = same("const out of range", s, refs("A.kt"))
        assert(p("A.kt") in r.errorFiles) { r.errorFiles.toString() }
    }

    @Test fun visibilityNarrowing() {
        w("B.kt", "package p\n\nfun b() = 1\n")
        w("A.kt", "package p\n\nfun a() = b()\n")
        val s = planned()
        same("cold", s, refs("A.kt"), false)
        w("B.kt", "package p\n\nprivate fun b() = 1\n")
        // private top-level in another file: unresolved for A
        val r = same("private", s, refs("A.kt"))
        assert(p("A.kt") in r.errorFiles)
    }

    @Test fun deprecationError() {
        w("B.kt", "package p\n\nfun b() = 1\n")
        w("A.kt", "package p\n\nfun a() = b()\n")
        val s = planned()
        same("cold", s, refs("A.kt"), false)
        w("B.kt", "package p\n\n@Deprecated(\"x\", level = DeprecationLevel.ERROR)\nfun b() = 1\n")
        val r = same("deprecated error", s, refs("A.kt"))
        assert(p("A.kt") in r.errorFiles)
    }

    @Test fun superclassBecomesFinal() {
        w("B.kt", "package p\n\nopen class Base\n")
        w("S.kt", "package p\n\nclass Sub : Base()\n")
        w("Z.kt", "package p\n\nfun z() = 1\n")
        val s = planned()
        same("cold", s, refs("S.kt"), false)
        w("B.kt", "package p\n\nclass Base\n")
        val r = same("final", s, refs("S.kt"))
        assert(p("S.kt") in r.errorFiles)
    }

    @Test fun interfaceGainsAbstractMember() {
        w("I.kt", "package p\n\ninterface I { fun m(): Int = 1 }\n")
        w("Impl.kt", "package p\n\nclass Impl : I\n")
        w("Z.kt", "package p\n\nfun z() = 1\n")
        val s = planned()
        same("cold", s, refs("Impl.kt"), false)
        w("I.kt", "package p\n\ninterface I { fun m(): Int }\n")
        val r = same("abstract", s, refs("Impl.kt"))
        assert(p("Impl.kt") in r.errorFiles)
    }

    @Test fun moveDeclarationBetweenFiles() {
        w("X.kt", "package p\n\nfun moved() = 1\n")
        w("Y.kt", "package p\n\nfun other() = 2\n")
        w("A.kt", "package p\n\nfun a() = moved()\n")
        val s = planned()
        same("cold", s, refs("A.kt"), false)
        w("X.kt", "package p\n\n")
        w("Y.kt", "package p\n\nfun other() = 2\nfun moved() = 1\n")
        same("moved", s, refs("A.kt"))
    }

    @Test fun packageChange() {
        w("B.kt", "package p\n\nfun b() = 1\n")
        w("A.kt", "package q\n\nimport p.b\n\nfun a() = b()\n")
        val s = planned()
        same("cold", s, refs("A.kt"), false)
        w("B.kt", "package p2\n\nfun b() = 1\n")
        val r = same("pkg", s, refs("A.kt"), false)
        assertEquals(FullReason.PACKAGE_REMOVED, s.lastPlan!!.reason)
        assert(p("A.kt") in r.errorFiles)
    }

    @Test fun extensionShadowedByMember() {
        w("K.kt", "package p\n\nclass K\n")
        w("E.kt", "package p\n\nfun K.ext(): Int = 1\n")
        w("A.kt", "package p\n\nfun a(k: K): Int = k.ext()\n")
        val s = planned()
        same("cold", s, refs("A.kt"), false)
        w("K.kt", "package p\n\nclass K { fun ext(): String = \"\" }\n")
        val r = same("member shadows ext", s, refs("A.kt"))
        assert(p("A.kt") in r.errorFiles)
    }

    @Test fun typealiasChange() {
        w("T.kt", "package p\n\ntypealias Id = Int\n")
        w("A.kt", "package p\n\nfun a(): Id = 1\n")
        val s = planned()
        same("cold", s, refs("A.kt"), false)
        w("T.kt", "package p\n\ntypealias Id = String\n")
        val r = same("alias", s, refs("A.kt"))
        assert(p("A.kt") in r.errorFiles)
    }

    @Test fun explicitImportBeatsPackageButNewTopLevelSameNameFunction() {
        // A calls foo(1) resolved to q.foo via explicit import; a new overload in q appears: conflicting overloads in q only.
        w("q/F.kt", "package q\n\nfun foo(x: Int) = x\n")
        w("A.kt", "package p\n\nimport q.foo\n\nfun a() = foo(1)\n")
        val s = planned()
        same("cold", s, refs("A.kt"), false)
        w("q/G.kt", "package q\n\nfun foo(x: Int) = x\n")
        same("conflicting overload", s, refs("A.kt"))
    }

    @Test fun starImportOfDeletedPackage() {
        w("d/D.kt", "package d\n\nfun dd() = 1\n")
        w("A.kt", "package p\n\nimport d.*\n\nfun a() = 1\n")
        val s = planned()
        same("cold", s, refs("A.kt"), false)
        File(p("d/D.kt")).delete()
        val r = same("star import of a package whose only file was deleted", s, refs("A.kt"), false)
        assertEquals(FullReason.PACKAGE_REMOVED, s.lastPlan!!.reason)
        assertEquals(setOf(p("A.kt")), r.errorFiles.keys)
    }

    @Test fun qualifiedUseOfDeletedPackage() {
        w("d/D.kt", "package d\n\nfun dd() = 1\n")
        w("A.kt", "package p\n\nfun a() = d.dd()\n")
        val s = planned()
        same("cold", s, refs("A.kt"), false)
        File(p("d/D.kt")).delete()
        same("qualified", s, refs("A.kt"), false)
        assertEquals(FullReason.PACKAGE_REMOVED, s.lastPlan!!.reason)
    }

    @Test fun cancellationInALaterWaveLeavesAHalfAppliedState() {
        w("Api.kt", "package p\n\nfun protocolProbe() {}\nfun base(): Int = 1\n")
        w("Use.kt", "package p\n\nfun u() { protocolProbe(); val x: Int = base() }\n")
        w("Far.kt", "package q\n\nimport p.base\n\nfun far(): Int = base()\n")
        val s = planned()
        val all = refs("Api.kt", "Use.kt", "Far.kt")
        same("cold", s, all, false)
        var compiles = 0
        IncrementalTestHook.onStage = { if (it == IncrementalStage.SLICE_COMPILE && ++compiles == 2) throw java.util.concurrent.CancellationException("client went away") }
        w("Api.kt", "package p\n\nfun protocolProbe() {}\nfun base(): Long = 1L\n")
        try {
            s.check(1, all, rules)
            kotlin.test.fail("the cancellation should propagate")
        } catch (_: java.util.concurrent.CancellationException) {
        }
        assertEquals(null, s.incrementalState.compileKey, "the half-applied state was dropped")
        IncrementalTestHook.onStage = null
        val r = same("next request after the cancelled one", s, all, false)
        assertEquals(FullReason.COLD, s.lastPlan!!.reason)
        assert(p("Use.kt") in r.errorFiles && p("Far.kt") in r.errorFiles) { r.errorFiles.toString() }
    }

    @Test fun aFileCompiledInWaveOneIsReaddedWhenALaterWaveChangesWhatItUses() {
        w("C.kt", "package p\n\nobject C { fun h(): Int = 1 }\n")
        w("B.kt", "package p\n\nobject B { fun g() = C.h() }\n")
        w("A.kt", "package p\n\nfun a() { val x: Int = B.g(); protocolProbe() }\n")
        w("Api.kt", "package p\n\nfun protocolProbe() {}\n")
        val s = planned()
        val all = refs("A.kt", "B.kt", "C.kt", "Api.kt")
        same("cold", s, all, false)
        // A is edited (body) together with C's signature: A is in wave 1 and B, A's dependency, only changes in wave 2.
        w("A.kt", "package p\n\nfun a() { val x: Int = B.g(); protocolProbe(); protocolProbe() }\n")
        w("C.kt", "package p\n\nobject C { fun h(): Long = 1L }\n")
        val r = same("A edited together with the signature of C", s, all)
        assert(p("A.kt") in r.errorFiles) { r.errorFiles.toString() }
    }

    @Test fun starImportOfDeletedPackageWhenTheImporterIsEditedToo() {
        w("d/D.kt", "package d\n\nfun dd() = 1\n")
        w("A.kt", "package p\n\nimport d.*\n\nfun a() = 1\n")
        val s = planned()
        same("cold", s, refs("A.kt"), false)
        File(p("d/D.kt")).delete()
        w("A.kt", "package p\n\nimport d.*\n\nfun a() = 2\n")
        val r = same("star import, importer edited in the same request", s, refs("A.kt"), false)
        assertEquals(FullReason.PACKAGE_REMOVED, s.lastPlan!!.reason)
        assertEquals(setOf(p("A.kt")), r.errorFiles.keys)
    }
}
