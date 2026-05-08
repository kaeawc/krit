package test

import io.mockk.mockk
import org.junit.Test

class RelaxedMockPositive {
    @Test
    fun works() {
        val id = mockk<Long>(relaxed = true)
        assert(id != null)
    }
}
