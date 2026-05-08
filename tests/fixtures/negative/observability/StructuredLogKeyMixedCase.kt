package test

interface Logger {
    fun atInfo(): EventBuilder
}

interface EventBuilder {
    fun addKeyValue(key: String, value: Any): EventBuilder
    fun log(message: String)
}

class MapLike {
    fun put(key: String, value: String) {}
}

fun record(logger: Logger, map: MapLike, id: String) {
    logger.atInfo()
        .addKeyValue("user_id", id)
        .addKeyValue("account_id", id)
        .addKeyValue("request_id", id)
        .log("done")
    map.put("requestId", id)
}
