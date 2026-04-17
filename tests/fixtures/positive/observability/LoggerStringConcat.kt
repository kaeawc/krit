package test

interface Logger {
    fun info(message: String)
}

fun recordValue(logger: Logger, value: Int) {
    logger.info("value=" + value)
}
