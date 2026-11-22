// Smoke: TestNG lifecycle annotations.
package stubs

import org.testng.annotations.AfterClass
import org.testng.annotations.AfterMethod
import org.testng.annotations.BeforeClass
import org.testng.annotations.BeforeMethod
import org.testng.annotations.Test

class TestNGSmoke {
    @Test
    fun test() {}

    @BeforeClass
    fun beforeClass() {}

    @AfterClass
    fun afterClass() {}

    @BeforeMethod
    fun beforeMethod() {}

    @AfterMethod
    fun afterMethod() {}
}
