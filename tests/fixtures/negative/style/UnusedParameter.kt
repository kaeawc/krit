package style

fun foo(value: Int) {
    println(value)
}

fun lambdaCapture(id: String) {
    listOf(1).map { id.length + it }
}

fun nestedFunctionCapture(id: String) {
    fun nested() {
        println(id)
    }
    nested()
}

abstract class SignalStyleListener {
    abstract fun onSuccess(result: Boolean?)
}

fun anonymousObjectCallbackCapture(timeRemaining: Long) {
    register(
        object : SignalStyleListener() {
            override fun onSuccess(result: Boolean?) {
                println(timeRemaining)
            }
        }
    )
}

fun register(listener: SignalStyleListener) {
    listener.onSuccess(true)
}

class SignalStyleState(val value: String)

fun laterDefaultArgument(
    oldState: SignalStyleState,
    value: String = oldState.value
) {
    println(value)
}

class SignalStyleConfiguration

fun configure(init: SignalStyleConfiguration.() -> Unit): SignalStyleConfiguration {
    val configuration = SignalStyleConfiguration()
    configuration.init()
    return configuration
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
