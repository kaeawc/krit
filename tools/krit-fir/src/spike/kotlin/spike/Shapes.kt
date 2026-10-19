package spike

import org.jetbrains.kotlin.cli.common.ExitCode
import java.io.File

/**
 * Probes F2 (slice compile against a same-module binary snapshot through the stock CLI frontend + IC components) on
 * constructs that may behave differently when part of the module comes from class files:
 * `internal`, top-level facades, @JvmName / multifile facades, sealed hierarchies split across source and binary,
 * expect/actual, redeclarations / conflicting overloads that need both sides in source, inline functions, const vals,
 * and krit rules that read facts of declarations in other files.
 *
 * Each case: compile the base state with the backend (snapshot), apply the edit, compile only the slice files from
 * source, and compare their verdicts (and every diagnostic) with a fresh full frontend-only compile of the edited state.
 * A case also reports files outside the slice whose full-compile verdict changed (what a slice cannot see).
 */
class Shapes(val o: Map<String, String>) {
    val dir = File(SCRATCH, "shapes")
    data class Case(
        val name: String, val base: Map<String, String>, val edit: Map<String, String?>, val slice: List<String>,
        val extraArgs: List<String> = emptyList(), val note: String = "",
    )

    val cases = listOf(
        Case("internal", mapOf(
            "a/A.kt" to "package p\n\ninternal fun helper(): Int = 1\ninternal class Hidden { internal val v = 2 }\n",
            "b/B.kt" to "package p\n\nfun use(): Int = helper() + Hidden().v\n",
        ), mapOf("b/B.kt" to "package p\n\nfun use(): Int = helper() + Hidden().v + 1\n"), listOf("b/B.kt")),
        Case("facade-jvmname", mapOf(
            "a/Utils.kt" to "@file:JvmName(\"UtilsX\")\npackage p\n\nfun top(): String = \"t\"\nval topVal: Int = 3\nfun String.ext(): Int = length\n",
            "b/B.kt" to "package p\n\nfun use(): Int = top().ext() + topVal\n",
        ), mapOf("b/B.kt" to "package p\n\nfun use(): Int = top().ext() + topVal + 1\n"), listOf("b/B.kt")),
        Case("multifile", mapOf(
            "a/M1.kt" to "@file:JvmName(\"Multi\")\n@file:JvmMultifileClass\npackage p\n\nfun m1(): Int = 1\n",
            "a/M2.kt" to "@file:JvmName(\"Multi\")\n@file:JvmMultifileClass\npackage p\n\nfun m2(): Int = 2\n",
            "b/B.kt" to "package p\n\nfun use(): Int = m1() + m2()\n",
        ), mapOf("a/M1.kt" to "@file:JvmName(\"Multi\")\n@file:JvmMultifileClass\npackage p\n\nfun m1(): Int = 10\n"), listOf("a/M1.kt", "b/B.kt"),
            note = "M2's part stays binary while M1's part is source"),
        Case("sealed-binary-parent", mapOf(
            "a/S.kt" to "package p\n\nsealed class S\nclass A1 : S()\n",
            "b/B.kt" to "package p\n\nclass B1 : S()\nfun f(s: S): Int = when (s) { is A1 -> 1; is B1 -> 2 }\n",
        ), mapOf("b/B.kt" to "package p\n\nclass B1 : S()\nfun f(s: S): Int = when (s) { is A1 -> 1; is B1 -> 3 }\n"), listOf("b/B.kt"),
            note = "subclass in source, sealed parent in binary: SEALED_INHERITOR_IN_DIFFERENT_MODULE?"),
        Case("sealed-binary-child-when", mapOf(
            "a/S.kt" to "package p\n\nsealed interface T\nclass C1 : T\nclass C2 : T\n",
            "b/W.kt" to "package p\n\nfun g(t: T): Int = when (t) { is C1 -> 1; is C2 -> 2 }\n",
        ), mapOf("b/W.kt" to "package p\n\nfun g(t: T): Int = when (t) { is C1 -> 1 }\n"), listOf("b/W.kt"),
            note = "exhaustiveness over a binary sealed hierarchy (missing branch must be an error)"),
        Case("sealed-add-subclass", mapOf(
            "a/S.kt" to "package p\n\nsealed interface U\nclass D1 : U\n",
            "b/W.kt" to "package p\n\nfun h(u: U): Int = when (u) { is D1 -> 1 }\n",
            "c/D2.kt" to "package p\n\nclass Other\n",
        ), mapOf("c/D2.kt" to "package p\n\nclass Other\nclass D2 : U\n"), listOf("c/D2.kt"),
            note = "new subclass in another file: W.kt's when becomes non-exhaustive; W.kt is outside the slice"),
        Case("sealed-add-subclass-when-in-slice", mapOf(
            "a/S.kt" to "package p\n\nsealed interface U\nclass D1 : U\n",
            "b/W.kt" to "package p\n\nfun h(u: U): Int = when (u) { is D1 -> 1 }\n",
            "c/D2.kt" to "package p\n\nclass Other\n",
        ), mapOf("c/D2.kt" to "package p\n\nclass Other\nclass D2 : U\n"), listOf("c/D2.kt", "b/W.kt"),
            note = "W.kt in the slice too, but the sealed parent's binary still lists only D1"),
        Case("sealed-add-subclass-parent-in-slice", mapOf(
            "a/S.kt" to "package p\n\nsealed interface U\nclass D1 : U\n",
            "b/W.kt" to "package p\n\nfun h(u: U): Int = when (u) { is D1 -> 1 }\n",
            "c/D2.kt" to "package p\n\nclass Other\n",
        ), mapOf("c/D2.kt" to "package p\n\nclass Other\nclass D2 : U\n"), listOf("c/D2.kt", "b/W.kt", "a/S.kt"),
            note = "the sealed parent's file in the slice as well"),
        Case("redeclaration", mapOf(
            "a/A.kt" to "package p\n\nclass Dup\n",
            "b/B.kt" to "package p\n\nclass Other\n",
        ), mapOf("b/B.kt" to "package p\n\nclass Other\nclass Dup\n"), listOf("b/B.kt"),
            note = "both files get the redeclaration error in a full compile; only B is in the slice"),
        Case("conflicting-overloads", mapOf(
            "a/A.kt" to "package p\n\nfun ov(x: Int): Int = x\n",
            "b/B.kt" to "package p\n\nfun other() = 1\n",
        ), mapOf("b/B.kt" to "package p\n\nfun other() = 1\nfun ov(x: Int): Int = x + 1\n"), listOf("b/B.kt")),
        Case("inline-and-const", mapOf(
            "a/A.kt" to "package p\n\nconst val TAG: String = \"a-very-long-log-tag-name-over-23\"\nconst val N: Int = 3\ninline fun <reified T> name(): String = T::class.java.name\ninline fun twice(f: () -> Int): Int = f() + f()\n",
            "b/B.kt" to "package p\n\nfun use(x: Int): Int = when (x) { N -> twice { 1 }; else -> name<String>().length }\nfun tagLen(): Int = TAG.length\n",
        ), mapOf("b/B.kt" to "package p\n\nfun use(x: Int): Int = when (x) { N -> twice { 2 }; else -> name<String>().length }\nfun tagLen(): Int = TAG.length\n"), listOf("b/B.kt")),
        Case("expect-actual", mapOf(
            "common/E.kt" to "package p\n\nexpect fun plat(): String\nexpect class Box() { fun v(): Int }\n",
            "jvm/A.kt" to "package p\n\nactual fun plat(): String = \"jvm\"\nactual class Box actual constructor() { actual fun v(): Int = 1 }\n",
            "jvm/U.kt" to "package p\n\nfun use() = plat() + Box().v()\n",
        ), mapOf("jvm/A.kt" to "package p\n\nactual fun plat(): String = \"jvm2\"\nactual class Box actual constructor() { actual fun v(): Int = 1 }\n"), listOf("jvm/A.kt"),
            extraArgs = listOf("-Xmulti-platform", "commonSources=common/E.kt"), note = "actual in source, expect only in the binary (expects are not compiled to class files)"),
        // krit rules whose verdict may need facts about declarations in other files
        Case("rule-longlogtag-const", mapOf(
            "a/T.kt" to "package p\n\nconst val LONG_TAG: String = \"ThisTagIsLongerThanTwentyThree\"\n",
            "b/L.kt" to "package p\n\nimport android.util.Log\n\nfun log() { Log.d(LONG_TAG, \"m\") }\n",
            "android/util/Log.kt" to "package android.util\n\nobject Log { @JvmStatic fun d(tag: String, msg: String): Int = 0 }\n",
        ), mapOf("b/L.kt" to "package p\n\nimport android.util.Log\n\nfun log() { Log.d(LONG_TAG, \"m2\") }\n"), listOf("b/L.kt"),
            note = "LongLogTag reads a const from another file"),
    )

    fun run() {
        val only = o["case"]
        for (c in cases) if (only == null || c.name == only) try { runCase(c) } catch (t: Throwable) { println("## ${c.name}: harness exception $t"); t.printStackTrace(System.out) }
        dir.deleteRecursively()
    }

    private fun args(files: List<String>, out: File, c: Case, root: File) = kritArgs(files, emptyList(), out).also { a ->
        for (x in c.extraArgs) when {
            x == "-Xmulti-platform" -> a.multiPlatform = true
            x.startsWith("commonSources=") -> a.commonSources = x.removePrefix("commonSources=").split(",").map { File(root, it).path }.toTypedArray()
        }
    }

    private fun runCase(c: Case) {
        val root = File(dir, c.name).also { it.deleteRecursively(); it.mkdirs() }
        val src = File(root, "src")
        for ((p, t) in c.base) File(src, p).also { it.parentFile.mkdirs() }.writeText(t)
        val snapDir = File(root, "snapshot")
        val snap = Snapshot(snapDir, "main")
        val all0 = c.base.keys.map { canon(File(src, it).path) }
        val tmp = File(root, "tmp").also { it.mkdirs() }
        val baseArgs = args(all0, tmp, c, src).also { it.reportOutputFiles = true }
        val base = runCheck(StockPhases.full(null), baseArgs, all0)
        tmp.renameTo(snapDir)
        snap.adopt(base.outputs.mapValues { (_, ps) -> ps.map { it.replace(tmp.path, snapDir.path) } })
        println("## ${c.name}${if (c.note.isNotEmpty()) " (${c.note})" else ""}")
        println("   base: exit=${base.exit} classes=${snap.classFiles} errors=${base.verdicts.filterValues { it.error }.keys.map { File(it).name }}")
        // edit
        for ((p, t) in c.edit) { val f = File(src, p); if (t == null) f.delete() else { f.parentFile.mkdirs(); f.writeText(t) } }
        val all = (c.base.keys + c.edit.filterValues { it != null }.keys).distinct().filter { File(src, it).exists() }.map { canon(File(src, it).path) }
        val slice = c.slice.map { canon(File(src, it).path) }
        val refOut = File(root, "ref").also { it.mkdirs() }
        val ref = runShipped(args(all, refOut, c, src), all, frontendOnly = true)
        snap.park(slice)
        val sliceOut = File(root, "slice").also { it.mkdirs() }
        // --friend=true: the naive variant: the snapshot is a *library* on the classpath with -Xfriend-paths (internal
        // visibility only), not the module's own IC binaries.
        val friend = o["friend"] == "true"
        val sa = args(slice, sliceOut, c, src).also { a ->
            a.classpath = a.classpath + File.pathSeparator + snapDir.path
            if (friend) a.friendPaths = arrayOf(snapDir.path) else a.incrementalCompilation = true
        }
        val lookups = CompileLookups()
        val inc = runCheck(StockPhases.frontend(null), sa, slice, services(lookups, if (friend) null else snap))
        snap.unpark()
        for (f in slice) {
            val r = ref.verdicts[f]; val i = inc.verdicts[f]
            val same = r == i
            println("   slice ${File(f).name}: ${if (same) "IDENTICAL to full compile" else "DIFFERS"} (full: error=${r?.error} findings=${r?.findings?.size}; slice: error=${i?.error} findings=${i?.findings?.size})")
            if (!same) {
                (r?.diags.orEmpty() - i?.diags.orEmpty().toSet()).forEach { println("      only full : ${it.take(200)}") }
                (i?.diags.orEmpty() - r?.diags.orEmpty().toSet()).forEach { println("      only slice: ${it.take(200)}") }
            }
        }
        // files outside the slice whose verdict changed with the edit: the slice cannot report these
        val outside = all.filter { it !in slice && base.verdicts[it] != ref.verdicts[it] }
        if (outside.isNotEmpty()) for (f in outside) {
            println("   outside-slice file ${File(f).name} changed in the full compile: ${(ref.verdicts[f]?.diags.orEmpty() - base.verdicts[f]?.diags.orEmpty().toSet()).map { it.take(160) }}")
            val looked = lookups.byFile.keys
            println("      (lookups recorded for slice files: ${looked.size}; would the lookup index flag it? see Inc)")
        }
        if (inc.exit == ExitCode.INTERNAL_ERROR) println("   slice compile crashed: ${inc.crashed ?: inc.firstError}")
    }
}
