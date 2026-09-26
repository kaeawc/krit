// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 17, 18, 19, 20, 21, 31, 32, 33, 34, 37, 39, 42x2, 46, 52, 54, 56, 62, 67, 73, 81, 87, 88, 90, 91, 92, 93
// Map lookups asserted with `!!`, which Go and FIR both report, and the
// lookalikes both leave alone.
package test

import java.util.TreeMap

class Holder(val maps: Maps)
class Maps(val current: Map<String, String>)

class Box {
    operator fun get(key: String): String? = null
}

fun indexAndCall(map: Map<String, String>, holder: Holder, key: String): String {
    val bracket = <!MapGetWithNotNullAssertionOperator!>map["key"]!!<!>
    val call = <!MapGetWithNotNullAssertionOperator!>map.get("key")!!<!>
    val nested = <!MapGetWithNotNullAssertionOperator!>holder.maps.current[key]!!<!>
    val named = <!MapGetWithNotNullAssertionOperator!>map.get(key = key)!!<!>
    val parenthesized = <!MapGetWithNotNullAssertionOperator!>(map[key])!!<!>
    return bracket + call + nested + named + parenthesized
}

fun mapKinds(
    mutable: MutableMap<String, Int>,
    hash: HashMap<String, Int>,
    linked: LinkedHashMap<String, Int>,
    tree: TreeMap<String, Int>,
): Int {
    return <!MapGetWithNotNullAssertionOperator!>mutable["a"]!!<!> +
        <!MapGetWithNotNullAssertionOperator!>hash["b"]!!<!> +
        <!MapGetWithNotNullAssertionOperator!>linked.get("c")!!<!> +
        <!MapGetWithNotNullAssertionOperator!>tree["d"]!!<!>
}

fun safeCall(map: Map<String, Int>?): Int = <!MapGetWithNotNullAssertionOperator!>map?.get("key")!!<!>

fun intKeys(map: Map<Int, String>): String = <!MapGetWithNotNullAssertionOperator!>map[1]!!<!>

// Two findings on one line (MapGetWithNotNullAssertionOperatorCountsTest).
fun sum(map: Map<String, Int>): Int = <!MapGetWithNotNullAssertionOperator!>map["a"]!!<!> + <!MapGetWithNotNullAssertionOperator!>map.get("b")!!<!>

// The receiver starts the reported line.
fun multiline(holder: Holder): String {
    return <!MapGetWithNotNullAssertionOperator!>holder<!>
        .maps
        .current["key"]!!
}

class Cache(private val entries: Map<String, String>) {
    val first: String get() = <!MapGetWithNotNullAssertionOperator!>entries["first"]!!<!>

    fun lookup(key: String): String = <!MapGetWithNotNullAssertionOperator!>entries[key]!!<!>

    fun inLambda(keys: List<String>): List<String> = keys.map { <!MapGetWithNotNullAssertionOperator!>entries[it]!!<!> }
}

object Registry {
    private val byName: Map<String, Int> = emptyMap()

    fun id(name: String): Int = <!MapGetWithNotNullAssertionOperator!>byName[name]!!<!>
}

fun anonymous(map: Map<String, Int>): Runnable = object : Runnable {
    override fun run() {
        println(<!MapGetWithNotNullAssertionOperator!>map["key"]!!<!>)
    }
}

fun local(map: Map<String, Int>): Int {
    class Reader {
        fun read(): Int = <!MapGetWithNotNullAssertionOperator!>map["key"]!!<!>
    }
    return Reader().read()
}

fun config(): Map<String, String> = emptyMap()

class Settings : HashMap<String, String>() {
    fun required(key: String): String = <!MapGetWithNotNullAssertionOperator!>this[key]!!<!>
}

class Delegating(private val backing: Map<String, Int>) : Map<String, Int> by backing

fun receivers(settings: Settings, delegating: Delegating, byId: Map<Long, String>, ids: List<Long>): Int {
    val fromCall = <!MapGetWithNotNullAssertionOperator!>config()["key"]!!<!>
    val inline = <!MapGetWithNotNullAssertionOperator!>mapOf("a" to 1)["a"]!!<!>
    val inferred = mapOf("a" to 1)
    val fromInferred = <!MapGetWithNotNullAssertionOperator!>inferred["a"]!!<!>
    val subclass = <!MapGetWithNotNullAssertionOperator!>settings["key"]!!<!>
    val delegated = <!MapGetWithNotNullAssertionOperator!>delegating["key"]!!<!>
    val computedKey = <!MapGetWithNotNullAssertionOperator!>byId[ids.first()]!!<!>
    return fromCall.length + inline + fromInferred + subclass.length + delegated + computedKey.length
}

// Negatives.
fun negatives(map: Map<String, String>, box: Box, list: List<String>, nullable: String?): String {
    val value = map.getValue("key")
    val orDefault = map.getOrDefault("key", "")
    val safe = map["key"] ?: ""
    val nonMapIndex = box["key"]!!
    val listIndex = list[0]
    val plain = nullable!!
    return value + orDefault + safe + nonMapIndex + listIndex + plain
}
