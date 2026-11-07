package style

class Factory {
    fun Pair(a: Int, b: Int): Int = a + b
}

fun example(a: Int, b: String, f: Factory): List<Any> {
    val infix = a to b
    val member = f.Pair(1, 2)
    val reference: (Int, Int) -> Pair<Int, Int> = ::Pair
    return listOf(infix, member, reference)
}
