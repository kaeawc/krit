package test

interface Logger {
    fun atInfo(): EventBuilder
}

interface EventBuilder {
    fun addKeyValue(key: String, value: Any): EventBuilder
    fun log(message: String)
}

fun record(logger: Logger, id: String, req: String) {
    logger.atInfo()
        .addKeyValue("user_id", id)
        .addKeyValue("account_id", id)
        .addKeyValue("session_id", id)
        .addKeyValue("requestId", req)
        .log("done")
}
