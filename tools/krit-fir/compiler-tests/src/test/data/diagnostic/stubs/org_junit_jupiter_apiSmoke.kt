// Smoke: JUnit 5 lifecycle annotations, Assertions statics, and reified assertThrows.
package stubs

import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.Assertions
import org.junit.jupiter.api.Assertions.assertEquals
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Disabled
import org.junit.jupiter.api.DisplayName
import org.junit.jupiter.api.Nested
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.assertThrows

@DisplayName("Jupiter smoke")
class JupiterSmoke {
    @BeforeEach
    fun setUp() {}

    @AfterEach
    fun tearDown() {}

    @Test
    fun adds() {
        assertEquals(2, 1 + 1)
        assertEquals(2, 1 + 1, "message")
        Assertions.assertTrue(true)
        Assertions.assertNotNull(Any(), "message")
        Assertions.assertThrows(IllegalStateException::class.java) { error("boom") }
        val thrown: IllegalStateException = assertThrows<IllegalStateException> { error("boom") }
        <!PrintlnInProduction!>println<!>(thrown)
    }

    @Disabled("flaky")
    @Test
    fun disabled() {}

    @Nested
    inner class Inner {
        @Test
        fun nested() {}
    }
}
