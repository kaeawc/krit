// Smoke: TestNG lifecycle annotations.
package stubs

import org.testng.annotations.AfterClass
import org.testng.annotations.AfterMethod
import org.testng.annotations.BeforeClass
import org.testng.annotations.BeforeMethod

class TestNGSmoke {
    @BeforeClass
    fun beforeClass() {}

    @AfterClass
    fun afterClass() {}

    @BeforeMethod
    fun beforeMethod() {}

    @AfterMethod
    fun afterMethod() {}
}
