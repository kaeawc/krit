package dev.krit.firsample.app

import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.platform.app.InstrumentationRegistry
import org.junit.Assert.assertEquals
import org.junit.Test
import org.junit.runner.RunWith
import dev.krit.firsample.app.test.R as TestR

// An instrumented test that uses both the app's R and its own (the
// androidTest component's, from src/androidTest/res). FIR compiles it against
// the debugAndroidTest R jar (processDebugAndroidTestResources), so without
// that jar this file is gated (#1724).
@RunWith(AndroidJUnit4::class)
class StringsInstrumentedTest {
    @Test
    fun readsAppAndTestStrings() {
        val instrumentation = InstrumentationRegistry.getInstrumentation()
        assertEquals("FIR coverage sample", instrumentation.targetContext.getString(R.string.app_name))
        assertEquals("Fixture user", instrumentation.context.getString(TestR.string.fixture_user))
    }
}
