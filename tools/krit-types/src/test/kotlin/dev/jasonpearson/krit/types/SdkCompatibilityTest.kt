package dev.jasonpearson.krit.types

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull
import kotlin.test.assertTrue

class SdkCompatibilityTest {

    private val jar = "/tmp/acme-rules.jar"

    @Test
    fun exactMatchProducesNoDiagnostic() {
        assertNull(SdkCompatibility.check(jar, "1.2.3", "1.2.3"))
    }

    @Test
    fun patchDriftIsSilentlyOk() {
        assertNull(SdkCompatibility.check(jar, "1.2.0", "1.2.7"))
        assertNull(SdkCompatibility.check(jar, "1.2.7", "1.2.0"))
    }

    @Test
    fun preReleaseAndBuildMetadataAreIgnoredForCompat() {
        assertNull(SdkCompatibility.check(jar, "1.2.3-rc1", "1.2.3"))
        assertNull(SdkCompatibility.check(jar, "1.2.3+sha.abc", "1.2.3"))
    }

    @Test
    fun minorDriftWarnsOnOneXButDoesNotBlock() {
        val d = SdkCompatibility.check(jar, "1.2.5", "1.3.0") ?: error("expected a diagnostic")
        assertEquals(PluginLoadDiagnostic.Level.WARN, d.level)
        assertEquals("1.2.5", d.ruleSdkVersion)
        assertEquals("1.3.0", d.daemonSdkVersion)
        assertTrue(d.message.contains("minor version differs"), d.message)
    }

    @Test
    fun majorDriftIsBlocking() {
        val d = SdkCompatibility.check(jar, "1.5.0", "2.0.0") ?: error("expected a diagnostic")
        assertEquals(PluginLoadDiagnostic.Level.ERROR, d.level)
        assertTrue(d.message.contains("major version mismatch"), d.message)
    }

    @Test
    fun zeroXMinorDriftIsBlocking() {
        val d = SdkCompatibility.check(jar, "0.2.0", "0.3.0") ?: error("expected a diagnostic")
        assertEquals(PluginLoadDiagnostic.Level.ERROR, d.level)
        assertTrue(d.message.contains("0.x"), d.message)
    }

    @Test
    fun zeroXPatchDriftIsOk() {
        assertNull(SdkCompatibility.check(jar, "0.2.0", "0.2.5"))
    }

    @Test
    fun missingManifestWarnsWithGuidance() {
        val d = SdkCompatibility.check(jar, "", "1.4.2") ?: error("expected a diagnostic")
        assertEquals(PluginLoadDiagnostic.Level.WARN, d.level)
        assertTrue(d.message.contains("missing"), d.message)
        assertTrue(d.message.contains("1.4.2"), d.message)
    }

    @Test
    fun unparseableVersionWarns() {
        val d = SdkCompatibility.check(jar, "not.a.semver", "1.2.3") ?: error("expected a diagnostic")
        assertEquals(PluginLoadDiagnostic.Level.WARN, d.level)
        assertTrue(d.message.contains("not.a.semver"), d.message)
    }

    @Test
    fun snapshotDaemonOrJarIsAlwaysCompatible() {
        // Local dev builds never produce a spurious diagnostic against a
        // -SNAPSHOT counterpart — that direction is the whole point of
        // composite-build dogfooding.
        assertNull(SdkCompatibility.check(jar, "0.0.0-SNAPSHOT", "1.2.3"))
        assertNull(SdkCompatibility.check(jar, "1.2.3", "0.0.0-SNAPSHOT"))
    }
}
