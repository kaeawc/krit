package dev.jasonpearson.krit.fir.incremental

import java.io.File
import java.util.Random

/**
 * A synthetic Kotlin module held as an in-memory model that is mirrored to disk, ported from the incremental spike's
 * generator. A quarter of the files are `lib` objects and the rest `app` objects that use three of them each,
 * chain to a neighbour and call `println`; a sealed `Shape` hierarchy in `lib` has a `when` user in `app`.
 * Edits mutate the model and the disk and report the files they touched, so a harness can run them like a watcher.
 *
 * Edit kinds: body, comment, sig (a return type), overload, add (a new file), delete (an original or an added file;
 * deleting an original leaves its chain successor with an unresolved import), shadow (a file declaring `println` in
 * a package, or its removal), sealed (a new `Shape` subclass), hot (a signature edit of a file every app file in
 * the first [hotDependents] uses), rename (a lib function renamed together with its callers) and renameBreak /
 * renameRepair (the declaration renamed alone, which breaks its callers, and then the callers following).
 */
internal class SyntheticProject(val root: File, val total: Int, seed: Long = 7L, val hotDependents: Int = 0) {
    val libDir = File(root, "lib")
    val appDir = File(root, "app")
    val rnd = Random(seed)
    val libCount = (total * 0.25).toInt()
    val appCount = total - libCount

    /** Absolute path -> text of every file of the module. */
    val texts = LinkedHashMap<String, String>()
    private var nextApp = appCount
    private val addedApps = ArrayList<Int>()
    private val shadowFiles = ArrayList<String>()
    private val deletedOriginals = HashSet<Int>()
    private val sigState = HashMap<Int, Boolean>() // lib index -> returns Long
    private val overloadState = HashMap<Int, Boolean>()
    private val renamed = HashSet<Int>() // lib index -> function renamed (callers follow)
    private val declarationRenamed = HashSet<Int>() // renamed in the declaration only
    private val bodyState = HashMap<Int, Pair<Int, Boolean>>()
    private var hotLong = false
    private var shadowSeq = 0
    private var shapeExtra = 0

    val sourceDirs: List<String> get() = listOf(libDir.path, appDir.path)

    fun libPath(i: Int) = File(libDir, "p${i % 20}/Lib$i.kt").path
    fun appPath(j: Int) = File(appDir, "p${j % 20}/App$j.kt").path

    private fun libText(i: Int): String {
        if (i == 0) return hotText()
        val long = sigState[i] == true
        val ov = overloadState[i] == true
        val f = if (i in renamed || i in declarationRenamed) "f${i}r" else "f$i"
        val sb = StringBuilder()
        sb.append("package lib.p${i % 20}\n\nobject Lib$i {\n")
        sb.append("    fun $f(x: Number): ${if (long) "Long" else "Int"} = x.to${if (long) "Long" else "Int"}() + $i\n")
        if (ov) sb.append("    fun $f(x: Int): Long = x.toLong() + $i\n")
        sb.append("    val c$i: String = \"lib$i\"\n")
        sb.append("    fun g$i(v: Int): Int { return v * ${i % 7 + 1} }\n")
        if (i % 40 == 0) sb.append("    fun noisy$i() { println(\"lib $i\") }\n")
        if (i % 60 == 0) sb.append("    fun trace$i() { try { g$i(1) } catch (e: Exception) { e.printStackTrace() } }\n")
        sb.append("}\n\nclass Data$i(val a: Int) {\n    fun twice(): Int = a * 2\n}\n")
        return sb.toString()
    }

    private fun hotText(): String {
        val t = if (hotLong) "Long" else "Int"
        return "package lib.p0\n\nobject Hot {\n    fun hot(x: Int): $t = ${if (hotLong) "x.toLong()" else "x"} + 1\n}\n"
    }

    private fun refs(j: Int): IntArray {
        val l = maxOf(1, libCount - 1)
        val rr = Random(j * 2654435761L + 12345L)
        val r = intArrayOf(1 + rr.nextInt(l), 1 + rr.nextInt(l), 1 + rr.nextInt(l))
        for (k in 1..2) while ((0 until k).any { r[it] == r[k] }) r[k] = 1 + r[k] % l
        return r
    }

    /** The callers of lib file [i]'s function: app files whose first reference is [i], by index. */
    private fun callersOf(i: Int): List<Int> =
        (0 until nextApp).filter { appPath(it) in texts && refs(it)[0] == i }

    private fun appText(j: Int, body: Int = 3, noisy: Boolean = j % 25 == 0): String {
        val (a, b, c) = refs(j).toList()
        val m = if (j > 0 && j % 3 != 0 && (j - 1) !in deletedOriginals) j - 1 else -1
        // The declaration was renamed alone: this file still calls the old name until the repair.
        val fa = if (a in renamed) "f${a}r" else "f$a"
        val hot = j < hotDependents
        val sb = StringBuilder()
        sb.append("package app.p${j % 20}\n\n")
        sb.append("import lib.p${a % 20}.Lib$a\nimport lib.p${b % 20}.Lib$b\nimport lib.p${c % 20}.Lib$c\n")
        if (hot) sb.append("import lib.p0.Hot\n")
        if (m >= 0) sb.append("import app.p${m % 20}.App$m\n")
        sb.append("\nobject App$j {\n")
        sb.append("    val r1: Int = Lib$a.$fa($j)\n")
        sb.append("    val r2: String = Lib$b.c$b\n")
        sb.append("    fun use(x: Int): Int = Lib$c.g$c(x) + r1\n")
        sb.append("    fun derived() = Lib$a.$fa(${j + 1})\n")
        if (m >= 0) sb.append("    fun chain(): Int = App$m.derived()\n")
        if (hot) sb.append("    fun hotUse(): Int = Hot.hot($j)\n")
        sb.append("    fun body(): Int { val t = $body; return t * 3 }\n")
        if (noisy) sb.append("    fun log() { println(\"app $j\") }\n")
        sb.append("}\n")
        return sb.toString()
    }

    private fun rewriteApp(j: Int) {
        val (v, noisy) = bodyState[j] ?: (3 to (j % 25 == 0))
        put(appPath(j), appText(j, v, noisy))
    }

    fun generate() {
        libDir.mkdirs(); appDir.mkdirs()
        for (i in 0 until libCount) put(libPath(i), libText(i))
        for (j in 0 until appCount) put(appPath(j), appText(j))
        put(File(libDir, "p1/Shape.kt").path, shapeText(0))
        put(File(appDir, "p1/ShapeUser.kt").path, shapeUserText())
        put(absPath, absText())
        put(absImplPath, absImplText())
    }

    private val absPath = File(libDir, "p3/Abs.kt").path
    private val absImplPath = File(appDir, "p3/AbsImpl.kt").path
    private var absLib = 0
    private var absImpl = 0
    private var redeclStage = 0

    private fun absText() = "package lib.p3\n\nabstract class Abs {\n    abstract fun m$absLib(): Int\n}\n"
    private fun absImplText() = "package app.p3\n\nimport lib.p3.Abs\n\nclass AbsImpl : Abs() {\n    override fun m$absImpl(): Int = 1\n}\n"

    private fun shapeText(extra: Int): String {
        val sb = StringBuilder("package lib.p1\n\nsealed interface Shape\nclass Circle : Shape\nclass Square : Shape\n")
        for (k in 0 until extra) sb.append("class Extra$k : Shape\n")
        return sb.toString()
    }

    private fun shapeUserText() =
        "package app.p1\n\nimport lib.p1.Shape\nimport lib.p1.Circle\nimport lib.p1.Square\n\n" +
            "object ShapeUser {\n    fun area(s: Shape): Int = when (s) {\n        is Circle -> 1\n        is Square -> 2\n    }\n}\n"

    private fun put(path: String, text: String) {
        texts[path] = text
        val f = File(path); f.parentFile.mkdirs(); f.writeText(text)
    }

    private fun remove(path: String) {
        texts.remove(path); File(path).delete()
    }

    class Edit(val kind: String, val changed: List<String>, val added: List<String> = emptyList(), val deleted: List<String> = emptyList())

    private fun liveApp(): Int {
        while (true) {
            val j = rnd.nextInt(appCount)
            if (j !in deletedOriginals && appPath(j) in texts) return j
        }
    }

    private fun liveLib(): Int {
        while (true) {
            val i = 1 + rnd.nextInt(libCount - 1)
            if (i !in renamed && i !in declarationRenamed) return i
        }
    }

    fun apply(kind: String): Edit {
        when (kind) {
            "body" -> {
                val j = liveApp()
                val (v, noisy) = bodyState[j] ?: (3 to (j % 25 == 0))
                val nn = if (rnd.nextInt(4) == 0) !noisy else noisy
                bodyState[j] = (v + 1) to nn
                put(appPath(j), appText(j, v + 1, nn))
                return Edit(kind, listOf(appPath(j)))
            }
            "comment" -> {
                val j = liveApp()
                val p = appPath(j)
                put(p, texts[p]!! + "// edit ${rnd.nextInt()}\n")
                return Edit(kind, listOf(p))
            }
            "sig" -> {
                val i = liveLib()
                sigState[i] = sigState[i] != true
                put(libPath(i), libText(i))
                return Edit(kind, listOf(libPath(i)))
            }
            "hot" -> {
                hotLong = !hotLong
                put(libPath(0), libText(0))
                return Edit(kind, listOf(libPath(0)))
            }
            "overload" -> {
                val i = liveLib()
                overloadState[i] = overloadState[i] != true
                put(libPath(i), libText(i))
                return Edit(kind, listOf(libPath(i)))
            }
            "add" -> {
                val j = nextApp++
                addedApps += j
                put(appPath(j), appText(j))
                return Edit(kind, emptyList(), added = listOf(appPath(j)))
            }
            "delete", "deleteBreak" -> {
                if (kind == "delete" && addedApps.isNotEmpty() && rnd.nextBoolean()) {
                    val j = addedApps.removeAt(rnd.nextInt(addedApps.size))
                    remove(appPath(j))
                    return Edit(kind, emptyList(), deleted = listOf(appPath(j)))
                }
                // A referenced original: its chain successor imports it.
                val j = liveApp().let { if (it % 3 == 2) it - 1 else it }.coerceAtLeast(1)
                if (j in deletedOriginals || appPath(j) !in texts) return apply("body")
                deletedOriginals += j
                remove(appPath(j))
                val successor = j + 1
                if (successor % 3 != 0 && appPath(successor) in texts) {
                    // "delete" follows up in the same edit; "deleteBreak" leaves the successor's import unresolved.
                    if (kind == "delete") {
                        rewriteApp(successor)
                        return Edit(kind, listOf(appPath(successor)), deleted = listOf(appPath(j)))
                    }
                    pendingSuccessor = successor
                }
                return Edit(kind, emptyList(), deleted = listOf(appPath(j)))
            }
            "deleteRepair" -> {
                val successor = pendingSuccessor ?: return apply("body")
                pendingSuccessor = null
                rewriteApp(successor)
                return Edit(kind, listOf(appPath(successor)))
            }
            "shadow" -> {
                if (shadowFiles.isNotEmpty() && rnd.nextBoolean()) {
                    val p = shadowFiles.removeAt(rnd.nextInt(shadowFiles.size))
                    remove(p)
                    return Edit(kind, emptyList(), deleted = listOf(p))
                }
                val pk = rnd.nextInt(20)
                val p = File(appDir, "p$pk/Shadow${shadowSeq++}.kt").path
                shadowFiles += p
                put(p, "package app.p$pk\n\nfun println(message: String) { }\n")
                return Edit(kind, emptyList(), added = listOf(p))
            }
            "sealed" -> {
                shapeExtra++
                val p = File(libDir, "p1/Shape.kt").path
                put(p, shapeText(shapeExtra))
                return Edit(kind, listOf(p))
            }
            "redeclare" -> {
                val pkg = File(appDir, "p2")
                val a = File(pkg, "RedeclA.kt").path
                val b = File(pkg, "RedeclB.kt").path
                val text = "package app.p2\n\nfun redecl(): Int = 1\n"
                val stage = redeclStage
                redeclStage = (redeclStage + 1) % 4
                return when (stage) {
                    0 -> { put(a, text); Edit(kind, emptyList(), added = listOf(a)) }
                    1 -> { put(b, text); Edit(kind, emptyList(), added = listOf(b)) }
                    2 -> { remove(b); Edit(kind, emptyList(), deleted = listOf(b)) }
                    else -> { remove(a); Edit(kind, emptyList(), deleted = listOf(a)) }
                }
            }
            "abstractBreak" -> {
                absLib++
                put(absPath, absText())
                return Edit(kind, listOf(absPath))
            }
            "abstractRepair" -> {
                absImpl = absLib
                put(absImplPath, absImplText())
                return Edit(kind, listOf(absImplPath))
            }
            "multi" -> {
                // An app file whose chain predecessor's inferred type moves with a signature edit of a lib file,
                // edited in the same request: the file is compiled first, its dependency only changes a wave later.
                repeat(50) {
                    val j = liveApp()
                    val pred = j - 1
                    if (j % 3 == 0 || j < 1 || pred in deletedOriginals || appPath(pred) !in texts) return@repeat
                    val i = refs(pred)[0]
                    if (i in renamed || i in declarationRenamed || i == 0) return@repeat
                    sigState[i] = sigState[i] != true
                    put(libPath(i), libText(i))
                    val (v, noisy) = bodyState[j] ?: (3 to (j % 25 == 0))
                    bodyState[j] = (v + 1) to noisy
                    put(appPath(j), appText(j, v + 1, noisy))
                    return Edit(kind, listOf(libPath(i), appPath(j)))
                }
                return apply("body")
            }
            "rename" -> {
                val i = liveLib()
                renamed += i
                put(libPath(i), libText(i))
                val callers = callersOf(i)
                for (j in callers) rewriteApp(j)
                return Edit(kind, listOf(libPath(i)) + callers.map { appPath(it) })
            }
            "renameBreak" -> {
                val i = liveLib()
                declarationRenamed += i
                put(libPath(i), libText(i))
                pendingRepair = i
                return Edit(kind, listOf(libPath(i)))
            }
            "renameRepair" -> {
                val i = pendingRepair ?: return apply("rename")
                pendingRepair = null
                declarationRenamed -= i
                renamed += i
                val callers = callersOf(i)
                for (j in callers) rewriteApp(j)
                return Edit(kind, callers.map { appPath(it) })
            }
        }
        error("unknown kind $kind")
    }

    private var pendingRepair: Int? = null
    private var pendingSuccessor: Int? = null

    /** True between a renameBreak and its repair: the module has unresolved references. */
    val brokenByRename: Boolean get() = pendingRepair != null || pendingSuccessor != null

    /** The files that exist now, absolute. */
    fun files(): List<String> = texts.keys.toList()
}
