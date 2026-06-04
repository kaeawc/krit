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

// Object member that is never reassigned still flags. A qualified read of the
// member (not a write) must not be mistaken for a reassignment.
object Config {
    private var enabled = false

    fun read() = Config.enabled
}
