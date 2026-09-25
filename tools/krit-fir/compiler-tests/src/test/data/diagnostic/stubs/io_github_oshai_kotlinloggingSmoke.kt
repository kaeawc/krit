// Smoke: kotlin-logging 5+ lazy lambda logging.
package stubs

import io.github.oshai.kotlinlogging.KLogger
import io.github.oshai.kotlinlogging.KotlinLogging

private val oshaiLogger: KLogger = KotlinLogging.logger {}

fun oshaiLogging(error: Throwable) {
    oshaiLogger.trace { "trace" }
    oshaiLogger.debug { "debug" }
    oshaiLogger.info { "info ${oshaiLogger.name}" }
    oshaiLogger.warn(error) { "warn" }
    oshaiLogger.error(error) { "error" }
    if (oshaiLogger.isDebugEnabled()) KotlinLogging.logger("named").info { "named" }
}
