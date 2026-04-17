package test

import org.junit.Test

abstract class BaseTestNeg {
    fun setup() {}
}

class ShallowTest : BaseTestNeg() {
    @Test
    fun works() {
        assert(true)
    }
}
