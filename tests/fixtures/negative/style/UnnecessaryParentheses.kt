package style

fun example(): Int {
    val x = 42
    val y = (x + 1) * 2
    val z = (x - 1) / 3
    return x
}

fun precedenceNeeded() {
    val a = (2 + 3) * 4
    val b = (a > 0) && (a < 10)
}
