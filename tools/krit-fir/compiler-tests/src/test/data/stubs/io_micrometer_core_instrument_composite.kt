// Compiler-test source stubs; never packaged in the production artifact.
package io.micrometer.core.instrument.composite

import io.micrometer.core.instrument.MeterRegistry

open class CompositeMeterRegistry : MeterRegistry() {
    fun add(registry: MeterRegistry): CompositeMeterRegistry = TODO()

    fun remove(registry: MeterRegistry): CompositeMeterRegistry = TODO()
}
