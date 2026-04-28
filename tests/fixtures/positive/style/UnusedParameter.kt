package style

fun foo(unused: Int) {
    println("hello")
}

fun substringCollision(id: String) {
    val guid = "x"
    println(guid)
}

fun commentsAndStringsDoNotCount(id: String) {
    // id
    println("id")
}

fun shadowedByLambda(id: String) {
    listOf("x").forEach { id -> println(id) }
}

fun shadowedByNestedFunction(id: String) {
    fun nested(id: String) {
        println(id)
    }
    nested("local")
}
