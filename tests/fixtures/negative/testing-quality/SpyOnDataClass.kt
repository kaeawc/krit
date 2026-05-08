package test

import io.mockk.spyk
import org.junit.Test

open class Service {
    fun compute(): Int = 42
}

class SpyOnDataClassNegative {
    @Test
    fun works() {
        val service = spyk<Service>()
        assert(service.compute() == 42)
    }
}
