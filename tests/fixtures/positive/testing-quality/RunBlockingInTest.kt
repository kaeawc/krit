package test

import kotlinx.coroutines.runBlocking
import org.junit.Test

class RunBlockingInTestPositive {
    @Test
    fun works() = runBlocking {
        assert(true)
    }
}
