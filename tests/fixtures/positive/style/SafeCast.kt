package style

fun example(x: Any) {
    if (x is String) {
        val s = x as String
        println(s)
    }
}
