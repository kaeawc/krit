package dev.jasonpearson.krit.fir.incremental

import dev.jasonpearson.krit.fir.runner.AnalysisSession
import dev.jasonpearson.krit.fir.runner.FileRef
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.io.File
import java.nio.file.Path
import kotlin.test.assertEquals
import kotlin.test.assertNotEquals
import kotlin.test.assertNotNull
import kotlin.test.assertTrue

/**
 * Constructs whose meaning could change when part of the module comes from class files instead of source (the
 * spike's `Shapes`). Each case builds a base module and its snapshot, applies an edit, and compares the verdict of
 * the files in the slice, checked alone against the snapshot, with a plain full check of the edited module.
 *
 * Most constructs are identical. The ones that are not (a new sealed subclass whose parent is not in the slice, a
 * redeclaration whose other side is not) are the structural rules the planner adds to its dirty set: the case that
 * puts the other side in the slice must be identical again.
 */
class SliceShapesTest {
    @TempDir lateinit var tmp: Path

    private class Case(
        val name: String,
        val base: Map<String, String>,
        val edit: Map<String, String?>,
        val slice: List<String>,
        /** Slice files (by name) whose slice verdict is expected to differ from the full check. */
        val differs: Set<String> = emptySet(),
        /** Files (by name) the full check reports a compiler error for after the edit. */
        val errors: Set<String> = emptySet(),
        /** Files outside the slice (by name) whose full verdict changed with the edit: what a slice cannot see. */
        val outside: Set<String> = emptySet(),
        val expectFindings: Boolean = false,
    )

    private val cases = listOf(
        Case(
            "internal",
            mapOf(
                "a/A.kt" to "package p\n\ninternal fun helper(): Int = 1\ninternal class Hidden { internal val v = 2 }\n",
                "b/B.kt" to "package p\n\nfun use(): Int = helper() + Hidden().v\n",
            ),
            mapOf("b/B.kt" to "package p\n\nfun use(): Int = helper() + Hidden().v + 1\n"), listOf("b/B.kt"),
        ),
        Case(
            "facade-jvmname",
            mapOf(
                "a/Utils.kt" to "@file:JvmName(\"UtilsX\")\npackage p\n\nfun top(): String = \"t\"\nval topVal: Int = 3\nfun String.ext(): Int = length\n",
                "b/B.kt" to "package p\n\nfun use(): Int = top().ext() + topVal\n",
            ),
            mapOf("b/B.kt" to "package p\n\nfun use(): Int = top().ext() + topVal + 1\n"), listOf("b/B.kt"),
        ),
        Case(
            "multifile",
            mapOf(
                "a/M1.kt" to "@file:JvmName(\"Multi\")\n@file:JvmMultifileClass\npackage p\n\nfun m1(): Int = 1\n",
                "a/M2.kt" to "@file:JvmName(\"Multi\")\n@file:JvmMultifileClass\npackage p\n\nfun m2(): Int = 2\n",
                "b/B.kt" to "package p\n\nfun use(): Int = m1() + m2()\n",
            ),
            // M2's part stays in the snapshot while M1's part is compiled from source.
            mapOf("a/M1.kt" to "@file:JvmName(\"Multi\")\n@file:JvmMultifileClass\npackage p\n\nfun m1(): Int = 10\n"),
            listOf("a/M1.kt", "b/B.kt"),
        ),
        Case(
            "sealed-parent-in-snapshot",
            mapOf(
                "a/S.kt" to "package p\n\nsealed class S\nclass A1 : S()\n",
                "b/B.kt" to "package p\n\nclass B1 : S()\nfun f(s: S): Int = when (s) { is A1 -> 1; is B1 -> 2 }\n",
            ),
            // A subclass in source of a sealed parent in the snapshot: not a "different module".
            mapOf("b/B.kt" to "package p\n\nclass B1 : S()\nfun f(s: S): Int = when (s) { is A1 -> 1; is B1 -> 3 }\n"),
            listOf("b/B.kt"),
        ),
        Case(
            "sealed-hierarchy-in-snapshot-when-in-slice",
            mapOf(
                "a/S.kt" to "package p\n\nsealed interface T\nclass C1 : T\nclass C2 : T\n",
                "b/W.kt" to "package p\n\nfun g(t: T): Int = when (t) { is C1 -> 1; is C2 -> 2 }\n",
            ),
            // A missing branch over a hierarchy that only exists as binaries is still an error.
            mapOf("b/W.kt" to "package p\n\nfun g(t: T): Int = when (t) { is C1 -> 1 }\n"),
            listOf("b/W.kt"),
            errors = setOf("W.kt"),
        ),
        Case(
            "sealed-new-subclass-other-file",
            mapOf(
                "a/S.kt" to "package p\n\nsealed interface U\nclass D1 : U\n",
                "b/W.kt" to "package p\n\nfun h(u: U): Int = when (u) { is D1 -> 1 }\n",
                "c/D2.kt" to "package p\n\nclass Other\n",
            ),
            mapOf("c/D2.kt" to "package p\n\nclass Other\nclass D2 : U\n"),
            listOf("c/D2.kt"),
            errors = setOf("W.kt"),
            outside = setOf("W.kt"), // the when in W.kt is no longer exhaustive; only a planner can know to recheck it
        ),
        Case(
            "sealed-new-subclass-when-in-slice",
            mapOf(
                "a/S.kt" to "package p\n\nsealed interface U\nclass D1 : U\n",
                "b/W.kt" to "package p\n\nfun h(u: U): Int = when (u) { is D1 -> 1 }\n",
                "c/D2.kt" to "package p\n\nclass Other\n",
            ),
            mapOf("c/D2.kt" to "package p\n\nclass Other\nclass D2 : U\n"),
            listOf("c/D2.kt", "b/W.kt"),
            // The parent's binary still lists only D1, so the when looks exhaustive: the parent's file has to be
            // in the slice too (next case).
            differs = setOf("W.kt"),
            errors = setOf("W.kt"),
        ),
        Case(
            "sealed-new-subclass-parent-in-slice",
            mapOf(
                "a/S.kt" to "package p\n\nsealed interface U\nclass D1 : U\n",
                "b/W.kt" to "package p\n\nfun h(u: U): Int = when (u) { is D1 -> 1 }\n",
                "c/D2.kt" to "package p\n\nclass Other\n",
            ),
            mapOf("c/D2.kt" to "package p\n\nclass Other\nclass D2 : U\n"),
            listOf("c/D2.kt", "b/W.kt", "a/S.kt"),
            errors = setOf("W.kt"),
        ),
        Case(
            "redeclaration",
            mapOf(
                "a/A.kt" to "package p\n\nclass Dup\n",
                "b/B.kt" to "package p\n\nclass Other\n",
            ),
            // The class in B shadows the binary one: only with A in the slice too does B see the redeclaration.
            mapOf("b/B.kt" to "package p\n\nclass Other\nclass Dup\n"),
            listOf("b/B.kt"),
            differs = setOf("B.kt"),
            errors = setOf("A.kt", "B.kt"),
            outside = setOf("A.kt"),
        ),
        Case(
            "redeclaration-both-in-slice",
            mapOf(
                "a/A.kt" to "package p\n\nclass Dup\n",
                "b/B.kt" to "package p\n\nclass Other\n",
            ),
            mapOf("b/B.kt" to "package p\n\nclass Other\nclass Dup\n"),
            listOf("b/B.kt", "a/A.kt"),
            errors = setOf("A.kt", "B.kt"),
        ),
        Case(
            "conflicting-overloads",
            mapOf(
                "a/A.kt" to "package p\n\nfun ov(x: Int): Int = x\n",
                "b/B.kt" to "package p\n\nfun other() = 1\n",
            ),
            mapOf("b/B.kt" to "package p\n\nfun other() = 1\nfun ov(x: Int): Int = x + 1\n"),
            listOf("b/B.kt"),
            errors = setOf("A.kt", "B.kt"),
            outside = setOf("A.kt"), // A.kt gets the conflicting-overloads error too
        ),
        Case(
            "conflicting-overloads-both-in-slice",
            mapOf(
                "a/A.kt" to "package p\n\nfun ov(x: Int): Int = x\n",
                "b/B.kt" to "package p\n\nfun other() = 1\n",
            ),
            mapOf("b/B.kt" to "package p\n\nfun other() = 1\nfun ov(x: Int): Int = x + 1\n"),
            listOf("b/B.kt", "a/A.kt"),
            errors = setOf("A.kt", "B.kt"),
        ),
        Case(
            "inline-and-const",
            mapOf(
                "a/A.kt" to "package p\n\nconst val TAG: String = \"a-very-long-log-tag-name-over-23\"\nconst val N: Int = 3\n" +
                    "inline fun <reified T> name(): String = T::class.java.name\ninline fun twice(f: () -> Int): Int = f() + f()\n",
                "b/B.kt" to "package p\n\nfun use(x: Int): Int = when (x) { N -> twice { 1 }; else -> name<String>().length }\nfun tagLen(): Int = TAG.length\n",
            ),
            mapOf(
                "b/B.kt" to "package p\n\nfun use(x: Int): Int = when (x) { N -> twice { 2 }; else -> name<String>().length }\nfun tagLen(): Int = TAG.length\n",
            ),
            listOf("b/B.kt"),
        ),
        Case(
            "rule-reads-a-const-from-the-snapshot",
            mapOf(
                "a/T.kt" to "package p\n\nconst val LONG_TAG: String = \"ThisTagIsLongerThanTwentyThree\"\n",
                "b/L.kt" to "package p\n\nimport android.util.Log\n\nfun log() { Log.d(LONG_TAG, \"m\") }\n",
                "android/util/Log.kt" to "package android.util\n\nobject Log { @JvmStatic fun d(tag: String, msg: String): Int = 0 }\n",
            ),
            mapOf("b/L.kt" to "package p\n\nimport android.util.Log\n\nfun log() { Log.d(LONG_TAG, \"m2\") }\n"),
            listOf("b/L.kt"),
            expectFindings = true,
        ),
    )

    private fun run(c: Case) {
        val root = tmp.resolve(c.name).toFile()
        val src = File(root, "src")
        for ((p, t) in c.base) src.writeSource(p, t)
        val session = recordingSession(src, File(root, "snapshots"))
        try {
            val baseResult = session.check(1, kotlinFilesIn(src).map { FileRef(it) }, emptySet())
            assertNotNull(session.incrementalState.snapshot, "${c.name}: the base module must compile cleanly: ${baseResult.errorFiles}")

            for ((p, t) in c.edit) { val f = File(src, p); if (t == null) f.delete() else src.writeSource(p, t) }
            val all = kotlinFilesIn(src)
            val ref = plainSession(src).check(2, all.map { FileRef(it) }, emptySet())

            val slice = c.slice.map { canon(File(src, it).path) }
            val outcome = session.sliceCheck(3, slice.map { FileRef(it) }, emptySet())
            val inc = (outcome as? SliceOutcome.Checked)?.result
                ?: error("${c.name}: the slice fell back: ${(outcome as SliceOutcome.Fallback).reason}")

            for (f in slice) {
                val name = File(f).name
                if (name in c.differs) {
                    assertNotEquals(ref.verdictOf(f).key(), inc.verdictOf(f).key(), "${c.name}: $name was expected to differ on a slice")
                } else {
                    assertEquals(ref.verdictOf(f).key(), inc.verdictOf(f).key(), "${c.name}: $name differs between the full check and the slice")
                }
            }
            assertEquals(c.errors, all.filter { ref.verdictOf(it).error }.map { File(it).name }.toSet(), "${c.name}: files with a compiler error")
            val changedOutside = all.filter { it !in slice && baseResult.verdictOf(it).key() != ref.verdictOf(it).key() }
            assertEquals(c.outside, changedOutside.map { File(it).name }.toSet(), "${c.name}: files outside the slice whose verdict changed")
            if (c.expectFindings) assertTrue(slice.any { ref.verdictOf(it).findings.isNotEmpty() }, "${c.name}: the case should produce findings")
        } finally {
            session.dispose()
        }
    }

    @Test fun internalAcrossFiles() = run(cases.first { it.name == "internal" })

    @Test fun topLevelFacadesAndJvmName() = run(cases.first { it.name == "facade-jvmname" })

    @Test fun multifileClassWithOnePartInSourceAndOneInTheSnapshot() = run(cases.first { it.name == "multifile" })

    @Test fun subclassInSliceOfSealedParentInSnapshot() = run(cases.first { it.name == "sealed-parent-in-snapshot" })

    @Test fun missingBranchOverASealedHierarchyInTheSnapshotIsAnError() = run(cases.first { it.name == "sealed-hierarchy-in-snapshot-when-in-slice" })

    @Test fun newSealedSubclassLeavesTheWhenFileToThePlanner() = run(cases.first { it.name == "sealed-new-subclass-other-file" })

    @Test fun newSealedSubclassWithTheWhenInTheSliceNeedsTheParentToo() {
        run(cases.first { it.name == "sealed-new-subclass-when-in-slice" })
    }

    @Test fun newSealedSubclassWithTheParentInTheSliceMatchesAFullCheck() = run(cases.first { it.name == "sealed-new-subclass-parent-in-slice" })

    @Test fun redeclarationNeedsBothSidesInTheSlice() = run(cases.first { it.name == "redeclaration" })

    @Test fun redeclarationWithBothSidesInTheSliceMatchesAFullCheck() = run(cases.first { it.name == "redeclaration-both-in-slice" })

    @Test fun conflictingOverloadIsSeenFromTheSliceSide() = run(cases.first { it.name == "conflicting-overloads" })

    @Test fun conflictingOverloadWithBothSidesInTheSliceMatchesAFullCheck() = run(cases.first { it.name == "conflicting-overloads-both-in-slice" })

    @Test fun inlineFunctionsAndConstantsFromTheSnapshot() = run(cases.first { it.name == "inline-and-const" })

    @Test fun aRuleThatReadsAConstFromTheSnapshotMatchesAFullCheck() = run(cases.first { it.name == "rule-reads-a-const-from-the-snapshot" })

    // ---- expect/actual stays on the full tier ---------------------------------------------------------------------

    private val expectActualSources = mapOf(
        "common/E.kt" to "package p\n\nexpect fun plat(): String\nexpect class Box() { fun v(): Int }\n",
        "jvm/A.kt" to "package p\n\nactual fun plat(): String = \"jvm\"\nactual class Box actual constructor() { actual fun v(): Int = 1 }\n",
        "jvm/U.kt" to "package p\n\nfun use() = plat() + Box().v()\n",
    )

    @Test fun expectActualInTheSourcesIsAFallbackAndBuildsNoSnapshot() {
        val src = tmp.resolve("expect-actual/src").toFile()
        for ((p, t) in expectActualSources) src.writeSource(p, t)
        val session = recordingSession(src, tmp.resolve("expect-actual/snapshots").toFile())
        try {
            val result = session.check(1, kotlinFilesIn(src).map { FileRef(it) }, emptySet())
            assertTrue(result.crashed.isEmpty(), result.crashed.toString())
            assertNull(session.incrementalState.snapshot)
            assertEquals(SliceFallback.EXPECT_ACTUAL, session.sliceFallback())
            val outcome = session.sliceCheck(2, listOf(FileRef(canon(File(src, "jvm/A.kt").path))), emptySet())
            assertEquals(SliceFallback.EXPECT_ACTUAL, (outcome as SliceOutcome.Fallback).reason)
            assertEquals(1, session.incrementalState.generation, "the fallback compiles nothing")
            assertTrue(File(tmp.resolve("expect-actual/snapshots").toFile(), "fir-snapshots").listFiles().orEmpty().isEmpty())
        } finally {
            session.dispose()
        }
    }

    @Test fun aMultiplatformSourceLayoutIsAFallbackEvenWithoutExpectDeclarations() {
        val common = tmp.resolve("mpp/src/commonMain/kotlin").toFile()
        val jvm = tmp.resolve("mpp/src/jvmMain/kotlin").toFile()
        common.writeSource("p/C.kt", "package p\n\nfun common() = 1\n")
        jvm.writeSource("p/J.kt", "package p\n\nfun jvm() = common()\n")
        val session = AnalysisSession(listOf(common.path, jvm.path), listOf(testStdlib), incrementalRecording = true)
        session.snapshotRootOverride = tmp.resolve("mpp/snapshots").toFile()
        try {
            session.check(1, listOf(FileRef(canon(File(common, "p/C.kt").path)), FileRef(canon(File(jvm, "p/J.kt").path))), emptySet())
            assertEquals(SliceFallback.EXPECT_ACTUAL, session.sliceFallback())
        } finally {
            session.dispose()
        }
    }

    private fun assertNull(value: Any?) = kotlin.test.assertNull(value)
}
