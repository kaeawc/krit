package dev.jasonpearson.krit.intellij

import kotlin.test.Test
import kotlin.test.assertFalse
import kotlin.test.assertTrue

class KritFileFilterTest {
    @Test
    fun `accepts kotlin source extensions`() {
        assertTrue(KritFileFilter.isSupported("Foo.kt"))
        assertTrue(KritFileFilter.isSupported("build.kts"))
        assertTrue(KritFileFilter.isSupported("settings.gradle.kts"))
    }

    @Test
    fun `accepts java source`() {
        assertTrue(KritFileFilter.isSupported("Foo.java"))
    }

    @Test
    fun `accepts xml manifest and resources`() {
        assertTrue(KritFileFilter.isSupported("AndroidManifest.xml"))
        assertTrue(KritFileFilter.isSupported("strings.xml"))
    }

    @Test
    fun `accepts groovy gradle files`() {
        assertTrue(KritFileFilter.isSupported("build.gradle"))
    }

    @Test
    fun `extension matching is case insensitive`() {
        // Some users have mixed-case filenames; matcher should not care.
        assertTrue(KritFileFilter.isSupported("Foo.KT"))
        assertTrue(KritFileFilter.isSupported("ANDROIDMANIFEST.XML"))
    }

    @Test
    fun `rejects unrelated extensions`() {
        assertFalse(KritFileFilter.isSupported("README.md"))
        assertFalse(KritFileFilter.isSupported("config.yaml"))
        assertFalse(KritFileFilter.isSupported("Foo.kts.bak"))
        assertFalse(KritFileFilter.isSupported("nothing"))
    }
}
