package dev.jasonpearson.krit.intellij

import kotlin.test.Test
import kotlin.test.assertEquals

class KritSettingsStateTest {
    @Test
    fun `default snapshot uses documented fix level`() {
        val s = KritSettingsState.Snapshot()
        assertEquals("idiomatic", s.fixLevel)
        assertEquals("", s.binaryPath)
        assertEquals("", s.configPath)
    }

    @Test
    fun `update replaces the snapshot in place`() {
        // The Configurable's apply() round-trips through update(), so this
        // pins the contract that a subsequent state read returns the new
        // value (no copy semantics that would silently lose edits).
        val state = KritSettingsState()
        val next = KritSettingsState.Snapshot(
            binaryPath = "/opt/krit/krit",
            fixLevel = "cosmetic",
            configPath = "/repo/krit.yml",
        )
        state.update(next)
        assertEquals(next, state.state)
    }

    @Test
    fun `loadState replaces the snapshot — PersistentStateComponent contract`() {
        // IntelliJ calls loadState during deserialisation; update() is the
        // app-side path. They must agree on the resulting state.
        val state = KritSettingsState()
        val next = KritSettingsState.Snapshot(binaryPath = "/x/krit")
        state.loadState(next)
        assertEquals(next, state.state)
    }
}
