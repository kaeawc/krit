package observability

import kotlinx.coroutines.launch
import org.slf4j.MDC

fun handle() {}

fun startWork(id: String) {
    MDC.put("reqId", id)
    launch {
        handle()
    }
}
