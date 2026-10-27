package dev.jasonpearson.krit.intellij

import java.io.File
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue

class KritConfigOverrideTest {
    private val base = File("/work/app").absolutePath

    @Test
    fun `blank setting means no override`() {
        assertNull(KritConfigOverride.resolve(null, base))
        assertNull(KritConfigOverride.resolve("   ", base))
        assertEquals(emptyList(), KritConfigOverride.cliArgs(null))
    }

    @Test
    fun `absolute paths are used as is, normalized`() {
        val abs = File("/etc/krit/../krit/strict.yml").absolutePath
        assertEquals(File("/etc/krit/strict.yml").absolutePath, KritConfigOverride.resolve(abs, base))
        assertEquals(File("/etc/krit/strict.yml").absolutePath, KritConfigOverride.resolve(" $abs ", null))
    }

    @Test
    fun `relative paths resolve against the project root`() {
        assertEquals(File(base, "config/krit.yml").path, KritConfigOverride.resolve("config/krit.yml", base))
        assertEquals(File(base, "krit.yml").path, KritConfigOverride.resolve("./config/../krit.yml", base))
        assertNull(KritConfigOverride.resolve("config/krit.yml", null))
    }

    @Test
    fun `an override is passed with --config and keeps the scan off the daemon`() {
        assertEquals(listOf("--config", "/x/krit.yml", "--no-daemon"), KritConfigOverride.cliArgs("/x/krit.yml"))
    }

    @Test
    fun `edits to the override file count as config changes`() {
        val override = KritConfigOverride.resolve("config/krit.yml", base)
        assertTrue(KritConfigOverride.isOverride(override, File(base, "config/krit.yml").path))
        assertFalse(KritConfigOverride.isOverride(override, File(base, "krit.yml").path))
        assertFalse(KritConfigOverride.isOverride(null, File(base, "config/krit.yml").path))
    }
}
