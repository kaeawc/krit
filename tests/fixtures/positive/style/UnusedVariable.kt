package fixtures.positive.style

fun foo() {
    val unused = 1
    println("hello")
}

class Worker {
    fun run() {
        val unusedInMethod = 2
        println("ready")
    }
}

fun walkPairs(pairs: List<Pair<Int, Int>>) {
    for ((a, b) in pairs) {
        println("body")
    }
}
