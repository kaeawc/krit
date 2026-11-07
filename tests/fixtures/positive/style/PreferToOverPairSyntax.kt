package style

fun example(a: Int, b: String): List<Pair<Int, String>> {
    val plain = Pair(a, b)
    val qualified = kotlin.Pair(a, b)
    val typed = Pair<Int, String>(a, b)
    val named = Pair(first = a, second = b)
    return listOf(plain, qualified, typed, named)
}
