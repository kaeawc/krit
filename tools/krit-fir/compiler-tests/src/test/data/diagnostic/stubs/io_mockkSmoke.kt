// Smoke: mockk/spyk, infix stubbing (returns/answers/throws/just Runs), verify.
package stubs

import io.mockk.Runs
import io.mockk.coEvery
import io.mockk.coVerify
import io.mockk.every
import io.mockk.just
import io.mockk.mockk
import io.mockk.spyk
import io.mockk.verify

interface MockedService {
    fun load(id: Int): String

    fun save(value: String)

    suspend fun fetch(): Int
}

class RealService : MockedService {
    override fun load(id: Int): String = "$id"

    override fun save(value: String) {}

    override suspend fun fetch(): Int = 1
}

suspend fun mockkSmoke() {
    val service = mockk<MockedService>()
    val relaxed: MockedService = mockk(relaxed = true)
    val spy = spyk(RealService())
    every { service.load(any()) } returns "loaded"
    every { service.load(2) } answers { "answer ${firstArg<Int>()}" }
    every { service.load(3) } throws IllegalStateException("boom")
    every { service.save(any()) } just Runs
    coEvery { service.fetch() } returns 42
    service.load(1)
    service.fetch()
    verify(exactly = 1) { service.load(1) }
    verify { spy.save(eq("x")) }
    coVerify { service.fetch() }
    <!PrintlnInProduction!>println<!>(relaxed)
}
