package fixtures.negative.style

fun foo() {
    val used = 1
    println(used)
}

class Bar {
    val classProperty = "not a local variable"
}

enum class Direction(val degrees: Int) {
    NORTH(0),
    SOUTH(180);

    val radians: Double get() = degrees * 3.14159 / 180.0
}
