package performance

fun process() {
    val x: String = "hello"
    val y: String = x as String
    println(y)
}

fun safeCastNullChecks(value: Any) {
    if (value as? String != null) {
        println(value)
    }

    if (null != value as? Foo) {
        println(value)
    }
}
