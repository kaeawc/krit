package test

import kotlinx.coroutines.test.runTest
import org.junit.Test

class RunTestWithThreadSleepNegative {
    @Test
    fun works() = runTest {
        advanceTimeBy(100)
        assert(true)
    }
}
