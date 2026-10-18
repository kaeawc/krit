package spike

import java.io.File
import java.util.Random

/**
 * Synthetic two-module project (lib <- app) held as an in-memory model that is
 * mirrored to disk. Edits mutate the model and the disk and report the changed
 * paths, so a harness can feed them to a runner exactly like a file watcher.
 */
class Gen(val root: File, val total: Int, seed: Long = 7L, val hotDependents: Int = 500) {
    val libDir = File(root, "lib/src")
    val appDir = File(root, "app/src")
    val rnd = Random(seed)
    val libCount = (total * 0.25).toInt()
    val appCount = total - libCount
    val texts = LinkedHashMap<String, String>() // abs path -> text (all files)
    private var nextApp = appCount
    private val addedApps = ArrayList<Int>()
    private val shadowFiles = ArrayList<String>()
    private val deletedOriginals = HashSet<Int>()
    private val sigState = HashMap<Int, Boolean>() // lib index -> returns Long
    private val overloadState = HashMap<Int, Boolean>()
    var hotLong = false
    var shadowSeq = 0

    fun libPath(i: Int) = File(libDir, "p${i % 20}/Lib$i.kt").path
    fun appPath(j: Int) = File(appDir, "p${j % 20}/App$j.kt").path

    private fun libText(i: Int): String {
        if (i == 0) return hotText()
        val long = sigState[i] == true
        val ov = overloadState[i] == true
        val sb = StringBuilder()
        sb.append("package lib.p${i % 20}\n\nobject Lib$i {\n")
        sb.append("    fun f$i(x: Number): ${if (long) "Long" else "Int"} = x.to${if (long) "Long" else "Int"}() + $i\n")
        if (ov) sb.append("    fun f$i(x: Int): Long = x.toLong() + $i\n")
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
        val r = intArrayOf(1 + (j * 7) % l, 1 + (j * 13 + 5) % l, 1 + (j * 29 + 11) % l)
        for (k in 1..2) while ((0 until k).any { r[it] == r[k] }) r[k] = 1 + r[k] % l
        return r
    }

    private fun appText(j: Int, body: Int = 3, noisy: Boolean = j % 25 == 0, extra: String = ""): String {
        val (a, b, c) = refs(j).toList()
        val m = if (j > 0 && j % 3 != 0 && (j - 1) !in deletedOriginals) j - 1 else -1
        val hot = j < hotDependents
        val sb = StringBuilder()
        sb.append("package app.p${j % 20}\n\n")
        sb.append("import lib.p${a % 20}.Lib$a\nimport lib.p${b % 20}.Lib$b\nimport lib.p${c % 20}.Lib$c\n")
        if (hot) sb.append("import lib.p0.Hot\n")
        if (m >= 0) sb.append("import app.p${m % 20}.App$m\n")
        sb.append("\nobject App$j {\n")
        sb.append("    val r1: Int = Lib$a.f$a($j)\n")
        sb.append("    val r2: String = Lib$b.c$b\n")
        sb.append("    fun use(x: Int): Int = Lib$c.g$c(x) + r1\n")
        sb.append("    fun derived() = Lib$a.f$a(${j + 1})\n")
        if (m >= 0) sb.append("    fun chain(): Int = App$m.derived()\n")
        if (hot) sb.append("    fun hotUse(): Int = Hot.hot($j)\n")
        sb.append("    fun body(): Int { val t = $body; return t * 3 }\n")
        if (noisy) sb.append("    fun log() { println(\"app $j\") }\n")
        sb.append(extra)
        sb.append("}\n")
        return sb.toString()
    }

    private val bodyState = HashMap<Int, Pair<Int, Boolean>>()

    fun generate() {
        libDir.mkdirs(); appDir.mkdirs()
        for (i in 0 until libCount) put(libPath(i), libText(i))
        for (j in 0 until appCount) put(appPath(j), appText(j))
        // a sealed hierarchy exercised by edits that add a subclass
        put(File(libDir, "p1/Shape.kt").path, shapeText(0))
        put(File(appDir, "p1/ShapeUser.kt").path, shapeUserText())
    }

    var shapeExtra = 0
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

    data class Edit(val kind: String, val changed: List<String>, val added: List<String> = emptyList(), val deleted: List<String> = emptyList())

    private fun liveApp(): Int {
        while (true) {
            val j = rnd.nextInt(appCount)
            if (j !in deletedOriginals && appPath(j) in texts) return j
        }
    }

    fun nextEdit(kinds: List<String>? = null): Edit {
        val roll = rnd.nextInt(100)
        val kind = kinds?.get(rnd.nextInt(kinds.size)) ?: when {
            roll < 35 -> "body"
            roll < 55 -> "sig"
            roll < 63 -> "overload"
            roll < 73 -> "add"
            roll < 83 -> "delete"
            roll < 89 -> "shadow"
            roll < 94 -> "sealed"
            roll < 98 -> "comment"
            else -> "hot"
        }
        return apply(kind)
    }

    fun apply(kind: String): Edit {
        when (kind) {
            "body" -> {
                val j = liveApp()
                val (v, noisy) = bodyState[j] ?: (3 to (j % 25 == 0))
                val flipNoisy = rnd.nextInt(4) == 0
                val nn = if (flipNoisy) !noisy else noisy
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
                val i = 1 + rnd.nextInt(libCount - 1)
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
                val i = 1 + rnd.nextInt(libCount - 1)
                overloadState[i] = overloadState[i] != true
                put(libPath(i), libText(i))
                return Edit(kind, listOf(libPath(i)))
            }
            "add" -> {
                val j = nextApp++
                addedApps += j
                put(appPath(j), appText(j))
                // sometimes the new file also becomes a dependency of an existing one
                return Edit(kind, emptyList(), added = listOf(appPath(j)))
            }
            "delete" -> {
                if (addedApps.isNotEmpty() && rnd.nextBoolean()) {
                    val j = addedApps.removeAt(rnd.nextInt(addedApps.size))
                    remove(appPath(j))
                    return Edit(kind, emptyList(), deleted = listOf(appPath(j)))
                }
                // a referenced original: its chain successor loses an import target
                val j = liveApp().let { if (it % 3 == 2) it - 1 else it }.coerceAtLeast(1)
                if (j in deletedOriginals || appPath(j) !in texts) return apply("body")
                deletedOriginals += j
                remove(appPath(j))
                // successor now fails to resolve App$j; regenerate it without the chain import would hide the
                // error, so leave it untouched: the unresolved import is the signal.
                return Edit(kind, emptyList(), deleted = listOf(appPath(j)))
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
        }
        error("unknown kind $kind")
    }
}
