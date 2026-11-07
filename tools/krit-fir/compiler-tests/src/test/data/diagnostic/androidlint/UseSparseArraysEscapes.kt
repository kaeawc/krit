// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 68
// A SparseArray is not a Map. A HashMap that is returned or stored as a Map,
// passed as an argument, handed on, or used through a Map-only member is not
// reported. The same map used only through get/put is reported (last function).
package test

fun consume(map: Map<Int, String>) {}

fun returnedAsMap(): Map<Int, String> = HashMap<Int, String>()

fun returnedAsMutableMap(): MutableMap<Int, String> {
    return HashMap<Int, String>()
}

fun returnedThroughApply(): Map<Int, String> = HashMap<Int, String>().apply { put(1, "a") }

fun branchReturned(flag: Boolean): Map<Int, String> = if (flag) HashMap<Int, String>() else emptyMap()

fun passedAsArgument() {
    consume(HashMap<Int, String>())
}

class Holder {
    val typed: Map<Int, String> = HashMap<Int, String>()
    var assigned: MutableMap<Int, String> = mutableMapOf()

    fun reset() {
        assigned = HashMap<Int, String>()
    }
}

fun localReturned(): Map<Int, String> {
    val map = HashMap<Int, String>()
    map[1] = "a"
    return map
}

fun localPassed() {
    val map = HashMap<Int, String>()
    consume(map)
}

fun localGetOrPut() {
    val cache = HashMap<Int, String>()
    cache.getOrPut(1) { "a" }
}

fun localEntries() {
    val map = HashMap<Long, String>()
    for ((key, value) in map) {
        println("$key $value")
    }
}

fun localValues() {
    val map = HashMap<Int, String>()
    println(map.values.size)
}

class MemberUsedAsMap {
    private val index = HashMap<Int, String>()

    fun snapshot(): Map<Int, String> = index
}

fun stillReported() {
    val map = <!UseSparseArrays!>HashMap<Int, String>()<!>
    map[1] = "a"
    println(map[1])
}

fun plusOperand(a: Map<Int, String>) {
    a + HashMap<Int, String>()
}
