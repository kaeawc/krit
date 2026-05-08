package complexity

data class Outer(val inner: Inner?)
data class Inner(val value: String?)

fun example() {
    val x = Outer(Inner("hello"))
    x.apply {
        inner.also {
            it?.value?.let {
                println(it)
            }
        }
    }
}
