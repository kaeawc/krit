package observability

import kotlinx.coroutines.GlobalScope
import kotlinx.coroutines.launch
import org.slf4j.MDC

data class Request(val id: String)

fun process(req: Request) {
    println(req.id)
}

fun handle(req: Request) {
    MDC.put("reqId", req.id)
    process(req)
}

// The MDC.clear() call below lives inside a coroutine builder lambda,
// which runs on a different thread/time than the caller. A scope-blind
// walker would treat it as a matching cleanup and silently swallow the
// real leak. The scope-aware walker stops at the lambda boundary so
// this function is still flagged.
fun handleLeaksAcrossLambda(req: Request) {
    MDC.put("reqId", req.id)
    GlobalScope.launch {
        process(req)
        MDC.clear()
    }
}

// Same shape with MDC.remove inside a local helper function rather
// than a lambda. The helper's body is its own scope and may never be
// invoked from this function, so the scope-aware walker treats it as
// a boundary and the put is still flagged as leaking.
fun handleLeaksAcrossLocalFun(req: Request) {
    MDC.put("reqId", req.id)
    fun cleanup() {
        MDC.remove("reqId")
    }
    process(req)
    // cleanup() is defined but never invoked.
}
