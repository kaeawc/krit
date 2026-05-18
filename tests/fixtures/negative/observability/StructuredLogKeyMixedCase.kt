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
    // Real code uses three snake_case keys. The migration note below
    // mentions a camelCase key inside a line comment. A lexical scanner
    // that ignores comment state would count that as a real key, push
    // snake_case past the 70% majority threshold, and flag the
    // camelCase entry as a false-positive mixed-case finding.
    // Migration note: addKeyValue("requestId", id) was renamed to "request_id".
    logger.atInfo()
        .addKeyValue("user_id", id)
        .addKeyValue("account_id", id)
        .addKeyValue("session_id", id)
        .log("done")
    map.put("requestId", id) // map.put is not MDC.put; ignored by the regex either way.
}

fun recordWithRawStringNote(logger: Logger, id: String) {
    // Same shape as `record`, but the camelCase example lives inside a
    // triple-quoted raw string. A scanner that does not track raw-string
    // state would still treat it as a real call.
    val migrationNote = """
        Legacy usage:
          addKeyValue("requestId", id)
    """.trimIndent()
    println(migrationNote)
    logger.atInfo()
        .addKeyValue("user_id", id)
        .addKeyValue("account_id", id)
        .addKeyValue("session_id", id)
        .log("done")
}

fun recordWithBlockCommentNote(logger: Logger, id: String) {
    /*
     * Example camelCase usage from the SDK README:
     *   addKeyValue("requestId", id)
     */
    logger.atInfo()
        .addKeyValue("user_id", id)
        .addKeyValue("account_id", id)
        .addKeyValue("session_id", id)
        .log("done")
}
