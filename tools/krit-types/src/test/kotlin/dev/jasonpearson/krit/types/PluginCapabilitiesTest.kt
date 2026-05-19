package dev.jasonpearson.krit.types

import dev.jasonpearson.krit.api.Capability
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

class PluginCapabilitiesTest {

    private val jar = "/tmp/acme-rules.jar"

    @Test
    fun everyEnumValueIsClassified() {
        // Every Capability must be either in SUPPORTED (krit-types
        // delivers the fact into RuleContext) or in FIR_ONLY
        // (deliberately not on this backend; a different backend hosts
        // it). A capability missing from both buckets is an accidental
        // omission — without this guard a typo at enum-add time would
        // pass silently and leak through `unsupported()` as a generic
        // "unknown capability" rejection.
        for (capability in Capability.values()) {
            val name = capability.name
            val supported = name in PluginCapabilities.SUPPORTED
            val firOnly = name in PluginCapabilities.FIR_ONLY
            assertTrue(
                supported xor firOnly,
                "$capability must be classified into exactly one of SUPPORTED " +
                    "or FIR_ONLY (supported=$supported, firOnly=$firOnly)",
            )
        }
    }

    @Test
    fun supportedCapabilitiesPassUnsupportedFilter() {
        // The eight non-FIR capabilities should all pass through the
        // krit-types gate cleanly.
        for (capability in Capability.values()) {
            if (capability.name in PluginCapabilities.FIR_ONLY) continue
            assertTrue(
                PluginCapabilities.unsupported(listOf(capability.name)).isEmpty(),
                "$capability is in SUPPORTED but unsupported() flagged it",
            )
        }
    }

    @Test
    fun firOnlyCapabilitiesAreRefusedByKaaBackend() {
        // Declaring NEEDS_FIR opts a rule into the FIR-backend-only
        // surface. The krit-types (KAA) backend must reject the rule at
        // load time so the rule never silently runs against a backend
        // that would throw `NotImplementedError` mid-analysis.
        val unsupported = PluginCapabilities.unsupported(listOf(Capability.NEEDS_FIR.name))
        assertEquals(listOf(Capability.NEEDS_FIR.name), unsupported)
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

    @Test
    fun firOnlyViolationProducesBackendSpecificDiagnostic() {
        // A rule declaring NEEDS_FIR needs to know which backend to
        // switch to, not the generic "unsupported capability" message.
        // The diagnostic must surface the FIR backend by name and the
        // --oracle-backend flag.
        val diagnostic = PluginCapabilities.buildLoadDiagnostic(
            jar = jar,
            ruleSdkVersion = "1.2.3",
            daemonSdkVersion = "1.2.3",
            violations = listOf(
                CapabilityViolation("acme.NeedsFirRule", listOf(Capability.NEEDS_FIR.name)),
            ),
        )
        assertEquals(PluginLoadDiagnostic.Level.ERROR, diagnostic.level)
        assertTrue(diagnostic.message.contains("--oracle-backend=fir"), diagnostic.message)
        assertTrue(diagnostic.message.contains("krit-fir"), diagnostic.message)
        assertTrue(diagnostic.message.contains("acme.NeedsFirRule"), diagnostic.message)
        assertTrue(diagnostic.message.contains(Capability.NEEDS_FIR.name), diagnostic.message)
        // The FIR-specific message replaces the generic tracking-issue
        // pointer because the user-actionable next step is a flag, not
        // an upstream issue.
        assertTrue(
            !diagnostic.message.contains(PluginCapabilities.TRACKING_ISSUE_URL),
            "FIR-only diagnostic should not point at the tracking issue: ${diagnostic.message}",
        )
    }

    @Test
    fun firOnlyTakesPrecedenceOverUnknownInDiagnostic() {
        // When the same jar declares both a FIR-only capability and an
        // unknown one, the FIR backend hint is the more actionable
        // signal — surface it. Both capability names still appear in
        // the unsupported list so the user can see the full picture.
        val diagnostic = PluginCapabilities.buildLoadDiagnostic(
            jar = jar,
            ruleSdkVersion = "1.2.3",
            daemonSdkVersion = "1.2.3",
            violations = listOf(
                CapabilityViolation(
                    "acme.MixedRule",
                    listOf(Capability.NEEDS_FIR.name, "NEEDS_TIME_TRAVEL"),
                ),
            ),
        )
        assertTrue(diagnostic.message.contains("--oracle-backend=fir"), diagnostic.message)
        assertTrue(diagnostic.message.contains(Capability.NEEDS_FIR.name), diagnostic.message)
        assertTrue(diagnostic.message.contains("NEEDS_TIME_TRAVEL"), diagnostic.message)
    }
}
