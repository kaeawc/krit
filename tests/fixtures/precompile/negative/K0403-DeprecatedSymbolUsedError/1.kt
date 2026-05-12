// Negative: default level is WARNING, not ERROR — kotlinc accepts the call.
@Deprecated("Prefer newApi")
fun oldApi() {}

fun callsOldApi() {
    oldApi()
}
