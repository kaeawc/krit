// Smoke: JUnit 4 test class with lifecycle hooks, rules, and Assert statics.
package stubs

import org.junit.After
import org.junit.AfterClass
import org.junit.Assert
import org.junit.Assert.assertEquals
import org.junit.Before
import org.junit.BeforeClass
import org.junit.Ignore
import org.junit.Rule
import org.junit.Test

class JUnit4Smoke {
    @get:Rule
    val rule: Any = Any()

    @Before
    fun setUp() {}

    @After
    fun tearDown() {}

    @Test
    fun adds() {
        assertEquals(2, 1 + 1)
        assertEquals("message", 2L, 2L)
        Assert.assertTrue(true)
        Assert.assertTrue("message", true)
        Assert.assertFalse(false)
        Assert.assertNull(null)
        Assert.assertNotNull(Any())
        Assert.assertEquals(1.0, 1.0, 0.001)
        Assert.assertThrows(IllegalStateException::class.java) { error("boom") }
    }

    @Test(expected = IllegalStateException::class)
    fun throws() {
        error("boom")
    }

    @Test(timeout = 1_000)
    fun fast() {}

    @Ignore("flaky")
    @Test
    fun ignored() {
        Assert.fail("not run")
    }

    companion object {
        @BeforeClass
        @JvmStatic
        fun beforeAll() {}

        @AfterClass
        @JvmStatic
        fun afterAll() {}
    }
}
