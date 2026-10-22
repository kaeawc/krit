package dev.jasonpearson.krit.intellij

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull

class KritScanDebounceTest {
    @Test
    fun `defaults preserve the previous one second behaviour`() {
        assertEquals(1_000, KritScanDebounce.DEFAULT_MS)
        assertEquals(100, KritScanDebounce.MIN_MS)
        assertEquals(1_000L, KritScanDebounce.effectiveMs(null))
    }

    @Test
    fun `clamp floors values below the minimum`() {
        assertEquals(100, KritScanDebounce.clamp(99))
        assertEquals(100, KritScanDebounce.clamp(0))
        assertEquals(100, KritScanDebounce.clamp(-5))
        assertEquals(100, KritScanDebounce.clamp(Int.MIN_VALUE))
    }

    @Test
    fun `clamp keeps values at or above the minimum`() {
        assertEquals(100, KritScanDebounce.clamp(100))
        assertEquals(200, KritScanDebounce.clamp(200))
        assertEquals(3_000, KritScanDebounce.clamp(3_000))
    }

    @Test
    fun `effectiveMs clamps configured values`() {
        assertEquals(200L, KritScanDebounce.effectiveMs(200))
        assertEquals(100L, KritScanDebounce.effectiveMs(10))
        assertEquals(3_000L, KritScanDebounce.effectiveMs(3_000))
    }

    @Test
    fun `parse accepts integers with surrounding whitespace`() {
        assertEquals(250, KritScanDebounce.parse("250"))
        assertEquals(250, KritScanDebounce.parse("  250 "))
    }

    @Test
    fun `parse rejects blank and non-numeric input`() {
        assertNull(KritScanDebounce.parse(""))
        assertNull(KritScanDebounce.parse("   "))
        assertNull(KritScanDebounce.parse("fast"))
        assertNull(KritScanDebounce.parse("1.5"))
        assertNull(KritScanDebounce.parse("99999999999"))
    }

    @Test
    fun `fromInput clamps numeric input and falls back to default otherwise`() {
        assertEquals(2_000, KritScanDebounce.fromInput("2000"))
        assertEquals(100, KritScanDebounce.fromInput("5"))
        assertEquals(100, KritScanDebounce.fromInput("-300"))
        assertEquals(1_000, KritScanDebounce.fromInput(""))
        assertEquals(1_000, KritScanDebounce.fromInput("abc"))
    }
}
