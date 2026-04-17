package style

class Foo(val foo: List<Int>)

fun example() {
    val lists = listOf(listOf(1, 2), listOf(3))
    val count = lists.flatMap { it }.size
}
