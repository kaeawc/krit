package test

import io.mockk.verify
import org.junit.Test

class RealApi {
    fun get(): String = "data"
}

class VerifyWithoutMockPositive {
    @Test
    fun works() {
        val api = RealApi()
        api.get()
        verify { api.get() }
    }
}
