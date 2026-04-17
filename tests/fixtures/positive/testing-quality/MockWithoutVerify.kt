package test

import io.mockk.mockk
import org.junit.Test

interface Api {
    fun get(): String
}

class Repo(private val api: Api) {
    fun load(): String = api.get()
}

class MockWithoutVerifyPositive {
    @Test
    fun load() {
        val api = mockk<Api>()
        val repo = Repo(api)

        repo.load()
    }
}
