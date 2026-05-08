package observability

import org.slf4j.MDC

data class Request(val id: String)

fun process(req: Request) {
    println(req.id)
}

fun handleWithRemove(req: Request) {
    MDC.put("reqId", req.id)
    try {
        process(req)
    } finally {
        MDC.remove("reqId")
    }
}

fun handleWithCloseable(req: Request) {
    MDC.putCloseable("reqId", req.id).use {
        process(req)
    }
}
