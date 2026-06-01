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

abstract class AppStyleListener {
    abstract fun onSuccess(result: Boolean?)
}

fun anonymousObjectCallbackCapture(timeRemaining: Long) {
    register(
        object : AppStyleListener() {
            override fun onSuccess(result: Boolean?) {
                println(timeRemaining)
            }
        }
    )
}

fun register(listener: AppStyleListener) {
    listener.onSuccess(true)
}

class AppStyleState(val value: String)

fun laterDefaultArgument(
    oldState: AppStyleState,
    value: String = oldState.value
) {
    println(value)
}

class AppStyleConfiguration

fun configure(init: AppStyleConfiguration.() -> Unit): AppStyleConfiguration {
    val configuration = AppStyleConfiguration()
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

fun forLoopIterableReference(params: List<String>) {
    for (param in params) {
        println(param)
    }
}

fun forLoopDestructuredOverParameter(entries: Map<String, String>) {
    for ((k, v) in entries) {
        println("$k=$v")
    }
}

class KSAnnotation {
    fun process() {}
}

// Soft-keyword param name (`annotation`) used as receiver — tree-sitter
// mis-parse handled via ERROR-mention fallback.
fun receivesSoftKeywordParam(annotation: KSAnnotation) {
    annotation.process()
}

// Parameter annotations parse as a `parameter_modifiers` sibling of the
// `parameter` node, so a `@Suppress` on the parameter must still be honored
// even though the annotation is not part of the parameter node's own text.
fun explicitlySuppressedUpper(@Suppress("UNUSED_PARAMETER") targetInfo: KSAnnotation): Int {
    return 0
}

fun explicitlySuppressedLower(@Suppress("unused") leftover: KSAnnotation): Int {
    return 0
}

fun suppressedAmongUsedParams(
    @Suppress("UNUSED_PARAMETER") unusedFirst: KSAnnotation,
    second: KSAnnotation
): Int {
    second.process()
    return 0
}
