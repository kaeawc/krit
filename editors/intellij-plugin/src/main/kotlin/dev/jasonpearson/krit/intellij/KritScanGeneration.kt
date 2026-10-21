package dev.jasonpearson.krit.intellij

import java.util.concurrent.atomic.AtomicLong

/**
 * Tracks which scan is the newest one the user is waiting for. A document
 * change bumps the generation; a running scan captured the generation it
 * started under and must not publish results (or touch the status surface)
 * once the counter has moved past it.
 */
class KritScanGeneration {
    private val counter = AtomicLong(0)

    fun current(): Long = counter.get()

    fun bump(): Long = counter.incrementAndGet()

    fun isSuperseded(captured: Long): Boolean = !shouldPublish(captured, current())

    companion object {
        /** A result may be published only when no newer scan has been requested since it started. */
        fun shouldPublish(captured: Long, current: Long): Boolean = captured == current
    }
}

/** Outcome of waiting for a process (or similar) with a deadline and a cancel check. */
enum class KritWaitResult { Done, TimedOut, Cancelled }

object KritScanWait {
    const val POLL_INTERVAL_MS = 100L

    /**
     * Waits up to [timeoutMs] in slices of at most [pollMs], checking [isCancelled]
     * before each slice. [waitStep] blocks for at most the given milliseconds and
     * returns true when the awaited work has finished.
     */
    fun await(
        timeoutMs: Long,
        pollMs: Long = POLL_INTERVAL_MS,
        isCancelled: () -> Boolean,
        waitStep: (Long) -> Boolean,
    ): KritWaitResult {
        var remaining = timeoutMs
        while (true) {
            if (isCancelled()) return KritWaitResult.Cancelled
            if (remaining <= 0) return KritWaitResult.TimedOut
            val slice = minOf(pollMs, remaining)
            if (waitStep(slice)) return KritWaitResult.Done
            remaining -= slice
        }
    }
}
