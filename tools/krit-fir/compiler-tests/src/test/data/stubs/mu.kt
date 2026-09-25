// Compiler-test source stubs; never packaged in the production artifact.
package mu

// kotlin-logging 1.x-3.x: on the JVM KLogger extends org.slf4j.Logger and adds
// lazy lambda overloads.
interface KLogger : org.slf4j.Logger {
    val underlyingLogger: org.slf4j.Logger

    fun trace(msg: () -> Any?)

    fun trace(t: Throwable?, msg: () -> Any?)

    fun debug(msg: () -> Any?)

    fun debug(t: Throwable?, msg: () -> Any?)

    fun info(msg: () -> Any?)

    fun info(t: Throwable?, msg: () -> Any?)

    fun warn(msg: () -> Any?)

    fun warn(t: Throwable?, msg: () -> Any?)

    fun error(msg: () -> Any?)

    fun error(t: Throwable?, msg: () -> Any?)
}

object KotlinLogging {
    fun logger(func: () -> Unit): KLogger = TODO()

    fun logger(name: String): KLogger = TODO()
}
