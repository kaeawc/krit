package dev.krit.firsample.app

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotEquals
import org.junit.Test

// A unit test that uses the app's R class. Its model entry carries the
// debugUnitTest R jar (generateDebugUnitTestStubRFile); the app's own R jar
// resolves these references too (#1724).
class GreetingTest {
    @Test
    fun greetsFirstVisit() {
        assertEquals("Welcome, Ada", greeting("Ada", 1))
    }

    @Test
    fun resourceIdsAreDistinct() {
        assertNotEquals(R.string.app_name, R.string.refresh)
    }
}
