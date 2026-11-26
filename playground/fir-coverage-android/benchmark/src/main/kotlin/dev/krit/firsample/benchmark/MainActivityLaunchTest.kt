package dev.krit.firsample.benchmark

import androidx.test.core.app.ActivityScenario
import androidx.test.ext.junit.runners.AndroidJUnit4
import dev.krit.firsample.app.MainActivity
import org.junit.Assert.assertEquals
import org.junit.Test
import org.junit.runner.RunWith

// A com.android.test module compiles against its target app (:app), so
// MainActivity resolves only through the target project's classes.
@RunWith(AndroidJUnit4::class)
class MainActivityLaunchTest {
    @Test
    fun launchesMainActivity() {
        ActivityScenario.launch(MainActivity::class.java).use { scenario ->
            scenario.onActivity { activity -> assertEquals(false, activity.isFinishing) }
        }
    }
}
