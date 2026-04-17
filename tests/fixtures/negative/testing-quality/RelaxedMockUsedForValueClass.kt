package test

import io.mockk.mockk
import org.junit.Test

interface Api {
    fun get(): String
}

class RelaxedMockNegative {
    @Test
    fun works() {
        val api = mockk<Api>(relaxed = true)
        assert(api != null)
    }
}
