package style

fun example(x: Any) {
    if (x is String) {
        val s = x as String
        println(s)
    }
}

// when-expression with redundant cast
fun processWhen(x: Any) {
    when (x) {
        is String -> {
            val s = x as String
            println(s)
        }
    }
}
