package style

fun example() {
    val nullable: String? = "hello"
    nullable?.let { it }
    val x = "world"
    x.let { it.toString() }
}
