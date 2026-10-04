// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 13, 21, 35, 38
// Look-alike lookups that are not a Map.get call.
package test

// A project class that reuses a map type name.
class HashMap<K, V> {
    operator fun get(key: K): V? = null
}

// Go reports this: it trusts the type name HashMap. This HashMap is the
// project class above, not a map, so FIR does not report it.
fun projectHashMap(cache: HashMap<String, Int>): Int = cache["key"]!!

// A project `get` extension on a map, with a key type the map does not have.
operator fun Map<*, *>.get(index: Int): String? = null

// Go reports this: the receiver is a Map whose key type is a wildcard, so it
// accepts any key and does not check which `get` the call resolves to. The
// call is the project extension above, not a map lookup.
fun extensionOnStarMap(map: Map<*, *>): String = map[0]!!

// Both leave this alone: Go because Int is not the String key type, FIR
// because the call is the project extension.
fun extensionOnTypedMap(map: Map<String, Int>): String = map[0]!!

// A map subclass with an unrelated `get` overload.
class Indexed : LinkedHashMap<String, String>() {
    operator fun get(index: Int): String? = values.elementAtOrNull(index)
}

// Go reports this: Indexed is a LinkedHashMap, and with no type arguments
// written on the receiver's type Go does not compare the key type. The call
// is the Int overload above, a positional lookup, not Map.get.
fun overload(indexed: Indexed): String = indexed[0]!!

// Both report the String lookup, which is the inherited Map.get.
fun overridden(indexed: Indexed): String = <!MapGetWithNotNullAssertionOperator!>indexed["key"]!!<!>

// Not a map.
class Table {
    fun get(key: String): String? = null
}

fun table(table: Table): String = table.get("key")!!

// Index access on a list or an array.
fun list(values: List<String?>): String = values[0]!!

fun array(values: Array<String?>): String = values[0]!!
