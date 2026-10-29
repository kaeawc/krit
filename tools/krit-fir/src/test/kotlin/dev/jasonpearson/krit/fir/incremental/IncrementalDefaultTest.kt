package dev.jasonpearson.krit.fir.incremental

import org.junit.jupiter.api.Test
import kotlin.test.assertFalse
import kotlin.test.assertTrue

/** #740: incremental checking is on unless `KRIT_FIR_INCREMENTAL` turns it off explicitly. */
class IncrementalDefaultTest {
    @Test fun onWhenUnsetOrAnythingButAnExplicitOff() {
        for (value in listOf(null, "", "1", "true", "yes")) assertTrue(incrementalEnabled(value), "value=$value")
    }

    @Test fun offForZeroFalseOrOff() {
        for (value in listOf("0", "false", "off", " OFF ", "False")) assertFalse(incrementalEnabled(value), "value=$value")
    }
}
