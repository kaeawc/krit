package fixtures.negative.emptyblocks

fun spin(condition: Boolean) {
    while (condition) {
        process()
    }
}
