// Smoke: Prometheus simpleclient counter built with the static builder.
package stubs

import io.prometheus.client.Counter

val RequestCounter: Counter = Counter.build()
    .name("requests_total")
    .help("Total requests.")
    .labelNames("path")
    .register()

fun countPrometheus() {
    RequestCounter.inc()
    RequestCounter.inc(2.0)
    RequestCounter.labels("/").inc()
    Counter.build("other_total", "Other.").create().inc()
}
