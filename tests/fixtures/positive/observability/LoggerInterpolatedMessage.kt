package test

interface Logger {
    fun info(message: String)
}

fun recordLogin(logger: Logger, id: String) {
    logger.info("user $id logged in")
}
