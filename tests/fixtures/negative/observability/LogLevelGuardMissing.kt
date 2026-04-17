package observability

interface Logger {
    val isDebugEnabled: Boolean
    val isTraceEnabled: Boolean

    fun debug(message: String)
    fun debug(marker: Marker, message: String)
    fun debug(template: String, value: Any?)
    fun trace(message: String)
    fun trace(template: String, value: Any?)
}

interface Marker

interface JavaLikeLogger {
    fun isDebugEnabled(): Boolean
    fun isTraceEnabled(): Boolean

    fun debug(message: String)
    fun trace(message: String)
}

interface AuditSink {
    fun debug(message: String)
    fun trace(message: String)
}

data class Thing(val payload: String)

fun serialize(thing: Thing): String = thing.payload.uppercase()

fun logPayload(logger: Logger, thing: Thing) {
    if (logger.isDebugEnabled) {
        logger.debug("payload={}", serialize(thing))
        logger.debug("payload=${serialize(thing)}")
    }

    if (logger.isDebugEnabled == true) {
        logger.debug("payload=${serialize(thing)}")
    }
    if (true == logger.isDebugEnabled) {
        logger.debug("payload=${serialize(thing)}")
    }

    if (!logger.isDebugEnabled) {
        println("skipping payload")
    } else {
        logger.debug("payload=${serialize(thing)}")
    }
    if (false == logger.isDebugEnabled) {
        println("skipping payload")
    } else {
        logger.debug("payload=${serialize(thing)}")
    }

    if (!logger.isDebugEnabled) return
    if (logger.isDebugEnabled != true) return
    if (false == logger.isDebugEnabled) return

    val includePayload = thing.payload.isNotBlank()
    if (includePayload && logger.isDebugEnabled) {
        logger.debug("payload=${serialize(thing)}")
    }

    if ((logger.isDebugEnabled && includePayload) || logger.isDebugEnabled) {
        logger.debug("payload=${serialize(thing)}")
    }

    if (logger.isTraceEnabled) {
        logger.trace("payload={}", serialize(thing))
        logger.trace("payload=${serialize(thing)}")
    }
}

fun logPayload(logger: Logger, marker: Marker, thing: Thing) {
    if (logger.isDebugEnabled) {
        logger.debug(marker, "payload=${serialize(thing)}")
    }
}

fun logPayload(javaLogger: JavaLikeLogger, thing: Thing) {
    if (javaLogger.isDebugEnabled()) {
        javaLogger.debug("payload=${serialize(thing)}")
    }

    if (javaLogger.isDebugEnabled() != false) {
        javaLogger.debug("payload=${serialize(thing)}")
    }

    if (!javaLogger.isDebugEnabled()) {
        println("skipping payload")
    } else {
        javaLogger.debug("payload=${serialize(thing)}")
    }

    if (!javaLogger.isTraceEnabled()) return
    javaLogger.trace("payload=${serialize(thing)}")

    if ((!logger.isDebugEnabled || thing.payload.isBlank()) && !logger.isDebugEnabled) return
    logger.debug("payload=${serialize(thing)}")

    when {
        javaLogger.isDebugEnabled() -> javaLogger.debug("payload=${serialize(thing)}")
    }

    when (javaLogger.isTraceEnabled()) {
        false -> return
    }
    javaLogger.trace("payload=${serialize(thing)}")

    when {
        logger.isDebugEnabled,
        logger.isDebugEnabled && thing.payload.isNotBlank() -> logger.debug("payload=${serialize(thing)}")
    }

    when {
        !logger.isDebugEnabled -> println("skipping payload")
        else -> logger.debug("payload=${serialize(thing)}")
    }

    when (javaLogger.isDebugEnabled()) {
        true,
        true -> javaLogger.debug("payload=${serialize(thing)}")
    }

    when (javaLogger.isTraceEnabled() == true) {
        true -> javaLogger.trace("payload=${serialize(thing)}")
    }

    when (javaLogger.isTraceEnabled()) {
        false -> println("skipping payload")
        else -> javaLogger.trace("payload=${serialize(thing)}")
    }
}

fun logPayload(logger: Logger?, thing: Thing, skip: Boolean) {
    if (logger?.isDebugEnabled == true) {
        logger?.debug("payload=${serialize(thing)}")
    }

    if (logger?.isDebugEnabled != true && skip) return
    if (logger?.isDebugEnabled != true) return
    logger?.debug("payload=${serialize(thing)}")
}

fun logKotlinLoggerPayload(logger: mu.KLogger, thing: Thing) {
    if (logger.isDebugEnabled) {
        logger.debug { "payload=${serialize(thing)}" }
    }
}

fun logAuditPayload(audit: AuditSink, thing: Thing) {
    audit.debug("payload=${serialize(thing)}")
    audit.trace("payload=${serialize(thing)}")
}

fun logQualifiedAuditPayload(audit: com.example.AuditSink, thing: Thing) {
    audit.debug("payload=${serialize(thing)}")
    audit.trace("payload=${serialize(thing)}")
}

class AuditService(
    private val audit: com.example.AuditSink
) {
    fun logPayload(thing: Thing) {
        audit.debug("payload=${serialize(thing)}")
    }
}
