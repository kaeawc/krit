package observability

import kotlinx.coroutines.launch

interface Logger {
    fun info(message: String)
}

fun startWork(logger: Logger) {
    launch {
        logger.info("work started")
    }
}
