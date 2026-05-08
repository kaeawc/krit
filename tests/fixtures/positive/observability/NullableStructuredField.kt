package test

interface Logger {
    fun atInfo(): Event
}

interface Event {
    fun addKeyValue(key: String, value: Any?): Event
    fun log(message: String)
}

data class User(val id: String)

fun handle(logger: Logger, user: User?) {
    logger.atInfo().addKeyValue("user_id", user?.id).log("ready")
}
