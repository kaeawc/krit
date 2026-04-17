package test

import org.junit.Test

class SharedMutableStatePositive {
    companion object {
        var counter = 0
    }

    @Test
    fun a() {
        counter++
        assert(counter > 0)
    }
}
