package observability

object LoggerFactory {
    fun getLogger(target: Any): Logger = Logger()
}

class Logger {
    fun info(message: String) {}
}

class Handler {
    fun handle() {
        val log = LoggerFactory.getLogger(javaClass)
        log.info("handle")
    }
}
