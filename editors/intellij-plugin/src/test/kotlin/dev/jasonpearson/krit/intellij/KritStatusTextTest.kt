package dev.jasonpearson.krit.intellij

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

class KritStatusTextTest {
    @Test
    fun `initializing state renders starting label`() {
        assertEquals("Krit: starting…", KritStatusText.render(KritState.Initializing))
    }

    @Test
    fun `scanning state renders scanning label`() {
        assertEquals("Krit: scanning…", KritStatusText.render(KritState.Scanning))
    }

    @Test
    fun `idle state renders finding count`() {
        assertEquals("Krit: 0", KritStatusText.render(KritState.Idle(0)))
        assertEquals("Krit: 42", KritStatusText.render(KritState.Idle(42)))
    }

    @Test
    fun `missing binary state renders not-found label`() {
        assertEquals("Krit: binary not found", KritStatusText.render(KritState.MissingBinary))
    }

    @Test
    fun `error state renders generic error label`() {
        // The detail goes in the tooltip; the status bar text stays short.
        assertEquals("Krit: error", KritStatusText.render(KritState.Error("boom")))
    }

    @Test
    fun `tooltip pluralizes findings correctly`() {
        assertEquals("Krit: no findings; click to configure", KritStatusText.tooltip(KritState.Idle(0)))
        assertEquals("Krit: 1 finding; click to configure", KritStatusText.tooltip(KritState.Idle(1)))
        assertEquals("Krit: 5 findings; click to configure", KritStatusText.tooltip(KritState.Idle(5)))
    }

    @Test
    fun `tooltip for error state includes the error message`() {
        assertTrue("scan timed out" in KritStatusText.tooltip(KritState.Error("scan timed out")))
    }

    @Test
    fun `tooltip for missing binary mentions both override mechanisms`() {
        val msg = KritStatusText.tooltip(KritState.MissingBinary)
        assertTrue("KRIT_BINARY" in msg)
        assertTrue("krit.binary" in msg)
    }
}
