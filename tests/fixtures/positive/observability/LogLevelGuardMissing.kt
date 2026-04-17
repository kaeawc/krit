package observability

import org.slf4j.Logger as Slf4jLogger

interface Logger {
    val isDebugEnabled: Boolean
    val isTraceEnabled: Boolean

    fun debug(message: String)
    fun debug(marker: Marker, message: String)
    fun trace(message: String)
}

interface Marker

data class Thing(val payload: String)

fun serialize(thing: Thing): String = thing.payload.uppercase()

fun logPayload(logger: Logger, thing: Thing) {
    logger.debug("payload=${serialize(thing)}")
    logger.trace("payload=${serialize(thing)}")

    val shouldSkip = thing.payload.isBlank()
    if (!logger.isDebugEnabled && shouldSkip) return
    logger.debug("payload=${serialize(thing)}")
}

fun logPayload(logger: Logger, thing: Thing, includePayload: Boolean) {
    when {
        logger.isDebugEnabled, includePayload -> logger.debug("payload=${serialize(thing)}")
    }
}

fun logPayload(logger: Logger, thing: Thing, shouldLog: Boolean, forceLog: Boolean) {
    if (logger.isDebugEnabled && shouldLog || forceLog) {
        logger.debug("payload=${serialize(thing)}")
    }
}

fun logPayload(logger: Logger, marker: Marker, thing: Thing) {
    logger.debug(marker, "payload=${serialize(thing)}")
}

fun logAuditPayload(audit: Slf4jLogger, thing: Thing) {
    audit.debug("payload=${serialize(thing)}")
}

fun logQualifiedAuditPayload(audit: org.slf4j.Logger, thing: Thing) {
    audit.debug("payload=${serialize(thing)}")
}

class AuditService(
    private val audit: org.slf4j.Logger
) {
    fun logPayload(thing: Thing) {
        audit.debug("payload=${serialize(thing)}")
    }
}

fun logger(): org.slf4j.Logger = TODO()

fun logLocalAuditPayload(thing: Thing) {
    val audit: org.slf4j.Logger = logger()
    audit.debug("payload=${serialize(thing)}")
}

fun logKotlinLoggerPayload(logger: mu.KLogger, thing: Thing) {
    logger.debug { "payload=${serialize(thing)}" }
}
