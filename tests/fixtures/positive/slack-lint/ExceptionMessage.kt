package fixtures.positive.slacklint

fun open(path: String?) {
    requireNotNull(path)
    check(path.isNotEmpty())
}
