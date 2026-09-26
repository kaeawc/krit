// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Negative: factories called with elements, the empty counterparts
// themselves, and other factories the rule does not cover.
package test

fun withElements(arr: Array<String>, maybe: String?) {
    val a = listOf("a")
    val b = listOfNotNull(maybe)
    val c = setOf(1, 2)
    val d = mapOf("a" to 1)
    val e = arrayOf("a")
    val f = sequenceOf(1)
    // A spread argument is an element source, not an empty call.
    val g = listOf(*arr)
    val h = arrayOf(*arr)
    println(listOf(a, b, c, d, e, f, g, h))
}

fun counterparts() {
    val a = emptyList<String>()
    val b = emptySet<Int>()
    val c = emptyMap<String, Int>()
    val d = emptyArray<String>()
    val e = emptySequence<Int>()
    println(listOf(a, b, c, d, e))
}

// Mutable factories and builders are out of scope.
fun others() {
    val a = mutableListOf<String>()
    val b = mutableSetOf<Int>()
    val c = mutableMapOf<String, Int>()
    val d = arrayListOf<String>()
    val e = hashSetOf<Int>()
    val f = buildList<String> { }
    val g = intArrayOf()
    println(listOf(a, b, c, d, e, f, g))
}

// A reference to the factory is not a call.
fun references() {
    val ref: () -> List<String> = ::emptyList
    println(ref())
}
