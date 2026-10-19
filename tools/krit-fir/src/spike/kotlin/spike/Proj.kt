package spike

import java.io.File
import java.util.Random

/** A module of the project model (kept from the AA spike's harness; the FIR-only harness compiles one module). */
class ModSpec(val name: String, val roots: List<String>, val deps: List<String> = emptyList(), val files: List<String>? = null)

/** What the incremental harness needs from a project under test: file texts, specs, and an edit generator. */
interface Proj {
    val texts: MutableMap<String, String>
    val rnd: Random
    val classpath: List<String>
    val sourceDirs: List<String>
    fun specs(slice: Set<String>?): List<ModSpec>
    fun nextEdit(kinds: List<String>?): Gen.Edit
    fun describe(): String
}

/**
 * A real Kotlin tree copied to scratch (so edits never touch the original). One module. Edit kinds:
 *  - literal: change a string literal inside a body (body-only edit)
 *  - comment: append a comment line
 *  - rename: rename a declared function that another file mentions (signature edit; callers break), toggled back later
 *  - add / delete: a new trivial top-level function file in an existing package / removal of an added file
 */
class RealProj(srcRoots: List<String>, override val classpath: List<String>, copyTo: File, seed: Long = 5L) : Proj {
    override val rnd = Random(seed)
    override val texts = LinkedHashMap<String, String>()
    private val roots: List<File>
    override val sourceDirs: List<String> get() = roots.map { it.path }
    private val original = HashMap<String, String>() // path -> text before rename
    private val added = ArrayList<String>()
    private var seq = 0
    private val nameFiles = HashMap<String, MutableSet<String>>() // identifier -> files mentioning it
    private val fnRe = Regex("^(\\s*)((?:(?:public|internal|protected|open|final|suspend|inline|operator|infix|tailrec)\\s+)*)fun\\s+(?:<[^>]*>\\s*)?([a-z][A-Za-z0-9_]*)\\(")
    private val litRe = Regex("\"([A-Za-z0-9 _.,:/-]{3,40})\"")

    init {
        val rs = ArrayList<File>()
        for ((i, r) in srcRoots.withIndex()) {
            val dst = File(copyTo, "m$i"); dst.deleteRecursively(); dst.mkdirs()
            File(r).walkTopDown().onEnter { it.name !in prunedDirs }.filter { it.isFile && it.extension == "kt" }.forEach { f ->
                val out = File(dst, f.relativeTo(File(r)).path); out.parentFile.mkdirs(); f.copyTo(out)
            }
            rs += dst
        }
        roots = rs
        for (r in roots) r.walkTopDown().filter { it.isFile && it.extension == "kt" }.forEach { texts[it.path] = it.readText() }
        val id = Regex("[A-Za-z_][A-Za-z0-9_]*")
        for ((p, t) in texts) for (m in id.findAll(t)) nameFiles.getOrPut(m.value) { HashSet() }.add(p)
    }

    override fun describe() = "real: ${texts.size} files, ${texts.values.sumOf { it.lines().size }} lines, roots=${roots.map { it.path }}"

    override fun specs(slice: Set<String>?): List<ModSpec> =
        if (slice == null) listOf(ModSpec("main", roots.map { it.path })) else listOf(ModSpec("main", emptyList(), files = slice.sorted()))

    private fun put(path: String, text: String) { texts[path] = text; File(path).parentFile.mkdirs(); File(path).writeText(text) }

    private fun pick(): String { val pool = System.getenv("SPIKE_POOL")?.toIntOrNull(); val ks = texts.keys.toList().let { if (pool != null) it.take(pool) else it }; return ks[rnd.nextInt(ks.size)] }

    override fun nextEdit(kinds: List<String>?): Gen.Edit {
        val roll = rnd.nextInt(100)
        val kind = kinds?.get(rnd.nextInt(kinds.size)) ?: when {
            roll < 45 -> "literal"
            roll < 55 -> "comment"
            roll < 80 -> "rename"
            roll < 92 -> "add"
            else -> "delete"
        }
        repeat(50) {
            val e = tryEdit(kind)
            if (e != null) return e
        }
        val p = pick(); put(p, texts[p]!! + "// edit ${seq++}\n"); return Gen.Edit("comment", listOf(p))
    }

    private fun tryEdit(kind: String): Gen.Edit? {
        when (kind) {
            "comment" -> { val p = pick(); put(p, texts[p]!! + "// edit ${seq++}\n"); return Gen.Edit(kind, listOf(p)) }
            "literal" -> {
                val p = pick(); val lines = texts[p]!!.lines().toMutableList()
                val idxs = lines.indices.filter { i -> val l = lines[i].trim(); !l.startsWith("import") && !l.startsWith("package") && !l.startsWith("@") && !l.startsWith("//") && !l.startsWith("*") && !l.contains("\"\"\"") && !l.contains("$") && litRe.containsMatchIn(l) && l.count { it == '"' } == 2 }
                if (idxs.isEmpty()) return null
                val i = idxs[rnd.nextInt(idxs.size)]
                val m = litRe.find(lines[i])!!; lines[i] = lines[i].replaceRange(m.range, "\"${m.groupValues[1]}~\"")
                put(p, lines.joinToString("\n")); return Gen.Edit(kind, listOf(p))
            }
            "rename" -> {
                val p = pick()
                original[p]?.let { orig -> put(p, orig); original.remove(p); return Gen.Edit(kind, listOf(p)) } // toggle back
                val lines = texts[p]!!.lines().toMutableList()
                val idxs = lines.indices.filter { i ->
                    val m = fnRe.find(lines[i]) ?: return@filter false
                    val name = m.groupValues[3]
                    !lines[i].contains("override") && name != "main" && (nameFiles[name]?.count { it != p } ?: 0) >= 1
                }
                if (idxs.isEmpty()) return null
                val i = idxs[rnd.nextInt(idxs.size)]
                val name = fnRe.find(lines[i])!!.groupValues[3]
                original[p] = texts[p]!!
                lines[i] = lines[i].replaceFirst("fun $name(", "fun ${name}Zz(").replaceFirst("fun <", "fun <").let { if (it == lines[i]) lines[i].replaceFirst(Regex("(fun\\s+(?:<[^>]*>\\s*)?)$name\\("), "$1${name}Zz(") else it }
                put(p, lines.joinToString("\n")); return Gen.Edit(kind, listOf(p))
            }
            "add" -> {
                val sibling = pick()
                val pkg = Regex("(?m)^package\\s+([\\w.]+)").find(texts[sibling]!!)?.groupValues?.get(1)
                val np = File(File(sibling).parentFile, "ZzNew${seq}.kt").path
                put(np, (if (pkg != null) "package $pkg\n\n" else "") + "fun zzNew${seq++}(): Int = 1\n")
                added += np; nameFiles.getOrPut("zzNew") { HashSet() }.add(np)
                return Gen.Edit(kind, emptyList(), added = listOf(np))
            }
            "delete" -> {
                if (added.isEmpty()) return null
                val p = added.removeAt(rnd.nextInt(added.size)); texts.remove(p); File(p).delete()
                return Gen.Edit(kind, emptyList(), deleted = listOf(p))
            }
        }
        return null
    }

    companion object {
        val prunedDirs = setOf(".git", "build", ".gradle", "node_modules", ".idea")
    }
}

fun makeProj(o: Map<String, String>, dir: File, n: Int, seed: Long): Proj {
    val real = o["real"] ?: return Gen(dir, n, seed).also { it.generate() }
    val cp = o["cp"]?.let { f -> File(f).readText().split(":").filter { it.endsWith(".jar") } } ?: emptyList()
    return RealProj(real.split(","), cp, dir, seed)
}
