// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 43
// Map lookups asserted with `!!` that Go misses because its source type
// inference cannot prove the receiver is a map, or cannot prove the key has
// the map's key type. Each one is a Map.get call (or the stdlib extension
// that forwards to it), so FIR reports it.
package test

import java.util.Properties
import java.util.concurrent.ConcurrentHashMap

fun environment(): Map<String, String> = emptyMap()

// Go does not type a `super` receiver.
class Settings : HashMap<String, String>() {
    fun viaSuper(key: String): String = <!MapGetWithNotNullAssertionOperator!>super.get(key)!!<!>
}

// Go does not type a companion object's property, even a declared one.
class Defaults {
    companion object {
        private val values: Map<String, String> = mapOf("a" to "b")

        fun value(key: String): String = <!MapGetWithNotNullAssertionOperator!>values[key]!!<!>
    }
}

fun receivers(properties: Properties, concurrent: ConcurrentHashMap<String, Int>): Int {
    // JDK maps whose names are not in Go's list of map types.
    val props = <!MapGetWithNotNullAssertionOperator!>properties["key"]!!<!>
    val jdk = <!MapGetWithNotNullAssertionOperator!>concurrent["key"]!!<!>
    // A Java method's return type.
    val env = <!MapGetWithNotNullAssertionOperator!>System.getenv()["HOME"]!!<!>
    // A scope-function parameter.
    val scoped = environment().let { <!MapGetWithNotNullAssertionOperator!>it["key"]!!<!> }
    return props.hashCode() + jdk + env.length + scoped.length
}

// Two lookups on one line. Go reports the inner `tables["a"]!!` but cannot
// type the outer receiver, the `!!` expression, so it misses the second
// lookup; FIR reports both (MapGetWithNotNullAssertionOperatorCountsTest
// checks the count).
fun nestedMaps(tables: Map<String, Map<String, Int>>): Int = <!MapGetWithNotNullAssertionOperator!>tables["a"]!!["b"]!!<!>

// A type parameter bounded by Map.
fun <M : Map<String, Int>> generic(map: M): Int = <!MapGetWithNotNullAssertionOperator!>map["key"]!!<!>

// A key whose type is a subtype of the map's key type: Go compares the type
// names, CharSequence and String.
fun subtypeKey(byName: Map<CharSequence, Int>, name: String): Int = <!MapGetWithNotNullAssertionOperator!>byName[name]!!<!>

// The stdlib `Map<out K, V>.get(key)` extension: a String-keyed map looked up
// with a CharSequence key.
fun stdlibExtension(byName: Map<String, Int>, name: CharSequence): Int = <!MapGetWithNotNullAssertionOperator!>byName.get(name)!!<!>

// A lookup that is the target of an assignment: tree-sitter parses the left
// side of an assignment as a directly assignable expression, not the
// postfix `!!` expression Go's rule visits.
class Counter(var count: Int, var items: Int)

fun assignmentTargets(counters: Map<String, Counter>, arrays: Map<String, IntArray>, key: String) {
    <!MapGetWithNotNullAssertionOperator!>counters[key]!!<!>.count += 1
    <!MapGetWithNotNullAssertionOperator!>counters[key]!!<!>.items += 1
    <!MapGetWithNotNullAssertionOperator!>counters[key]!!<!>.count = 0
    <!MapGetWithNotNullAssertionOperator!>arrays[key]!!<!>[0] = 1
    <!MapGetWithNotNullAssertionOperator!>arrays[key]!!<!>[0] += 1
}

// Receivers typed by an anonymous object: Go does not type an object
// expression.
fun anonymousHashMap(key: String): Int {
    val o = object : HashMap<String, Int>() {}
    return <!MapGetWithNotNullAssertionOperator!>o[key]!!<!>
}

fun anonymousDelegated(m: Map<String, Int>): Int {
    val o = object : Map<String, Int> by m {}
    return <!MapGetWithNotNullAssertionOperator!>o["a"]!!<!>
}

fun anonymousOverride(): Int {
    val o = object : HashMap<String, Int>() {
        override fun get(key: String): Int? = 1
    }
    return <!MapGetWithNotNullAssertionOperator!>o["a"]!!<!>
}

fun anonymousThis(): Any = object : HashMap<String, Int>() {
    fun first(): Int = <!MapGetWithNotNullAssertionOperator!>this.get("a")!!<!>
}
