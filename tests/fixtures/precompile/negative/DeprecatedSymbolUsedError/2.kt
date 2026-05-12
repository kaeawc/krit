// Negative: WARNING and HIDDEN are not the ERROR target.
@Deprecated("Soft warning", level = DeprecationLevel.WARNING)
fun warnApi() {}

@Deprecated("Hidden", level = DeprecationLevel.HIDDEN)
fun hiddenApi() {}

fun callsThem() {
    warnApi()
}
