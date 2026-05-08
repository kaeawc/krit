package test

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.test.runTest
import org.junit.Test

class TestDispatcherNotInjectedPositive {
    @Test
    fun works() = runTest {
        val result = withContext(Dispatchers.IO) { "data" }
        assert(result == "data")
    }
}
