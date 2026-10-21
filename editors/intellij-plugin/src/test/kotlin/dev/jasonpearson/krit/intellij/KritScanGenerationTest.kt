package dev.jasonpearson.krit.intellij

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

class KritScanGenerationTest {
    @Test
    fun `result publishes when generation unchanged`() {
        assertTrue(KritScanGeneration.shouldPublish(captured = 3, current = 3))
    }

    @Test
    fun `result is dropped when a newer generation exists`() {
        assertFalse(KritScanGeneration.shouldPublish(captured = 3, current = 4))
    }

    @Test
    fun `bump supersedes a captured generation`() {
        val gen = KritScanGeneration()
        val captured = gen.current()
        assertFalse(gen.isSuperseded(captured))
        gen.bump()
        assertTrue(gen.isSuperseded(captured))
        assertFalse(gen.isSuperseded(gen.current()))
    }

    @Test
    fun `await returns done when step completes`() {
        val result = KritScanWait.await(1_000, 100, { false }) { true }
        assertEquals(KritWaitResult.Done, result)
    }

    @Test
    fun `await times out after the full budget in slices`() {
        val slices = mutableListOf<Long>()
        val result = KritScanWait.await(250, 100, { false }) { slices.add(it); false }
        assertEquals(KritWaitResult.TimedOut, result)
        assertEquals(listOf(100L, 100L, 50L), slices)
    }

    @Test
    fun `await reports cancelled and stops waiting`() {
        var steps = 0
        val result = KritScanWait.await(10_000, 100, { steps >= 2 }) { steps++; false }
        assertEquals(KritWaitResult.Cancelled, result)
        assertEquals(2, steps)
    }

    @Test
    fun `await checks cancel before the first wait`() {
        var stepped = false
        val result = KritScanWait.await(1_000, 100, { true }) { stepped = true; true }
        assertEquals(KritWaitResult.Cancelled, result)
        assertFalse(stepped)
    }
}
