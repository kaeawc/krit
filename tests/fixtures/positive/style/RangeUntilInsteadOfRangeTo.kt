package style

fun example() {
    for (i in 0 until 10) {
        println(i)
    }
    for (i in 0 .. 10 - 1) {
        println(i)
    }
    val r = 0.rangeTo(10 - 1)
}
