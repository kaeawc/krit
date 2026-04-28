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

class FakeTimer {
    suspend fun delay(millis: Long) {}
}

class RunTestWithDelayProjectReceiver {
    private val timer = FakeTimer()

    @Test
    fun recordsDelay() = runTest {
        timer.delay(1000)
        assert(true)
    }
}

suspend fun delay(millis: Long) {}

class RunTestWithDelayLocalLookalike {
    @Test
    fun recordsDelay() = runTest {
        delay(1000)
        assert(true)
    }
}
