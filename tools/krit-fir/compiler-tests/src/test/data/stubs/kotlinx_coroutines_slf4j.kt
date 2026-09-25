// Compiler-test source stubs; never packaged in the production artifact.
package kotlinx.coroutines.slf4j

import kotlin.coroutines.AbstractCoroutineContextElement
import kotlin.coroutines.CoroutineContext
import kotlinx.coroutines.ThreadContextElement
import org.slf4j.MDC

typealias MDCContextMap = Map<String, String>?

class MDCContext(
    val contextMap: MDCContextMap = MDC.getCopyOfContextMap(),
) : ThreadContextElement<MDCContextMap>, AbstractCoroutineContextElement(Key) {
    companion object Key : CoroutineContext.Key<MDCContext>

    override fun updateThreadContext(context: CoroutineContext): MDCContextMap = TODO()

    override fun restoreThreadContext(context: CoroutineContext, oldState: MDCContextMap) {
        TODO()
    }
}
