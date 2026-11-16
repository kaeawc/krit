// Smoke: a JUnit 3 test case calling the inherited and qualified static assertions.
package stubs

import junit.framework.Assert
import junit.framework.TestCase

class JUnit3Smoke : TestCase() {
    override fun setUp() {
        super.setUp()
    }

    fun testChecks() {
        assertTrue(true)
        assertFalse(false)
        Assert.assertTrue("message", true)
        TestCase.assertFalse("message", false)
    }

    override fun tearDown() {
        super.tearDown()
    }
}
