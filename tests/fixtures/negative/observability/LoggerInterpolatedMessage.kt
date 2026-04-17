package test

interface Logger {
    fun info(message: String, arg: Any)
}

object Timber {
    fun i(message: String) {}
}

fun recordLogin(logger: Logger, id: String) {
    logger.info("user {} logged in", id)
    Timber.i("user $id logged in")
}
