package observability

import org.slf4j.MDC

data class Request(val id: String)

fun process(req: Request) {
    println(req.id)
}

fun handle(req: Request) {
    MDC.put("reqId", req.id)
    process(req)
}
