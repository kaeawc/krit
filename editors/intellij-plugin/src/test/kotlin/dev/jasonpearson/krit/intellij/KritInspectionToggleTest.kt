package dev.jasonpearson.krit.intellij

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

class KritInspectionToggleTest {
    @Test
    fun `covers every Krit inspection registered in plugin xml`() {
        assertEquals(listOf("Krit", "KritJava", "KritXml"), KritInspectionToggle.SHORT_NAMES)
    }

    @Test
    fun `scans pause only when all three inspections are off`() {
        assertFalse(KritInspectionToggle.anyEnabled { false })
        assertTrue(KritInspectionToggle.anyEnabled { true })
        for (on in KritInspectionToggle.SHORT_NAMES) {
            assertTrue(KritInspectionToggle.anyEnabled { it == on }, "only $on enabled should keep scanning")
        }
    }
}
