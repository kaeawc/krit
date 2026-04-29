package style

import java.util.concurrent.TimeUnit

const val MAX_COUNT = 42

fun example() {
    val x = 1
    val y = 0
    val z = -1
    val w = 2
}

// Companion object properties should be ignored (ignoreCompanionObjectPropertyDeclaration=true)
class Config {
    companion object {
        val TIMEOUT = 5000
        const val MAX = 100
    }
}

// Named arguments should be ignored (ignoreNamedArgument=true)
fun usages() {
    call(timeout = 5000)
    create(width = 100, height = 200)
}

// Extension function receivers should be ignored (ignoreExtensionFunctions=true)
fun extensions() {
    val x = 100.toLong()
    val y = 24.hours
}

// Default parameter values should be ignored (detekt unconditional)
fun defaults(timeout: Int = 5000, retries: Int = 3) {}
class Foo(val x: Int = 42)

// Function return constants should be ignored (detekt unconditional)
fun maxSize() = 100
fun getDefault(): Int {
    return 42
}

// Duration literals paired with a canonical TimeUnit are self-documenting.
fun durationCalls() {
    Observable.interval(0, 5, TimeUnit.SECONDS)
    events.throttleLatest(500, TimeUnit.MILLISECONDS)
    completable.timeout(10, TimeUnit.SECONDS, fallback)
}

fun color(alpha: Int, red: Int, green: Int, blue: Int): Int {
    return (alpha shl 24) or (red shl 16) or (green shl 8) or blue
}

fun httpStatusRanges(response: Response): Boolean {
    return response.code !in 200 until 300 && response.code != HTTP_RESPONSE_NOT_MODIFIED
}

fun httpStatusComparisons(response: JavaResponse): Boolean {
    return response.code() >= 200 && response.code() < 300
}
