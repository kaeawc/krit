package style

fun example(flag: Boolean?) {
    if (flag ?: false) {
        println("true")
    }
    val isEnabled = flag ?: true
}
