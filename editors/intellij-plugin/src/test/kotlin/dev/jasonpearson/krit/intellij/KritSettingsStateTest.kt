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
    fun `default snapshot uses the documented scan debounce`() {
        assertEquals(1_000, KritSettingsState.Snapshot().debounceMs)
    }

    @Test
    fun `snapshot persisted without debounceMs loads with the default`() {
        // Settings written by an older plugin version lack the field; the
        // serializer then leaves the constructor default in place. Building a
        // snapshot from only the pre-existing fields models that.
        val legacy = KritSettingsState.Snapshot(
            binaryPath = "/opt/krit/krit",
            fixLevel = "semantic",
            configPath = "/repo/krit.yml",
        )
        assertEquals(KritScanDebounce.DEFAULT_MS, legacy.debounceMs)
        assertEquals(1_000L, KritScanDebounce.effectiveMs(legacy.debounceMs))
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
            debounceMs = 250,
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
