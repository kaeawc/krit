package dev.jasonpearson.krit.gradle

import org.gradle.api.provider.Property
import org.gradle.api.services.BuildService
import org.gradle.api.services.BuildServiceParameters

/**
 * Shares warm krit-fir check JVMs across the krit tasks of a build, and across
 * builds of the same root, as the Kotlin daemon does for compilation.
 *
 * krit normally starts one persistent FIR daemon per source-dir set, so N
 * per-project checks keep N JVMs warm and none is reused by another project.
 * Each task here leases a slot for the length of its krit run and passes it as
 * `KRIT_FIR_DAEMON_POOL=<pool>-<slot>`. krit then keys its check daemon by the
 * slot, and every request retargets that JVM's session to the project at hand.
 * Gradle caps concurrent users at `maxParallelUsages` (the slot count), so a
 * running task always finds a free slot and never a busy daemon. The JVMs outlive
 * the build and exit on krit-fir's idle timeout (30 minutes).
 */
abstract class KritFirDaemonPool : BuildService<KritFirDaemonPool.Params> {
    interface Params : BuildServiceParameters {
        /** Stable pool name: derived from the build root, so later builds reuse the JVMs. */
        val poolName: Property<String>

        /** Number of slots, one warm JVM each. */
        val slots: Property<Int>
    }

    private val lock = Object()
    private val free = ArrayDeque<Int>()

    init {
        repeat(parameters.slots.get().coerceAtLeast(1)) { free.addLast(it) }
    }

    /** Runs [block] with a leased slot name (`<pool>-<slot>`), returning the slot afterwards. */
    fun <T> withSlot(block: (String) -> T): T {
        val slot = synchronized(lock) {
            while (free.isEmpty()) lock.wait()
            free.removeFirst()
        }
        try {
            return block("${parameters.poolName.get()}-$slot")
        } finally {
            synchronized(lock) {
                free.addFirst(slot)
                lock.notifyAll()
            }
        }
    }

    companion object {
        const val SERVICE_NAME = "kritFirDaemonPool"

        /** Read by krit (internal/firchecks.FirDaemonPoolEnv). */
        const val POOL_ENV = "KRIT_FIR_DAEMON_POOL"

        /** krit's FIR daemon heap cap (`-Xmx`), e.g. "4g". */
        const val HEAP_ENV = "KRIT_FIR_XMX"
    }
}
