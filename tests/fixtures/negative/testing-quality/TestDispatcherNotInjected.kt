package test

import kotlinx.coroutines.test.UnconfinedTestDispatcher
import kotlinx.coroutines.test.runTest
import org.junit.Test

class TestDispatcherNotInjectedNegative {
    @Test
    fun works() = runTest(UnconfinedTestDispatcher()) {
        assert(true)
    }
}
