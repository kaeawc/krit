package coroutines

import kotlinx.coroutines.launch
import kotlinx.coroutines.test.runTest
import org.junit.Test

class MyTest {
    @Test
    fun testSomething() = runTest {
        launch {
            println("doing work")
        }
    }
}
