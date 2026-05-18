package test

interface Logger {
    fun atInfo(): Event
}

interface Event {
    fun addKeyValue(key: String, value: Any?): Event
    fun log(message: String)
}

data class User(val id: String, val tier: String)

fun handle(logger: Logger, user: User) {
    logger.atInfo().addKeyValue("user_id", user.id).log("ready")
    logger.atInfo().addKeyValue("user_tier", user.tier).log("ready")
}

fun nullable(logger: Logger, user: User?) {
    logger.atInfo().addKeyValue("user_id", user?.id ?: "anonymous").log("ready")
}

// String literal containing the bytes "?." must not be mistaken for a real
// safe-call operator in the AST.
fun stringLiteralWithSafeCallBytes(logger: Logger) {
    logger.atInfo().addKeyValue("hint", "use ?. operator").log("ready")
}
