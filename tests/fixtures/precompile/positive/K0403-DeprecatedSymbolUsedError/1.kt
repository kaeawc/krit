// EXPECTED-KOTLINC-ERROR: DEPRECATION_ERROR
@Deprecated("Use newApi instead", level = DeprecationLevel.ERROR)
fun oldApi() {}

fun callsOldApi() {
    oldApi()
}
