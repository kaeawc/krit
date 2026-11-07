package style

fun example(n: Int) {
    for (i in 0 until n) {
        println(i)
    }
    for (i in 0..n) {
        println(i)
    }
    for (i in 0..n - 2) {
        println(i)
    }
    for (i in 0..(n - 1)) {
        println(i)
    }
    val r = 0.rangeTo(n)
}
