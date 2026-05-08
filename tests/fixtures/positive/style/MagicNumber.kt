package style

object TimeUnit {
    const val SECONDS = "seconds"
}

fun example() {
    val x = 42
    val timeout = 3000
    val retries = 5
    pollEvery(5, TimeUnit.SECONDS)
    val sample = List(200) { it }
}
