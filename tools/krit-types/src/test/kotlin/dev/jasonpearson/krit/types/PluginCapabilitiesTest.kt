package dev.jasonpearson.krit.types

import dev.jasonpearson.krit.api.Capability
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

class PluginCapabilitiesTest {

    private val jar = "/tmp/acme-rules.jar"

    @Test
    fun resolverAndParsedFilesAreSupported() {
        assertTrue(PluginCapabilities.unsupported(listOf(Capability.NEEDS_RESOLVER.name)).isEmpty())
        assertTrue(PluginCapabilities.unsupported(listOf(Capability.NEEDS_PARSED_FILES.name)).isEmpty())
    }

    @Suppress("DEPRECATION")
    @Test
    fun unsupportedListsEveryDeprecatedCapability() {
        val unsupported = PluginCapabilities.unsupported(
            listOf(
                Capability.NEEDS_RESOLVER.name,
                Capability.NEEDS_CROSS_FILE.name,
                Capability.NEEDS_MODULE_INDEX.name,
                Capability.NEEDS_PARSED_FILES.name,
                Capability.NEEDS_MANIFEST.name,
                Capability.NEEDS_RESOURCES.name,
                Capability.NEEDS_GRADLE.name,
            ),
        )
        assertEquals(
            listOf(
                Capability.NEEDS_CROSS_FILE.name,
                Capability.NEEDS_MODULE_INDEX.name,
                Capability.NEEDS_MANIFEST.name,
                Capability.NEEDS_RESOURCES.name,
                Capability.NEEDS_GRADLE.name,
            ),
            unsupported,
        )
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

    @Suppress("DEPRECATION")
    @Test
    fun loadDiagnosticIsErrorLevelWithRuleIdAndCapability() {
        val diagnostic = PluginCapabilities.buildLoadDiagnostic(
            jar = jar,
            ruleSdkVersion = "1.2.3",
            daemonSdkVersion = "1.2.3",
            violations = listOf(
                CapabilityViolation("acme.NoTodo", listOf(Capability.NEEDS_GRADLE.name)),
                CapabilityViolation(
                    "acme.NoFixme",
                    listOf(Capability.NEEDS_MANIFEST.name, Capability.NEEDS_RESOURCES.name),
                ),
            ),
        )
        assertEquals(PluginLoadDiagnostic.Level.ERROR, diagnostic.level)
        assertEquals(jar, diagnostic.jar)
        assertEquals("1.2.3", diagnostic.ruleSdkVersion)
        assertEquals("1.2.3", diagnostic.daemonSdkVersion)
        assertTrue(diagnostic.message.contains("acme.NoTodo"), diagnostic.message)
        assertTrue(diagnostic.message.contains("acme.NoFixme"), diagnostic.message)
        assertTrue(diagnostic.message.contains(Capability.NEEDS_GRADLE.name), diagnostic.message)
        assertTrue(diagnostic.message.contains(Capability.NEEDS_MANIFEST.name), diagnostic.message)
        assertTrue(diagnostic.message.contains(Capability.NEEDS_RESOURCES.name), diagnostic.message)
        assertTrue(
            diagnostic.message.contains(PluginCapabilities.TRACKING_ISSUE_URL),
            diagnostic.message,
        )
    }

    @Suppress("DEPRECATION")
    @Test
    fun loadDiagnosticOrdersRulesAlphabeticallyForDeterminism() {
        val diagnostic = PluginCapabilities.buildLoadDiagnostic(
            jar = jar,
            ruleSdkVersion = "1.2.3",
            daemonSdkVersion = "1.2.3",
            violations = listOf(
                CapabilityViolation("z.RuleZ", listOf(Capability.NEEDS_GRADLE.name)),
                CapabilityViolation("a.RuleA", listOf(Capability.NEEDS_MANIFEST.name)),
            ),
        )
        val idxA = diagnostic.message.indexOf("a.RuleA")
        val idxZ = diagnostic.message.indexOf("z.RuleZ")
        assertTrue(idxA in 0 until idxZ, "rules out of order: ${diagnostic.message}")
    }
}
