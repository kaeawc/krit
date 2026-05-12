// Negative: return is the last statement; nothing follows.
fun tailReturn(x: Int): Int {
    val y = x + 1
    return y
}

// Negative: trailing comments are not statements.
fun trailingComment(x: Int): Int {
    return x
    // this is just a comment, not unreachable code
}
