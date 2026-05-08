package test

import io.mockk.mockk
import io.mockk.spyk
import io.mockk.verify
import org.junit.Before
import org.junit.Test

interface Api {
    fun get(): String
    fun consume(value: String)
}

class VerifyWithoutMockNegative {
    private val fieldMock = mockk<Api>()
    private lateinit var setupMock: Api

    @Before
    fun setUp() {
        setupMock = buildMock()
    }

    @Test
    fun works() {
        val api = mockk<Api>()
        api.get()
        verify { api.get() }
        verify { fieldMock.consume(DataSet.VALUE) }
        verify { setupMock.consume(DataSet.VALUE) }
        verify { spyk(api).consume(DataSet.VALUE) }
    }

    private fun buildMock(): Api {
        val mock = mockk<Api>()
        return mock
    }
}

class VerifyWithoutMockNonMockKNegative {
    lateinit var staticIdentityUtil: MockedStaticIdentityUtil

    @Test
    fun works() {
        staticIdentityUtil.verify { IdentityUtil.saveIdentity(DataSet.VALUE) }
    }
}

object DataSet {
    val VALUE = "data"
}

object IdentityUtil {
    fun saveIdentity(value: String) {}
}

class MockedStaticIdentityUtil {
    fun verify(block: () -> Unit) {}
}
