package dev.krit.firsample.shared

import kotlin.test.Test
import kotlin.test.assertTrue

// commonTest is compiled with each target's test compilation (jvmTest here) and
// resolves commonMain's expect through that target's actual.
class PlatformTest {
    @Test
    fun namesThePlatform() {
        assertTrue(platformName().isNotBlank())
    }
}
