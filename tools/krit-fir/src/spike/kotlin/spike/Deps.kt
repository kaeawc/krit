package spike

/**
 * Lexical dependents index (spike approximation of what the Go side already has
 * in the scanner cross-file index). Facts per file: identifier tokens and
 * declarations with a "header" (the part of a declaration that other files can
 * observe: for declarations without an explicit type the whole initializer).
 *
 * A file F depends on a changed item (owner, name) when F mentions `name` and,
 * for members, also mentions `owner`. Declarations with inferred types make the
 * closure transitive: a dependent that declares them may expose a changed type
 * to its own dependents.
 */
class DepIndex {
    data class Decl(val owner: String?, val name: String, val header: String, val inferred: Boolean, val supers: List<String>, val sealed: Boolean = false)
    class Facts(val idents: Set<String>, val decls: List<Decl>)

    val facts = HashMap<String, Facts>()
    private val inverted = HashMap<String, MutableSet<String>>()

    fun put(path: String, text: String) {
        remove(path)
        val f = analyze(text)
        facts[path] = f
        indexDecls(path, f, true)
        for (t in f.idents) inverted.getOrPut(t) { HashSet() }.add(path)
    }

    fun remove(path: String) {
        val old = facts.remove(path) ?: return
        indexDecls(path, old, false)
        for (t in old.idents) inverted[t]?.remove(path)
    }

    /** Items whose observable shape differs between [old] and [new] facts (either may be null). */
    fun changedItems(old: Facts?, new: Facts?): Set<Pair<String?, String>> {
        val a = old?.decls.orEmpty().groupBy { it.owner to it.name }.mapValues { e -> e.value.map { it.header }.sorted() }
        val b = new?.decls.orEmpty().groupBy { it.owner to it.name }.mapValues { e -> e.value.map { it.header }.sorted() }
        val out = HashSet<Pair<String?, String>>()
        for (k in a.keys + b.keys) if (a[k] != b[k]) out += k
        // new/removed/changed class headers expose their supertypes (exhaustive `when`, abstract members, ...)
        val supers = (old?.decls.orEmpty() + new?.decls.orEmpty()).filter { (it.owner to it.name) in out }.flatMap { it.supers }
        for (s in supers) out += (null to s)
        return out
    }

    /** Files (other than [exclude]) that observe any of [items], closed over inferred-type exposure. */
    fun dependents(items: Set<Pair<String?, String>>, exclude: Set<String>, closure: Boolean = true): Set<String> {
        val result = LinkedHashSet<String>()
        val seen = HashSet<Pair<String?, String>>()
        val work = ArrayDeque<Pair<String?, String>>()
        items.forEach { if (seen.add(it)) work.add(it) }
        while (work.isNotEmpty()) {
            val (owner, name) = work.removeFirst()
            val cands = inverted[name] ?: continue
            for (p in cands) {
                if (p in exclude) continue
                val f = facts[p] ?: continue
                if (owner != null && owner !in f.idents) continue
                if (!result.add(p)) continue
                if (closure) for (d in f.decls) if (d.inferred) {
                    val it = d.owner to d.name
                    if (seen.add(it)) work.add(it)
                }
            }
        }
        return result
    }

    /** Files declaring top-level name [n] (owner == null). */
    private val topLevel = HashMap<String, MutableSet<String>>()
    private val subtypesOf = HashMap<String, MutableSet<String>>()

    private fun indexDecls(path: String, f: Facts, add: Boolean) {
        for (d in f.decls) {
            if (d.owner == null) { val s = topLevel.getOrPut(d.name) { HashSet() }; if (add) s.add(path) else s.remove(path) }
            for (sup in d.supers) { val s = subtypesOf.getOrPut(sup) { HashSet() }; if (add) s.add(path) else s.remove(path) }
        }
    }

    /**
     * Files [seeds] need in order to resolve: top-level declarations whose names they mention, transitively, plus
     * the files declaring subtypes of any sealed type pulled in (exhaustiveness). Conservative by name.
     */
    fun forwardClosure(seeds: Collection<String>): Set<String> {
        val out = LinkedHashSet<String>()
        val work = ArrayDeque<String>()
        for (s in seeds) if (facts.containsKey(s) && out.add(s)) work.add(s)
        while (work.isNotEmpty()) {
            val f = facts[work.removeFirst()] ?: continue
            for (id in f.idents) topLevel[id]?.forEach { if (out.add(it)) work.add(it) }
            for (d in f.decls) if (d.sealed) subtypesOf[d.name]?.forEach { if (out.add(it)) work.add(it) }
        }
        return out
    }

    companion object {
        private val ident = Regex("[A-Za-z_][A-Za-z0-9_]*")
        private val declRe = Regex(
            "^\\s*(?:(?:public|private|internal|protected|override|open|abstract|final|sealed|data|enum|annotation|inline|operator|infix|suspend|const|lateinit|tailrec|external|expect|actual)\\s+)*" +
                "(fun|val|var|class|object|interface|typealias)\\s+(?:<[^>]*>\\s*)?(?:[A-Za-z0-9_<>?,. ]+\\.)?([A-Za-z_][A-Za-z0-9_]*)(.*)$",
        )

        fun analyze(text: String): Facts {
            val idents = HashSet<String>()
            val decls = ArrayList<Decl>()
            var owner: String? = null
            var depth = 0
            var ownerDepth = -1
            for (line in text.lineSequence()) {
                val code = line.substringBefore("//")
                ident.findAll(code).forEach { idents += it.value }
                val m = declRe.find(code)
                if (m != null) {
                    val kw = m.groupValues[1]
                    val name = m.groupValues[2]
                    val rest = m.groupValues[3]
                    val isType = kw == "class" || kw == "object" || kw == "interface"
                    val own = if (isType) (if (depth == 0) null else owner) else owner
                    val inferred: Boolean
                    var header: String
                    if (kw == "fun") {
                        val explicit = Regex("\\)\\s*:").containsMatchIn(rest)
                        inferred = !explicit && rest.contains("=")
                        header = if (explicit) rest.substringBefore("=").substringBefore("{") else if (inferred) rest else rest.substringBefore("{")
                    } else if (kw == "val" || kw == "var") {
                        val explicit = Regex("^\\s*:").containsMatchIn(rest)
                        inferred = !explicit && rest.contains("=")
                        header = if (explicit) rest.substringBefore("=") else rest
                    } else {
                        inferred = false
                        header = rest.substringBefore("{")
                    }
                    header = kw + " " + header.trim().replace(Regex("\\s+"), " ")
                    val supers = if (isType && header.contains(":")) {
                        ident.findAll(header.substringAfter(":")).map { it.value }.toList()
                    } else emptyList()
                    decls += Decl(own, name, header, inferred, supers, Regex("\\bsealed\\b").containsMatchIn(code.substringBefore(kw)))
                    if (isType && code.contains("{") && depth == 0) { owner = name; ownerDepth = 0 }
                }
                for (c in code) { if (c == '{') depth++ else if (c == '}') depth-- }
                if (owner != null && depth <= ownerDepth) { owner = null; ownerDepth = -1 }
            }
            return Facts(idents, decls)
        }
    }
}
