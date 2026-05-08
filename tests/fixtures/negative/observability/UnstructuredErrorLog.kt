package test

interface Logger {
    fun error(message: String)
    fun error(message: String, throwable: Throwable)
}

class Builder {
    fun error(message: String) {}
}

fun record(logger: Logger, builder: Builder, e: Throwable, message: String) {
    logger.error("failure", e)
    logger.error("user error: $message")
    logger.error("user error: " + message)
    builder.error("failure: $e")
}
