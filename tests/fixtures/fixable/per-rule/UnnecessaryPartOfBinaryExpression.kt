package performance

fun check(x: Boolean): Boolean {
    if (x && true) {
        return true
    }
    return x || false
}
