package test

import org.junit.Test

abstract class BaseTest {
    fun setup() {}
}

abstract class MiddleTest : BaseTest() {
    fun middle() {}
}

class DeepTest : MiddleTest() {
    @Test
    fun works() {
        assert(true)
    }
}
