package test

import io.mockk.spyk
import org.junit.Test

data class User(val name: String)

class SpyOnDataClassPositive {
    @Test
    fun works() {
        val user = spyk(User("alice"))
        assert(user.name == "alice")
    }
}
