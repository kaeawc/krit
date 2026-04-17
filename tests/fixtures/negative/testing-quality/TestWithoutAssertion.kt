package test

import org.junit.Test
import org.junit.Assert.assertEquals

class TestWithoutAssertionNegative {
    @Test
    fun loads() {
        val x = 42
        assertEquals(42, x)
    }
}
