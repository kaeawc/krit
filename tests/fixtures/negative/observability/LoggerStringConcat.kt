package test

interface Logger {
    fun info(message: String, arg: Any)
}

fun recordValue(logger: Logger, value: Int) {
    logger.info("value={}", value)
}
