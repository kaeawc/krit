package test

import kotlinx.coroutines.test.runTest
import org.junit.Test

class RunBlockingInTestNegative {
    @Test
    fun works() = runTest {
        assert(true)
    }
}
