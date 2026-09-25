// Smoke: LoggingEventBuilder reached from Logger.atInfo()/atError().
package stubs

import org.slf4j.Logger
import org.slf4j.spi.LoggingEventBuilder

fun fluent(logger: Logger, error: Throwable) {
    val builder: LoggingEventBuilder = logger.atError()
    builder.setMessage("failed {}").addArgument(1).addArgument { "lazy" }.setCause(error).log()
    logger.atWarn().log("warn {} {}", 1, 2)
}
