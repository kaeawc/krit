// Compiler-test source stubs; never packaged in the production artifact.
package io.github.oshai.kotlinlogging

// kotlin-logging 5+: a standalone interface (no SLF4J supertype).
interface KLogger {
    val name: String

    fun trace(message: () -> Any?)

    fun trace(throwable: Throwable?, message: () -> Any?)

    fun debug(message: () -> Any?)

    fun debug(throwable: Throwable?, message: () -> Any?)

    fun info(message: () -> Any?)

    fun info(throwable: Throwable?, message: () -> Any?)

    fun warn(message: () -> Any?)

    fun warn(throwable: Throwable?, message: () -> Any?)

    fun error(message: () -> Any?)

    fun error(throwable: Throwable?, message: () -> Any?)

    fun isTraceEnabled(): Boolean = TODO()

    fun isDebugEnabled(): Boolean = TODO()

    fun isInfoEnabled(): Boolean = TODO()

    fun isWarnEnabled(): Boolean = TODO()

    fun isErrorEnabled(): Boolean = TODO()
}

object KotlinLogging {
    fun logger(func: () -> Unit): KLogger = TODO()

    fun logger(name: String): KLogger = TODO()
}
