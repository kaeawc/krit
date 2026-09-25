// Smoke: MDCContext added to a coroutine context alongside a dispatcher.
package stubs

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.slf4j.MDCContext
import kotlinx.coroutines.withContext
import org.slf4j.MDC

suspend fun withMdc() {
    MDC.put("requestId", "42")
    withContext(Dispatchers.IO + MDCContext()) {
        println(MDC.get("requestId"))
    }
    val explicit = MDCContext(mapOf("k" to "v"))
    println(explicit.contextMap)
}
