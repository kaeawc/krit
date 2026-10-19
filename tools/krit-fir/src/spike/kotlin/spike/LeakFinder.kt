package spike

import org.jetbrains.kotlin.fir.FirSession
import java.io.File
import java.lang.ref.WeakReference
import java.lang.reflect.Modifier
import java.util.IdentityHashMap

/**
 * Finds what keeps old source sessions alive when the library sessions are reused (F1): runs warm whole-module
 * compiles, keeps weak references to each source session, then searches the object graph reachable from the reused
 * daemon state (library sessions, environment) for the oldest source session that survived GC and prints the chain.
 */
class LeakFinder(val o: Map<String, String>) {
    fun run() {
        val n = (o["files"] ?: "300").toInt()
        val dir = File(SCRATCH, "leak").also { it.deleteRecursively(); it.mkdirs() }
        val g = Gen(File(dir, "src"), n); g.generate()
        val st = WarmState().also { it.pinLvs = o["pinLvs"] != "false"; it.freshJavaComponents = o["freshJava"] != "false" }
        val refs = ArrayList<WeakReference<FirSession>>()
        val all = g.texts.keys.map(::canon)
        repeat((o["compiles"] ?: "4").toInt()) {
            g.apply("body")
            runCheck(Phases.warmFrontend(st, null, Timings()) { refs += WeakReference(it.session) }, kritArgs(all, g.classpath, File(dir, "out")), all)
            if (o["release"] != "false") st.releaseHeldCompiles()
        }
        repeat(3) { System.gc(); Thread.sleep(100) }
        val alive = refs.dropLast(1).mapNotNull { it.get() }
        println("sessions created=${refs.size}, older ones still reachable after GC=${alive.size}")
        if (alive.isEmpty()) { println("no leak"); return }
        val roots = linkedMapOf<String, Any?>("sharedLibrarySession" to st.shared, "libraryList" to st.libraryList, "context" to st.context, "env" to st.env)
        for ((i, target) in alive.withIndex()) {
            val path = bfs(roots, target, (o["maxNodes"] ?: "20000000").toInt())
            if (path == null) println("session #$i: not reachable from the daemon state within the node budget") else {
                println("session #$i retention chain (root -> old source session):")
                path.forEach { println("   $it") }
            }
        }
        st.dispose(); dir.deleteRecursively()
    }

    private fun bfs(roots: Map<String, Any?>, target: Any, max: Int): List<String>? {
        val parent = IdentityHashMap<Any, Pair<Any?, String>>()
        val queue = ArrayDeque<Any>()
        for ((name, r) in roots) if (r != null && !parent.containsKey(r)) { parent[r] = null to name; queue += r }
        var visited = 0
        while (queue.isNotEmpty() && visited < max) {
            val cur = queue.removeFirst(); visited++
            if (cur === target) {
                val chain = ArrayList<String>()
                var x: Any? = cur
                while (x != null) { val (p, label) = parent[x]!!; chain += "$label  [${x.javaClass.name}]"; x = p }
                return chain.reversed()
            }
            for ((label, child) in children(cur)) {
                if (child == null || parent.containsKey(child)) continue
                val cls = child.javaClass
                if (cls.isPrimitive || child is String || child is Number || child is Boolean || child is Class<*> || child is ClassLoader || child is Thread) continue
                parent[child] = cur to label
                queue += child
            }
        }
        println("bfs visited $visited objects")
        return null
    }

    private fun children(o: Any): List<Pair<String, Any?>> {
        val out = ArrayList<Pair<String, Any?>>()
        when (o) {
            is Array<*> -> o.forEachIndexed { i, e -> out += "[$i]" to e }
            is Map<*, *> -> try { o.entries.forEachIndexed { i, (k, v) -> out += "key#$i" to k; out += "value[$k]".take(120) to v } } catch (_: Exception) {}
            is Collection<*> -> try { o.forEachIndexed { i, e -> out += "[$i]" to e } } catch (_: Exception) {}
        }
        if (o is Array<*> || (o.javaClass.name.startsWith("java.") && (o is Map<*, *> || o is Collection<*>))) return out
        if (o is java.lang.ref.Reference<*>) return out // weak/soft/phantom referents do not retain
        var c: Class<*>? = o.javaClass
        while (c != null && c != Any::class.java) {
            for (f in c.declaredFields) {
                if (Modifier.isStatic(f.modifiers) || f.type.isPrimitive) continue
                try { f.isAccessible = true; out += ".${f.name}" to f.get(o) } catch (_: Throwable) {}
            }
            c = c.superclass
        }
        return out
    }
}
