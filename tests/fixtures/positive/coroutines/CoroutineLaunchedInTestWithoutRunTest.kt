package coroutines

import kotlinx.coroutines.launch
import org.junit.Test

class MyTest {
    @Test
    fun testSomething() {
        launch {
            println("doing work")
        }
    }
}
