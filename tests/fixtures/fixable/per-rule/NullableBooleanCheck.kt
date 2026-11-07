package style

fun example(flag: Boolean?) {
    if (flag ?: false) {
        println("true")
    }
    val isEnabled = flag ?: true
    if (flag ?: true) {
        println("not false")
    }
}
