package test

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.runBlocking
import kotlinx.coroutines.test.runTest
import kotlinx.coroutines.withContext
import org.junit.Test

class RunBlockingInTestNegative {
    @Test
    fun works() = runTest {
        assert(true)
    }

    @Test
    fun preservesThreadIdentityAcrossDispatcherSwitch() {
        runBlocking {
            val original = Thread.currentThread()
            val switched = withContext(Dispatchers.Default) {
                Thread.currentThread()
            }

            assert(original !== switched)
        }
    }
}
