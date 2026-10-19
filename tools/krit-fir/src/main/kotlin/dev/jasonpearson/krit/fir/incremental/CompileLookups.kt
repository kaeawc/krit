package dev.jasonpearson.krit.fir.incremental

import org.jetbrains.kotlin.incremental.components.LookupTracker
import org.jetbrains.kotlin.incremental.components.Position
import org.jetbrains.kotlin.incremental.components.ScopeKind
import java.io.File
import java.util.concurrent.ConcurrentHashMap

/** A lookup symbol as the compiler's incremental compilation stores it: (scope fq name, name). */
internal data class Sym(val scope: String, val name: String) {
    override fun toString() = "$scope#$name"
}

internal fun canonicalPath(path: String): String =
    try { File(path).canonicalPath } catch (_: Exception) { File(path).absolutePath }

/**
 * Records the [LookupTracker] events of one compile per canonical source path. This is the dependency
 * recording Gradle/JPS incremental compilation uses: K2 reports every name it looks up in every package
 * or classifier scope it searches, whether or not the lookup succeeds, so a later declaration that would
 * shadow or overload is covered too. Passed to the compiler through `Services`.
 *
 * `ImportTracker` is intentionally not wired: the lookup of an imported name already arrives here, in the
 * scope the import names, so imports add no dependency this tracker misses.
 */
internal class CompileLookups : LookupTracker {
    val byFile = ConcurrentHashMap<String, MutableSet<Sym>>()
    private val canonical = ConcurrentHashMap<String, String>()

    override val requiresPosition: Boolean get() = false

    override fun record(filePath: String, position: Position, scopeFqName: String, scopeKind: ScopeKind, name: String) {
        val file = canonical.getOrPut(filePath) { canonicalPath(filePath) }
        byFile.getOrPut(file) { ConcurrentHashMap.newKeySet() }.add(Sym(scopeFqName, name))
    }

    override fun clear() {}
}

/** Reverse index of the lookups of each file's last compile: symbol -> files that looked it up. */
internal class LookupIndex {
    private val forward = HashMap<String, Set<Sym>>()
    private val reverse = HashMap<Sym, MutableSet<String>>()
    private val byScope = HashMap<String, MutableSet<String>>()

    val files: Set<String> get() = forward.keys
    fun lookupsOf(file: String): Set<Sym> = forward[file].orEmpty()

    fun update(file: String, syms: Set<Sym>) {
        remove(file)
        forward[file] = syms
        for (s in syms) {
            reverse.getOrPut(s) { HashSet() }.add(file)
            byScope.getOrPut(s.scope) { HashSet() }.add(file)
        }
    }

    fun remove(file: String) {
        val old = forward.remove(file) ?: return
        for (s in old) {
            reverse[s]?.let { files -> files.remove(file); if (files.isEmpty()) reverse.remove(s) }
            byScope[s.scope]?.let { files -> files.remove(file); if (files.isEmpty()) byScope.remove(s.scope) }
        }
    }

    fun clear() {
        forward.clear(); reverse.clear(); byScope.clear()
    }

    /** Files that looked up any of [syms], or anything at all in one of [scopes] (a changed class header). */
    fun dependents(syms: Collection<Sym>, scopes: Collection<String>): Set<String> {
        val out = HashSet<String>()
        for (s in syms) reverse[s]?.let(out::addAll)
        for (scope in scopes) byScope[scope]?.let(out::addAll)
        return out
    }
}
