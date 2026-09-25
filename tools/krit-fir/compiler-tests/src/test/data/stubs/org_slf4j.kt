// Compiler-test source stubs; never packaged in the production artifact.
package org.slf4j

import java.io.Closeable
import org.slf4j.spi.LoggingEventBuilder

// Java interface; unannotated String/Object parameters accept null in real
// code, so they are modeled nullable. Java getName()/isXEnabled() are read as
// synthetic properties at Kotlin call sites.
interface Logger {
    val name: String

    val isTraceEnabled: Boolean

    val isDebugEnabled: Boolean

    val isInfoEnabled: Boolean

    val isWarnEnabled: Boolean

    val isErrorEnabled: Boolean

    fun trace(msg: String?)

    fun trace(format: String?, arg: Any?)

    fun trace(format: String?, arg1: Any?, arg2: Any?)

    fun trace(format: String?, vararg arguments: Any?)

    fun trace(msg: String?, t: Throwable?)

    fun debug(msg: String?)

    fun debug(format: String?, arg: Any?)

    fun debug(format: String?, arg1: Any?, arg2: Any?)

    fun debug(format: String?, vararg arguments: Any?)

    fun debug(msg: String?, t: Throwable?)

    fun info(msg: String?)

    fun info(format: String?, arg: Any?)

    fun info(format: String?, arg1: Any?, arg2: Any?)

    fun info(format: String?, vararg arguments: Any?)

    fun info(msg: String?, t: Throwable?)

    fun warn(msg: String?)

    fun warn(format: String?, arg: Any?)

    fun warn(format: String?, arg1: Any?, arg2: Any?)

    fun warn(format: String?, vararg arguments: Any?)

    fun warn(msg: String?, t: Throwable?)

    fun error(msg: String?)

    fun error(format: String?, arg: Any?)

    fun error(format: String?, arg1: Any?, arg2: Any?)

    fun error(format: String?, vararg arguments: Any?)

    fun error(msg: String?, t: Throwable?)

    // SLF4J 2.x fluent API (Java default methods).
    fun atTrace(): LoggingEventBuilder = TODO()

    fun atDebug(): LoggingEventBuilder = TODO()

    fun atInfo(): LoggingEventBuilder = TODO()

    fun atWarn(): LoggingEventBuilder = TODO()

    fun atError(): LoggingEventBuilder = TODO()

    // Java interface constant; callable id gains `.Companion` (see README).
    companion object {
        const val ROOT_LOGGER_NAME: String = "ROOT"
    }
}

object LoggerFactory {
    fun getLogger(name: String?): Logger = TODO()

    fun getLogger(clazz: Class<*>?): Logger = TODO()
}

object MDC {
    fun put(key: String?, `val`: String?) {
        TODO()
    }

    fun get(key: String?): String? = TODO()

    fun remove(key: String?) {
        TODO()
    }

    fun clear() {
        TODO()
    }

    fun getCopyOfContextMap(): Map<String, String>? = TODO()

    fun setContextMap(contextMap: Map<String, String>?) {
        TODO()
    }

    fun putCloseable(key: String?, `val`: String?): MDCCloseable = TODO()

    class MDCCloseable : Closeable {
        override fun close() {
            TODO()
        }
    }
}
