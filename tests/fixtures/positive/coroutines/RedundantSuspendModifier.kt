package fixtures.positive.coroutines

suspend fun simple() {
    println("no suspend calls here")
}

fun helper() = Unit

suspend fun onlyProjectNonSuspendCalls() {
    helper()
}
