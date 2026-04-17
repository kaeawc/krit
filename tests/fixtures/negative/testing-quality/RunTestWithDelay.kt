package test

import kotlinx.coroutines.test.runTest
import org.junit.Test

class RunTestWithDelayNegative {
    @Test
    fun works() = runTest {
        advanceTimeBy(1000)
        assert(true)
    }
}
