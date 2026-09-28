package dev.jasonpearson.krit.fir

import org.junit.jupiter.api.Test
import java.util.concurrent.CountDownLatch
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicBoolean
import kotlin.test.assertEquals
import kotlin.test.assertTrue

class ParentWatchTest {
    @Test
    fun parentDeathStopsWatch() {
        val alive = AtomicBoolean(true)
        val stopped = CountDownLatch(1)
        val watcher = ParentWatch(alive::get, stopped::countDown, 10).start()
        assertTrue(!stopped.await(30, TimeUnit.MILLISECONDS))
        alive.set(false)
        assertTrue(stopped.await(1, TimeUnit.SECONDS))
        watcher.join(1_000)
        assertTrue(!watcher.isAlive)
    }

    @Test
    fun ephemeralIdleTimeoutIsShorter() {
        assertEquals(30 * 60 * 1_000, daemonIdleMillis(null))
        assertEquals(2 * 60 * 1_000, daemonIdleMillis(123))
    }
}
