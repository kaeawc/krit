// Smoke: SLF4J logger overloads, the fluent atX() API, and MDC.
package stubs

import org.slf4j.Logger
import org.slf4j.LoggerFactory
import org.slf4j.MDC

class Slf4jOwner

private val slf4jLogger: Logger = LoggerFactory.getLogger(Slf4jOwner::class.java)

fun slf4jLogging(error: Throwable, nullable: String?) {
    slf4jLogger.info("plain")
    slf4jLogger.info("one {}", 1)
    slf4jLogger.info("two {} {}", 1, 2)
    slf4jLogger.info("many {} {} {}", 1, 2, 3)
    slf4jLogger.info("failed", error)
    slf4jLogger.debug(nullable)
    slf4jLogger.warn("warn {}", nullable)
    slf4jLogger.error("error", error)
    slf4jLogger.trace("trace")
    if (slf4jLogger.isDebugEnabled) slf4jLogger.debug("debug")
    slf4jLogger.atInfo().addKeyValue("k", "v").setCause(error).log("fluent {}", 1)
    slf4jLogger.atDebug().log { "lazy" }
    LoggerFactory.getLogger("named").warn("named {}", slf4jLogger.name)
    MDC.put("requestId", "42")
    MDC.putCloseable("scoped", "1").use { println(MDC.get("scoped")) }
    val copy: Map<String, String>? = MDC.getCopyOfContextMap()
    MDC.remove("requestId")
    MDC.clear()
    println(copy)
}
