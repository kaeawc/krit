package performance

fun check(x: Boolean, y: Boolean): Boolean {
    if (x && y) {
        return true
    }
    return x || y
}
