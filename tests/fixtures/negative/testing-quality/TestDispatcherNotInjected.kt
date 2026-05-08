package test

import kotlinx.coroutines.test.UnconfinedTestDispatcher
import kotlinx.coroutines.test.runTest
import org.junit.Test

class TestDispatcherNotInjectedNegative {
    @Test
    fun works() = runTest(UnconfinedTestDispatcher()) {
        assert(true)
    }

    @Test
    fun mainImmediateIsSubjectUnderTest() = runTest {
        val dispatcher = kotlinx.coroutines.Dispatchers.Main.immediate
        assert(dispatcher != null)
    }
}
