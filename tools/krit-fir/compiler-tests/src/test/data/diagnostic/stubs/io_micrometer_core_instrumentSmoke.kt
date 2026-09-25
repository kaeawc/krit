// Smoke: Micrometer counters via registry shortcuts, the builder, and Metrics.
package stubs

import io.micrometer.core.instrument.Counter
import io.micrometer.core.instrument.MeterRegistry
import io.micrometer.core.instrument.Metrics
import io.micrometer.core.instrument.Timer

fun countRequests(registry: MeterRegistry) {
    val counter: Counter = registry.counter("requests", "path", "/")
    counter.increment()
    counter.increment(2.0)
    Counter.builder("built").tag("k", "v").description("d").register(registry).increment()
    Metrics.counter("global").increment()
    Metrics.globalRegistry.counter("global2").increment()
    val timer: Timer = registry.timer("latency")
    timer.record(Runnable { <!PrintlnInProduction!>println<!>("timed") })
    <!PrintlnInProduction!>println<!>(counter.count())
}
