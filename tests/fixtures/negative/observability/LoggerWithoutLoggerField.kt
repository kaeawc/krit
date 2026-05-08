package observability

object LoggerFactory {
    fun getLogger(target: Any): Logger = Logger()
}

class Logger {
    fun info(message: String) {}
}

class Handler {
    private val log = LoggerFactory.getLogger(javaClass)

    fun handle() {
        log.info("handle")
    }
}
