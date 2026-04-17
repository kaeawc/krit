package complexity

data class A(val b: String?)

fun example(a: A?) {
    val result = a?.b
}
