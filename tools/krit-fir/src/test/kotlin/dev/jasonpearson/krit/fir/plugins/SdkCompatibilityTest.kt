package dev.jasonpearson.krit.fir.plugins

import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertNotNull
import kotlin.test.assertNull
import kotlin.test.assertTrue

class SdkCompatibilityTest {

    @Test
    fun blankSdkVersionEmitsManifestAttributeWarn() {
        // A jar shipped without `Krit-SDK-Version` predates the
        // compat-gate work. WARN keeps load behavior compatible with
        // krit-types so the same JAR doesn't get flagged differently
        // across backends.
        val d = SdkCompatibility.check(jar = "/p/rules.jar", ruleSdkVersion = "")
        assertNotNull(d)
        assertEquals(PluginLoadDiagnostic.Level.WARN, d.level)
        assertTrue("Krit-SDK-Version manifest attribute" in d.message, d.message)
    }

    @Test
    fun matchingMajorAndMinorIsCompatible() {
        val d = SdkCompatibility.check(
            jar = "/p/rules.jar",
            ruleSdkVersion = "1.2.0",
            daemonSdkVersion = "1.2.7",
        )
        assertNull(d)
    }

    @Test
    fun majorMismatchIsBreakingError() {
        val d = SdkCompatibility.check(
            jar = "/p/rules.jar",
            ruleSdkVersion = "1.0.0",
            daemonSdkVersion = "2.0.0",
        )
        assertNotNull(d)
        assertEquals(PluginLoadDiagnostic.Level.ERROR, d.level)
        assertTrue("major version mismatch" in d.message, d.message)
    }

    @Test
    fun zeroXMinorMismatchIsBreaking() {
        // Under semver, 0.x is "anything goes". The compat gate
        // mirrors that by treating 0.x minor differences as breaking
        // ERROR rather than a soft WARN. Matches krit-types verbatim.
        val d = SdkCompatibility.check(
            jar = "/p/rules.jar",
            ruleSdkVersion = "0.3.0",
            daemonSdkVersion = "0.4.1",
        )
        assertNotNull(d)
        assertEquals(PluginLoadDiagnostic.Level.ERROR, d.level)
        assertTrue("0.x minor version mismatch" in d.message, d.message)
    }

    @Test
    fun postOneMinorMismatchIsSoftWarn() {
        val d = SdkCompatibility.check(
            jar = "/p/rules.jar",
            ruleSdkVersion = "1.4.0",
            daemonSdkVersion = "1.6.2",
        )
        assertNotNull(d)
        assertEquals(PluginLoadDiagnostic.Level.WARN, d.level)
        assertTrue("minor version differs" in d.message, d.message)
    }

    @Test
    fun devSnapshotSkipsTheCompatGate() {
        // 0.0.0-SNAPSHOT is the dev-time version published by the
        // local build. The compat gate must skip it so contributors
        // running against a SNAPSHOT daemon aren't blocked.
        assertNull(SdkCompatibility.check("/p/rules.jar", "0.0.0-SNAPSHOT", "1.2.3"))
        assertNull(SdkCompatibility.check("/p/rules.jar", "1.2.3", "0.0.0-SNAPSHOT"))
    }

    @Test
    fun unparseableRuleSdkVersionEmitsWarn() {
        val d = SdkCompatibility.check(
            jar = "/p/rules.jar",
            ruleSdkVersion = "garbage",
            daemonSdkVersion = "1.2.3",
        )
        assertNotNull(d)
        assertEquals(PluginLoadDiagnostic.Level.WARN, d.level)
        assertTrue("could not parse rule Krit-SDK-Version" in d.message, d.message)
    }

    @Test
    fun semverWithPreReleaseAndBuildIsAccepted() {
        // Pre-release and build metadata are stripped before
        // comparison so an RC build of the rule API is treated as
        // its base version. Mirrors krit-types' parser regex.
        assertEquals(Semver(1, 2, 3), Semver.parse("1.2.3-rc1"))
        assertEquals(Semver(1, 2, 3), Semver.parse("1.2.3+build.7"))
        assertEquals(null, Semver.parse("not.a.version"))
    }
}
