package test

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.test.runTest
import org.junit.Test

class TestDispatcherNotInjectedPositive {
    @Test
    fun works() = runTest {
        val repository = Repository(Dispatchers.IO)
        assert(repository.load() == "data")
    }
}
