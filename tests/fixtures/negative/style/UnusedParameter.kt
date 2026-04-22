package style

fun foo(value: Int) {
    println(value)
}

fun lambdaCapture(id: String) {
    listOf(1).map { id.length + it }
}

fun stringInterpolation(endpoint: String) {
    println("https://example.com/$endpoint")
}

interface UnusedParameterBase {
    fun render(id: String)
}

class UnusedParameterImpl : UnusedParameterBase {
    override fun render(id: String) {
        println("framework override")
    }
}

annotation class Subscribe

@Subscribe
fun subscribed(event: String) {
    println("framework entrypoint")
}

fun placeholders(ignored: String, expected: Int, _: Boolean) {
    println("allowed names")
}
