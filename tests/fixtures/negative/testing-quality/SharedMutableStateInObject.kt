package test

import org.junit.Before
import org.junit.Test

class SharedMutableStateNegative {
    private var counter = 0

    @Before
    fun setup() {
        counter = 0
    }

    @Test
    fun a() {
        counter++
        assert(counter > 0)
    }
}
