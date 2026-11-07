package style

fun example(x: Any) = if (x is String) x else null

fun negated(x: Any) = if (x !is String) null else x

fun withBlocks(x: Any): Any? {
    return if (x is Int) {
        x
    } else {
        null
    }
}
