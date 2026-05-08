package test

import kotlinx.coroutines.test.runTest
import org.junit.Test

class RunTestWithThreadSleepPositive {
    @Test
    fun works() = runTest {
        Thread.sleep(100)
        assert(true)
    }
}
