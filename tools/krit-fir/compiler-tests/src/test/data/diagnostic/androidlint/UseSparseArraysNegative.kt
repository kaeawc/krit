// RENDER_DIAGNOSTICS_FULL_TEXT
// Negatives Go also leaves alone: a key that is not Int, Integer, or Long, a
// nullable key, map types that are not HashMap (LinkedHashMap, a subclass), a
// superclass delegation, a constructor reference, and factory functions.
package test

class IntMap : HashMap<Int, String>()

fun stringKey(): Map<String, String> = HashMap<String, String>()

fun nullableKey(): Map<Int?, String> = HashMap<Int?, String>()

fun shortKey(): Map<Short, String> = HashMap<Short, String>()

fun linked(): Map<Int, String> = LinkedHashMap<Int, String>()

fun subclass(): Map<Int, String> = IntMap()

fun factory(): Map<Int, String> = hashMapOf<Int, String>()

fun mutable(): Map<Int, String> = mutableMapOf<Int, String>()

fun reference(): () -> HashMap<Int, String> = ::HashMap

fun anonymousSubclass(): Map<Int, String> = object : HashMap<Int, String>() {}

fun localSubclass(): Map<Int, String> {
    class Local : HashMap<Int, String>()
    return Local()
}

fun <K> typeParameterKey(): Map<K, String> = HashMap<K, String>()
