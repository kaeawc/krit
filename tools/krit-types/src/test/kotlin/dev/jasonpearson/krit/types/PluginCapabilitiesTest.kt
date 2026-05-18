package dev.jasonpearson.krit.types

import dev.jasonpearson.krit.api.Capability
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

class PluginCapabilitiesTest {

    private val jar = "/tmp/acme-rules.jar"

    @Test
    fun everyEnumValueIsSupported() {
        // Issue #357 promotes the final four `@Deprecated` capabilities
        // (MANIFEST, RESOURCES, MODULE_INDEX, CROSS_FILE) to supported.
        // A Capability enum value must be either in SUPPORTED or
        // removed from the enum entirely — there is no third
        // "advisory" state.
        for (capability in Capability.values()) {
            assertTrue(
                PluginCapabilities.unsupported(listOf(capability.name)).isEmpty(),
                "$capability should be supported but unsupported() returned a non-empty list",
            )
        }
    }

    @Test
    fun emptyNeedsListIsAlwaysSupported() {
        assertTrue(PluginCapabilities.unsupported(emptyList()).isEmpty())
    }

    @Test
    fun unknownEnumNameIsRejectedRatherThanSilentlyAccepted() {
        // Defensive: if a rule jar carries a capability the daemon
        // doesn't recognize at all (e.g. ahead-of-daemon rule SDK),
        // treat it as unsupported. The SDK-compat gate catches the
        // version skew, but if the user has --no-strict-sdk or similar
        // we still don't want to silently honor unknown facts.
        val unsupported = PluginCapabilities.unsupported(listOf("NEEDS_TIME_TRAVEL"))
        assertEquals(listOf("NEEDS_TIME_TRAVEL"), unsupported)
    }

    @Test
    fun loadDiagnosticIsErrorLevelWithRuleIdAndCapability() {
        val diagnostic = PluginCapabilities.buildLoadDiagnostic(
            jar = jar,
            ruleSdkVersion = "1.2.3",
            daemonSdkVersion = "1.2.3",
            violations = listOf(
                CapabilityViolation("acme.NoTodo", listOf("NEEDS_TIME_TRAVEL")),
                CapabilityViolation(
                    "acme.NoFixme",
                    listOf("NEEDS_QUANTUM_TUNNEL", "NEEDS_WORMHOLE"),
                ),
            ),
        )
        assertEquals(PluginLoadDiagnostic.Level.ERROR, diagnostic.level)
        assertEquals(jar, diagnostic.jar)
        assertEquals("1.2.3", diagnostic.ruleSdkVersion)
        assertEquals("1.2.3", diagnostic.daemonSdkVersion)
        assertTrue(diagnostic.message.contains("acme.NoTodo"), diagnostic.message)
        assertTrue(diagnostic.message.contains("acme.NoFixme"), diagnostic.message)
        assertTrue(diagnostic.message.contains("NEEDS_TIME_TRAVEL"), diagnostic.message)
        assertTrue(diagnostic.message.contains("NEEDS_QUANTUM_TUNNEL"), diagnostic.message)
        assertTrue(diagnostic.message.contains("NEEDS_WORMHOLE"), diagnostic.message)
        assertTrue(
            diagnostic.message.contains(PluginCapabilities.TRACKING_ISSUE_URL),
            diagnostic.message,
        )
    }

    @Test
    fun loadDiagnosticOrdersRulesAlphabeticallyForDeterminism() {
        val diagnostic = PluginCapabilities.buildLoadDiagnostic(
            jar = jar,
            ruleSdkVersion = "1.2.3",
            daemonSdkVersion = "1.2.3",
            violations = listOf(
                CapabilityViolation("z.RuleZ", listOf("NEEDS_TIME_TRAVEL")),
                CapabilityViolation("a.RuleA", listOf("NEEDS_QUANTUM_TUNNEL")),
            ),
        )
        val idxA = diagnostic.message.indexOf("a.RuleA")
        val idxZ = diagnostic.message.indexOf("z.RuleZ")
        assertTrue(idxA in 0 until idxZ, "rules out of order: ${diagnostic.message}")
    }
}
