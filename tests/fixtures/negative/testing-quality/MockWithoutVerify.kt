package test

import io.mockk.every
import io.mockk.mockk
import io.mockk.verify
import org.junit.Test
import org.mockito.kotlin.mock
import org.mockito.kotlin.whenever

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

    @Test
    fun loadWithMockitoWhenever() {
        val api = mock<Api>()
        whenever(api.get()).thenReturn("data")
        val repo = Repo(api)

        repo.load()
    }

    @Test
    fun loadWithConstructorInjection() {
        val api = mock<Api>()
        val repo = Repo(api)

        repo.load()
    }
}
