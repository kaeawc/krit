package fixtures.positive.style

fun compute() {
    var x = 1
    println(x)
}

class Holder {
    private var unused = 42
    fun read() = unused
}

class Box(var x: Int)

fun ignoresOtherReceiver(other: Box) {
    var x = 0
    other.x = 42
    println(x)
}
