package test

import kotlinx.coroutines.delay
import kotlinx.coroutines.test.runTest
import org.junit.Test

class RunTestWithDelayPositive {
    @Test
    fun works() = runTest {
        delay(1000)
        assert(true)
    }
}
