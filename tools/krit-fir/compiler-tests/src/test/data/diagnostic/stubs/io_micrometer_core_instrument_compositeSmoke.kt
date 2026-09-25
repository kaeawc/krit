// Smoke: the global registry is a CompositeMeterRegistry.
package stubs

import io.micrometer.core.instrument.MeterRegistry
import io.micrometer.core.instrument.Metrics
import io.micrometer.core.instrument.composite.CompositeMeterRegistry

fun addRegistry(child: MeterRegistry) {
    val global: CompositeMeterRegistry = Metrics.globalRegistry
    global.add(child)
}
