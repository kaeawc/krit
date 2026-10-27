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

    @Test
    fun `a config that failed to load is reported from krit's warning`() {
        val stderr = "info: something\n" +
            "warning: config: loading config /x/krit.yml: open /x/krit.yml: no such file or directory\n" +
            "warning: unrelated\n"
        assertEquals(
            "config: loading config /x/krit.yml: open /x/krit.yml: no such file or directory",
            KritConfigOverride.loadFailure(stderr),
        )
        assertNull(KritConfigOverride.loadFailure("warning: unknown --max-cost value \"x\"\n"))
        assertNull(KritConfigOverride.loadFailure(""))
    }

    @Test
    fun `the preflight before a fix rejects an override krit can't load or validate`() {
        // krit --validate-config exits 0 for a missing or unparsable file and
        // only warns.
        val missing = "warning: config: loading config /x/krit.yml: open /x/krit.yml: no such file or directory\n" +
            "info: Config validation passed.\n"
        assertEquals(
            "config: loading config /x/krit.yml: open /x/krit.yml: no such file or directory",
            KritConfigOverride.preflightFailure(0, missing),
        )
        // Validation errors exit 1.
        val invalid = "[error] nosuchset: unknown ruleset 'nosuchset'\ninfo: Config validation failed with 1 issue(s).\n"
        assertEquals("[error] nosuchset: unknown ruleset 'nosuchset'", KritConfigOverride.preflightFailure(1, invalid))
        assertEquals("krit config validation failed (exit code 1)", KritConfigOverride.preflightFailure(1, ""))
        // Warnings that aren't about loading the file don't block the fix.
        assertNull(KritConfigOverride.preflightFailure(0, "[warn] style: something\ninfo: Config validation passed with 1 warning(s).\n"))
        assertNull(KritConfigOverride.preflightFailure(0, "info: Config validation passed.\n"))
    }
}
