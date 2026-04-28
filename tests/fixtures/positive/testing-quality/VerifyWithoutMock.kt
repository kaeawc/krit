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

    @Test
    fun ignoresNestedArgumentReceivers() {
        val api = RealApi()
        verify { api.consume(DataSet.VALUE) }
    }
}

object DataSet {
    val VALUE = "data"
}

fun RealApi.consume(value: String) {
    get()
}
