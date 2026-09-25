// Compiler-test source stubs; never packaged in the production artifact.
package org.apache.logging.log4j

// Log4j 2 Java interface (the p0..p9 arity overloads are folded into vararg).
interface Logger {
    val name: String

    val isTraceEnabled: Boolean

    val isDebugEnabled: Boolean

    val isInfoEnabled: Boolean

    val isWarnEnabled: Boolean

    val isErrorEnabled: Boolean

    fun trace(message: String?)

    fun trace(message: String?, vararg params: Any?)

    fun trace(message: String?, t: Throwable?)

    fun trace(message: Any?)

    fun debug(message: String?)

    fun debug(message: String?, vararg params: Any?)

    fun debug(message: String?, t: Throwable?)

    fun debug(message: Any?)

    fun info(message: String?)

    fun info(message: String?, vararg params: Any?)

    fun info(message: String?, t: Throwable?)

    fun info(message: Any?)

    fun warn(message: String?)

    fun warn(message: String?, vararg params: Any?)

    fun warn(message: String?, t: Throwable?)

    fun warn(message: Any?)

    fun error(message: String?)

    fun error(message: String?, vararg params: Any?)

    fun error(message: String?, t: Throwable?)

    fun error(message: Any?)

    fun fatal(message: String?)

    fun fatal(message: String?, t: Throwable?)
}

object LogManager {
    fun getLogger(): Logger = TODO()

    fun getLogger(name: String?): Logger = TODO()

    fun getLogger(clazz: Class<*>?): Logger = TODO()
}
