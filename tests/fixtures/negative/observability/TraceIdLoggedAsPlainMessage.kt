package test

interface Logger {
    fun info(message: String)
}

object MDC {
    fun put(key: String, value: String) {}
}

class Builder {
    fun info(message: String) {}
}

fun record(logger: Logger, builder: Builder, traceId: String, traceFile: String) {
    MDC.put("trace_id", traceId)
    logger.info("processed")
    logger.info("trace file=$traceFile")
    builder.info("trace=$traceId")
}
