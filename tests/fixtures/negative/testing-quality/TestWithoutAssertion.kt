package test

import org.junit.Ignore
import org.junit.Test
import org.junit.Assert.assertEquals
import org.thoughtcrime.securesms.testing.assertIsSize

class TestWithoutAssertionNegative {
    @Test
    fun loads() {
        val x = 42
        assertEquals(42, x)
    }

    @Test
    fun signalStyleInfixAssertion() {
        val values = listOf(1, 2)
        values assertIsSize 2
    }

    @Test(expected = IllegalArgumentException::class)
    fun expectedException() {
        parse("")
    }
}

@Ignore("manual preview")
class TestWithoutAssertionIgnoredNegative {
    @Test
    fun preview() {
        println("manual")
    }
}
