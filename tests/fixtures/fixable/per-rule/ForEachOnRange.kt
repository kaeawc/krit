package fixtures.positive.performance

fun printNumbers() {
    (1..10).forEach { println(it) }
}

fun printUntil() {
    (1 until 10).forEach { println(it) }
}

fun printDownTo() {
    (10 downTo 1).forEach { println(it) }
}

fun printReversed() {
    (1..10).reversed().forEach { println(it) }
}

fun printStep() {
    (10 downTo 1 step 2).forEach { println(it) }
}

fun printLong() {
    (1L..10L).forEach { println(it) }
}

fun printChar() {
    ('a'..'z').forEach { println(it) }
}
