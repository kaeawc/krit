package style

data class Quad(val a: Int, val b: Int, val c: Int, val d: Int)

fun example() {
    val quad = Quad(1, 2, 3, 4)
    val (a, b, c, d) = quad
    println("$a $b $c $d")
}
