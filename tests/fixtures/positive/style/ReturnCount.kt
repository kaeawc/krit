package style

fun classify(x: Int, y: Int): String {
    println("starting")
    val result = if (x > y) {
        if (x > 100) {
            return "big"
        }
        println("x-branch")
        "x"
    } else {
        if (y < 0) {
            return "negative"
        }
        if (y == 0) {
            return "zero"
        }
        if (y > 1000) {
            return "huge"
        }
        println("y-branch")
        "y"
    }
    return result
}
