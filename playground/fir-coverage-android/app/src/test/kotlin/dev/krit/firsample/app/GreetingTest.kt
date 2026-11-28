package dev.krit.firsample.app

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotEquals
import org.junit.Test

// A unit test that uses the app's R class. An application's unit tests have no
// R class of their own (AGP registers generate<Variant>UnitTestStubRFile only for
// a library): they compile against the tested variant's R jar, which the model
// records for them (#1724).
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
