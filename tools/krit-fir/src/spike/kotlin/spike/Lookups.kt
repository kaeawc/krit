package spike

import org.jetbrains.kotlin.incremental.components.ImportTracker
import org.jetbrains.kotlin.incremental.components.LookupTracker
import org.jetbrains.kotlin.incremental.components.Position
import org.jetbrains.kotlin.incremental.components.ScopeKind
import java.util.concurrent.ConcurrentHashMap

/** A lookup symbol as IC stores it: (scope fq name, name). */
data class Sym(val scope: String, val name: String) {
    override fun toString() = "$scope#$name"
}

/**
 * Records `LookupTracker` / `ImportTracker` events of one compile, per canonical file path. This is the compiler's own
 * IC dependency recording (`CommonConfigurationKeys.LOOKUP_TRACKER` via `Services`): K2 reports every name it looks up
 * in every package / classifier scope it searches, whether or not the lookup succeeds.
 */
class CompileLookups : LookupTracker, ImportTracker {
    val byFile = ConcurrentHashMap<String, MutableSet<Sym>>()
    val importsByFile = ConcurrentHashMap<String, MutableSet<String>>()
    var records = 0L
    private val canonCache = ConcurrentHashMap<String, String>()
    private fun c(p: String) = canonCache.getOrPut(p) { canon(p) }

    override val requiresPosition: Boolean get() = false
    override fun record(filePath: String, position: Position, scopeFqName: String, scopeKind: ScopeKind, name: String) {
        records++
        byFile.getOrPut(c(filePath)) { ConcurrentHashMap.newKeySet() }.add(Sym(scopeFqName, name))
    }
    override fun clear() {}
    override fun report(filePath: String, importedFqName: String) {
        importsByFile.getOrPut(c(filePath)) { ConcurrentHashMap.newKeySet() }.add(importedFqName)
    }
}

/** Daemon-side reverse index over the lookups of the last compile of each file. */
class LookupIndex {
    private val fwd = HashMap<String, Set<Sym>>()
    private val rev = HashMap<Sym, MutableSet<String>>()
    private val byScope = HashMap<String, MutableSet<String>>()
    private val byName = HashMap<String, MutableSet<String>>()

    /** Files that looked up any of [names] in any scope (conservative, scope-blind). */
    fun dependentsByName(names: Collection<String>): Set<String> = names.flatMapTo(HashSet()) { byName[it].orEmpty() }

    val files: Int get() = fwd.size
    val edges: Long get() = fwd.values.sumOf { it.size.toLong() }

    fun update(file: String, syms: Set<Sym>) {
        remove(file)
        fwd[file] = syms
        for (s in syms) {
            rev.getOrPut(s) { HashSet() }.add(file)
            byScope.getOrPut(s.scope) { HashSet() }.add(file)
            byName.getOrPut(s.name) { HashSet() }.add(file)
        }
    }

    fun remove(file: String) {
        val old = fwd.remove(file) ?: return
        for (s in old) {
            rev[s]?.remove(file)
            byScope[s.scope]?.remove(file)
            byName[s.name]?.remove(file)
        }
    }

    /** Files that looked up any of [syms], or anything at all in one of [scopes] (a changed class header). */
    fun dependents(syms: Collection<Sym>, scopes: Collection<String>): Set<String> {
        val out = HashSet<String>()
        for (s in syms) rev[s]?.let(out::addAll)
        for (sc in scopes) byScope[sc]?.let(out::addAll)
        return out
    }
}
