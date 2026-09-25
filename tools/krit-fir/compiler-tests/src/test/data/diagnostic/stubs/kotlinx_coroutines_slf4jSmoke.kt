// Smoke: MDCContext added to a coroutine context alongside a dispatcher.
package stubs

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.slf4j.MDCContext
import kotlinx.coroutines.withContext
import org.slf4j.MDC

suspend fun withMdc() {
    MDC.put("requestId", "42")
    withContext(Dispatchers.IO + MDCContext()) {
        <!PrintlnInProduction!>println<!>(MDC.get("requestId"))
    }
    val explicit = MDCContext(mapOf("k" to "v"))
    <!PrintlnInProduction!>println<!>(explicit.contextMap)
}
