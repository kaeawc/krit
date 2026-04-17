package observability

import kotlinx.coroutines.launch
import kotlinx.coroutines.slf4j.MDCContext
import org.slf4j.MDC

fun handle() {}

fun startWork(id: String) {
    MDC.put("reqId", id)
    launch(MDCContext()) {
        handle()
    }
}
