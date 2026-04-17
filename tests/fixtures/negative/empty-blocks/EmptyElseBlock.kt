package fixtures.negative.emptyblocks

fun check(x: Boolean) {
    if (x) {
        foo()
    } else {
        bar()
    }
}
