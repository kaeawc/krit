package test

interface Logger {
    fun info(message: String)
}

fun record(logger: Logger, traceId: String) {
    logger.info("trace=$traceId processed")
}
