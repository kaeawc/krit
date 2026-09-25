// Smoke: Log4j 2 LogManager loggers with parameterized and throwable overloads.
package stubs

import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger

class Log4jOwner

private val log4jLogger: Logger = LogManager.getLogger(Log4jOwner::class.java)

fun log4jLogging(error: Throwable) {
    log4jLogger.info("plain")
    log4jLogger.info("one {}", 1)
    log4jLogger.debug("two {} {}", 1, 2)
    log4jLogger.warn("failed", error)
    log4jLogger.error("failed", error)
    log4jLogger.trace("trace")
    if (log4jLogger.isDebugEnabled) log4jLogger.debug(Any())
    LogManager.getLogger("named").info("named")
    LogManager.getLogger().info("caller")
}
