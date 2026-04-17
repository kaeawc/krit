package fixtures.negative.performance

fun printNumbers() {
    for (i in 1..10) {
        println(i)
    }
}

fun printList() {
    listOf(1, 2, 3).forEach { println(it) }
}

fun checkRange() {
    (1..10).isEmpty()
}
