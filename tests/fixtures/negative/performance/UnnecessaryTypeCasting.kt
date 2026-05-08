package performance

fun process() {
    val x: Any = "hello"
    val y = x as String
    println(y)
}

fun standaloneSafeCast(value: Any) {
    val maybe = value as? String
    println(maybe)
}
