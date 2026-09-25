// Compiler-test source stubs; never packaged in the production artifact.
package org.slf4j.spi

import java.util.function.Supplier

interface LoggingEventBuilder {
    fun setCause(cause: Throwable?): LoggingEventBuilder

    fun addArgument(p: Any?): LoggingEventBuilder

    fun addArgument(objectSupplier: Supplier<*>): LoggingEventBuilder

    fun addKeyValue(key: String?, value: Any?): LoggingEventBuilder

    fun setMessage(message: String?): LoggingEventBuilder

    fun setMessage(messageSupplier: Supplier<String>): LoggingEventBuilder

    fun log()

    fun log(message: String?)

    fun log(format: String?, arg: Any?)

    fun log(format: String?, arg0: Any?, arg1: Any?)

    fun log(format: String?, vararg args: Any?)

    fun log(messageSupplier: Supplier<String>)
}
