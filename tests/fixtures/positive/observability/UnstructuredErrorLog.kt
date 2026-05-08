package test

interface Logger {
    fun error(message: String)
}

fun record(logger: Logger, e: Throwable) {
    logger.error("failure: $e")
}
