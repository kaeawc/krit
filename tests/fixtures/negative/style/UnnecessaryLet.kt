package style

fun example() {
    val nullable: String? = "hello"
    nullable?.let {
        transform(it)
        save(it)
    }
    val x = listOf(1, 2, 3)
    x.let { list -> list.filter { it > 1 } }
}
