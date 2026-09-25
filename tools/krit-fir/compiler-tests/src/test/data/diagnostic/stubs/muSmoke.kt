// Smoke: kotlin-logging 1.x-3.x (mu) lambda logging on an SLF4J-backed KLogger.
package stubs

import mu.KLogger
import mu.KotlinLogging

private val muLogger: KLogger = KotlinLogging.logger {}

fun muLogging(error: Throwable) {
    muLogger.debug { "debug" }
    muLogger.info { "info" }
    muLogger.warn(error) { "warn" }
    muLogger.error(error) { "error" }
    muLogger.info("plain slf4j {}", 1)
    val slf4j: org.slf4j.Logger = muLogger
    println(slf4j.name)
}
