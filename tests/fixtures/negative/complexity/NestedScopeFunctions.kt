package complexity

data class Outer(val inner: String?)

fun example() {
    val x = Outer("hello")
    x.apply {
        println(inner)
    }
}
