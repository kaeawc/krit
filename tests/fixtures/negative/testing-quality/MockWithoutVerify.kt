package test

import io.mockk.every
import io.mockk.mockk
import io.mockk.verify
import org.junit.Test

interface Api {
    fun get(): String
}

class Repo(private val api: Api) {
    fun load(): String = api.get()
}

class MockWithoutVerifyNegative {
    @Test
    fun load() {
        val api = mockk<Api>()
        every { api.get() } returns "data"
        val repo = Repo(api)

        repo.load()

        verify { api.get() }
    }
}
