package observability

import kotlinx.coroutines.launch
import kotlinx.coroutines.slf4j.MDCContext

interface Logger {
    fun info(message: String)
}

fun startWork(logger: Logger) {
    launch(MDCContext()) {
        logger.info("work started")
    }
}
