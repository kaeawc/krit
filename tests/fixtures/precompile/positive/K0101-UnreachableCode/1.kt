// EXPECTED-KOTLINC-ERROR: UNREACHABLE_CODE
fun afterReturn(x: Int): Int {
    return x
    val dead = x + 1
    println(dead)
}
