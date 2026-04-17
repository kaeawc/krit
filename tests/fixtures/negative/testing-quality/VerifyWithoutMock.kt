package test

import io.mockk.mockk
import io.mockk.verify
import org.junit.Test

interface Api {
    fun get(): String
}

class VerifyWithoutMockNegative {
    @Test
    fun works() {
        val api = mockk<Api>()
        api.get()
        verify { api.get() }
    }
}
