// Smoke: runTest with TestScope helpers and Dispatchers.setMain.
package stubs

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlinx.coroutines.test.StandardTestDispatcher
import kotlinx.coroutines.test.TestDispatcher
import kotlinx.coroutines.test.TestResult
import kotlinx.coroutines.test.TestScope
import kotlinx.coroutines.test.UnconfinedTestDispatcher
import kotlinx.coroutines.test.advanceTimeBy
import kotlinx.coroutines.test.advanceUntilIdle
import kotlinx.coroutines.test.currentTime
import kotlinx.coroutines.test.resetMain
import kotlinx.coroutines.test.runCurrent
import kotlinx.coroutines.test.runTest
import kotlinx.coroutines.test.setMain
import org.junit.Test

@OptIn(ExperimentalCoroutinesApi::class)
class CoroutineTestSmoke {
    private val dispatcher: TestDispatcher = StandardTestDispatcher()

    @Test
    fun virtualTime(): TestResult = runTest {
        Dispatchers.setMain(dispatcher)
        launch { delay(1_000) }
        advanceTimeBy(500)
        runCurrent()
        advanceUntilIdle()
        <!PrintlnInProduction!>println<!>(currentTime + testScheduler.currentTime)
        backgroundScope.launch {}
        Dispatchers.resetMain()
    }

    @Test
    fun scoped() = TestScope(UnconfinedTestDispatcher()).runTest {
        val scope: TestScope = this
        <!PrintlnInProduction!>println<!>(scope)
    }
}
