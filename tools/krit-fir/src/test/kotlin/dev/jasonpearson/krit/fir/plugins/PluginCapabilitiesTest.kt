package dev.jasonpearson.krit.fir.plugins

import dev.jasonpearson.krit.api.Capability
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

class PluginCapabilitiesTest {

    @Test
    fun needsFirIsSupportedOnTheFirBackend() {
        // The whole point of the krit-fir backend: rules can opt
        // into FIR-only facts via NEEDS_FIR. krit-types refuses
        // them at load time; the FIR daemon accepts them.
        assertEquals(emptyList(), PluginCapabilities.unsupported(listOf(Capability.NEEDS_FIR.name)))
    }

    @Test
    fun coreCapabilitiesAreSupported() {
        val needs = listOf(
            Capability.NEEDS_RESOLVER,
            Capability.NEEDS_PARSED_FILES,
            Capability.NEEDS_GRADLE,
            Capability.NEEDS_MANIFEST,
            Capability.NEEDS_RESOURCES,
            Capability.NEEDS_MODULE_INDEX,
            Capability.NEEDS_CROSS_FILE,
        ).map { it.name }
        assertEquals(emptyList(), PluginCapabilities.unsupported(needs))
    }

    @Test
    fun unknownCapabilityNameIsReportedAsUnsupported() {
        // Test future-proofing: if a new capability ships in
        // krit-rule-api before the backend catches up, the loader
        // must reject the rule jar rather than silently dropping
        // the declaration.
        val unsupported = PluginCapabilities.unsupported(
            listOf("NEEDS_FIR", "NEEDS_TIME_TRAVEL"),
        )
        assertEquals(listOf("NEEDS_TIME_TRAVEL"), unsupported)
    }

    @Test
    fun buildLoadDiagnosticSortsViolationsByRuleId() {
        val diagnostic = PluginCapabilities.buildLoadDiagnostic(
            jar = "/p/rules.jar",
            ruleSdkVersion = "1.2.3",
            daemonSdkVersion = "1.2.3",
            violations = listOf(
                CapabilityViolation("z.Rule", listOf("NEEDS_X")),
                CapabilityViolation("a.Rule", listOf("NEEDS_Y", "NEEDS_Z")),
            ),
        )
        assertEquals(PluginLoadDiagnostic.Level.ERROR, diagnostic.level)
        // Sorted insertion: a.Rule renders before z.Rule.
        val aIdx = diagnostic.message.indexOf("a.Rule")
        val zIdx = diagnostic.message.indexOf("z.Rule")
        assertTrue(aIdx in 0 until zIdx, "expected a.Rule before z.Rule: ${diagnostic.message}")
    }
}
