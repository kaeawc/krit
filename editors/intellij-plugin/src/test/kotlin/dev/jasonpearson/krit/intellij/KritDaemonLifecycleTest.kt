package dev.jasonpearson.krit.intellij

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue

class KritDaemonLifecycleTest {
    @Test
    fun `a daemon the plugin started is stopped on close`() {
        val ownership = KritDaemonOwnership<String>()
        assertTrue(ownership.beginStart())
        assertFalse(ownership.finishStart("/bin/krit"))
        assertEquals("/bin/krit", ownership.release())
    }

    @Test
    fun `a daemon someone else started is left running`() {
        val ownership = KritDaemonOwnership<String>()
        assertTrue(ownership.beginStart())
        assertFalse(ownership.finishStart(null))
        assertNull(ownership.release())
    }

    @Test
    fun `closing while the daemon starts stops it once it is up`() {
        val ownership = KritDaemonOwnership<String>()
        assertTrue(ownership.beginStart())
        assertNull(ownership.release())
        assertTrue(ownership.finishStart("/bin/krit"))
    }

    @Test
    fun `starts at most once and never after close`() {
        val ownership = KritDaemonOwnership<String>()
        assertTrue(ownership.beginStart())
        assertFalse(ownership.beginStart())
        ownership.finishStart("/bin/krit")
        assertFalse(ownership.beginStart())

        val closed = KritDaemonOwnership<String>()
        closed.release()
        assertFalse(closed.beginStart())
    }

    @Test
    fun `release hands out the stop only once`() {
        val ownership = KritDaemonOwnership<String>()
        ownership.beginStart()
        ownership.finishStart("/bin/krit")
        assertEquals("/bin/krit", ownership.release())
        assertNull(ownership.release())
    }

    @Test
    fun `classifies daemon start output`() {
        assertEquals(
            KritDaemonStart.Result.STARTED,
            KritDaemonStart.classify(0, "krit-daemon started, PID 42, socket /repo/.krit/daemon.sock\n"),
        )
        assertEquals(
            KritDaemonStart.Result.ALREADY_RUNNING,
            KritDaemonStart.classify(0, "krit-daemon already running, PID 42\n"),
        )
        assertEquals(KritDaemonStart.Result.FAILED, KritDaemonStart.classify(1, ""))
    }
}
