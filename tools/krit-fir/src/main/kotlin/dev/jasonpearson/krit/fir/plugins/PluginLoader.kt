package dev.jasonpearson.krit.fir.plugins

import dev.jasonpearson.krit.api.Capability
import dev.jasonpearson.krit.api.KritRule
import dev.jasonpearson.krit.api.KritRuleInfo
import dev.jasonpearson.krit.api.RuleApiVersion
import java.io.File
import java.net.URLClassLoader
import java.util.ServiceLoader
import java.util.jar.JarFile

/**
 * Plugin-rule loader for the krit-fir backend. Mirrors the loader in
 * krit-types' `PluginRules.kt` so a JAR built against the same
 * `krit-rule-api` works against either backend.
 *
 * Differences from the krit-types loader are intentional:
 *
 * - The FIR backend SUPPORTS `NEEDS_FIR` capability. krit-types treats
 *   it as a load-time error because the KAA-backed daemon cannot
 *   deliver FIR-only facts.
 * - The actual rule-execution surface (`Resolver`, payload contexts)
 *   lives in later PRs in this series — this PR ships the loader,
 *   SDK-compat gate, capability gate, and `listPlugins` RPC only.
 */
internal data class PluginRuleDescriptor(
    val ruleId: String,
    val category: String,
    val severity: String,
    val maturity: String,
    val languages: List<String>,
    val needs: List<String>,
    val sdkVersion: String,
)

internal data class LoadedPluginRule(
    val rule: KritRule,
    val descriptor: PluginRuleDescriptor,
)

/**
 * Load-time diagnostic for one rule jar. SDK-compat warnings and
 * capability violations both surface as entries here so the
 * `listPlugins` RPC can attribute them back to the originating jar.
 */
internal data class PluginLoadDiagnostic(
    val jar: String,
    val level: Level,
    val ruleSdkVersion: String,
    val daemonSdkVersion: String,
    val message: String,
) {
    enum class Level { WARN, ERROR }
}

/**
 * Semver-based compatibility gate for the rule SDK. Identical policy
 * to krit-types' [SdkCompatibility]: a `0.x` minor mismatch is
 * treated as breaking, otherwise matching major + minor is
 * compatible. See `docs/external-rules.md#compatibility-matrix` for
 * the rationale.
 */
internal object SdkCompatibility {
    val DAEMON_SDK_VERSION: String = RuleApiVersion.VERSION

    private const val DEV_SNAPSHOT = "0.0.0-SNAPSHOT"

    fun check(
        jar: String,
        ruleSdkVersion: String,
        daemonSdkVersion: String = DAEMON_SDK_VERSION,
    ): PluginLoadDiagnostic? {
        if (ruleSdkVersion.isBlank()) {
            return PluginLoadDiagnostic(
                jar = jar,
                level = PluginLoadDiagnostic.Level.WARN,
                ruleSdkVersion = "",
                daemonSdkVersion = daemonSdkVersion,
                message = "rule jar is missing the Krit-SDK-Version manifest attribute; " +
                    "rebuild against dev.jasonpearson.krit:krit-rule-api:$daemonSdkVersion",
            )
        }
        if (ruleSdkVersion == DEV_SNAPSHOT || daemonSdkVersion == DEV_SNAPSHOT) {
            return null
        }
        val rule = Semver.parse(ruleSdkVersion)
        val daemon = Semver.parse(daemonSdkVersion)
        if (rule == null) {
            return PluginLoadDiagnostic(
                jar = jar,
                level = PluginLoadDiagnostic.Level.WARN,
                ruleSdkVersion = ruleSdkVersion,
                daemonSdkVersion = daemonSdkVersion,
                message = "could not parse rule Krit-SDK-Version '$ruleSdkVersion'; " +
                    "expected semver matching daemon $daemonSdkVersion",
            )
        }
        if (daemon == null) return null
        if (rule.major == daemon.major && rule.minor == daemon.minor) {
            return null
        }
        val breaking = rule.major != daemon.major ||
            (rule.major == 0 && rule.minor != daemon.minor)
        val level = if (breaking) PluginLoadDiagnostic.Level.ERROR else PluginLoadDiagnostic.Level.WARN
        val reason = when {
            rule.major != daemon.major -> "major version mismatch"
            rule.major == 0 -> "0.x minor version mismatch (treated as breaking under semver)"
            else -> "minor version differs"
        }
        val verb = if (breaking) "is incompatible with" else "may not be fully compatible with"
        return PluginLoadDiagnostic(
            jar = jar,
            level = level,
            ruleSdkVersion = ruleSdkVersion,
            daemonSdkVersion = daemonSdkVersion,
            message = "rule jar built against krit-rule-api $ruleSdkVersion " +
                "$verb daemon krit-rule-api $daemonSdkVersion ($reason); " +
                "rebuild against $daemonSdkVersion",
        )
    }
}

internal data class Semver(val major: Int, val minor: Int, val patch: Int) {
    companion object {
        // Accept `1.2.3`, `1.2.3-rc1`, `1.2.3+build.7`. Pre-release / build
        // metadata is ignored for compatibility comparisons — the policy
        // is expressed in terms of MAJOR.MINOR only.
        private val SEMVER = Regex("""^(\d+)\.(\d+)\.(\d+)(?:[-+].*)?$""")

        fun parse(s: String): Semver? {
            val m = SEMVER.matchEntire(s.trim()) ?: return null
            return Semver(
                major = m.groupValues[1].toInt(),
                minor = m.groupValues[2].toInt(),
                patch = m.groupValues[3].toInt(),
            )
        }
    }
}

internal data class CapabilityViolation(
    val ruleId: String,
    val unsupported: List<String>,
)

/**
 * Source of truth for the capabilities the krit-fir daemon can
 * actually deliver into a rule's [`RuleContext`]. Anything outside
 * this set is rejected at jar-load time so a rule cannot silently run
 * without the facts it asked for.
 *
 * Differs from krit-types' supported set: the krit-fir backend
 * accepts `NEEDS_FIR` (rules that opt into FIR-only facts) because
 * the FIR helpers land alongside the loader. Rule-execution support
 * for the project-scope payload contexts (`NEEDS_GRADLE`, etc.) lands
 * in a later PR; the daemon advertises them as supported now so a
 * jar shipped today doesn't reject when the FIR runtime catches up.
 */
internal object PluginCapabilities {
    val SUPPORTED: Set<String> = setOf(
        Capability.NEEDS_RESOLVER.name,
        Capability.NEEDS_FIR.name,
        Capability.NEEDS_PARSED_FILES.name,
        Capability.NEEDS_GRADLE.name,
        Capability.NEEDS_MANIFEST.name,
        Capability.NEEDS_RESOURCES.name,
        Capability.NEEDS_MODULE_INDEX.name,
        Capability.NEEDS_CROSS_FILE.name,
    )

    fun unsupported(needs: List<String>): List<String> =
        needs.filter { it !in SUPPORTED }

    fun buildLoadDiagnostic(
        jar: String,
        ruleSdkVersion: String,
        daemonSdkVersion: String,
        violations: List<CapabilityViolation>,
    ): PluginLoadDiagnostic {
        // Sort by rule ID so the message is deterministic across
        // ServiceLoader iteration order — otherwise the same offending
        // jar would produce different diagnostic strings on different
        // runs, breaking exact-match assertions in downstream tests.
        val sorted = violations.sortedBy { it.ruleId }
        val rendered = sorted.joinToString(separator = "; ") { (id, caps) ->
            "$id: ${caps.joinToString(separator = ", ")}"
        }
        return PluginLoadDiagnostic(
            jar = jar,
            level = PluginLoadDiagnostic.Level.ERROR,
            ruleSdkVersion = ruleSdkVersion,
            daemonSdkVersion = daemonSdkVersion,
            message = "rule jar declares capabilities the krit-fir daemon does not " +
                "yet provide to plugin rules; the rule would run without the facts " +
                "it asked for. Remove the declaration(s) or wait for support. " +
                "Unsupported: [$rendered]",
        )
    }
}

/**
 * Process-wide, synchronized registry of loaded plugin jars and their
 * rules. The K2-backed daemon may rebuild the per-request analysis
 * session, but the loaded JARs are stable across the daemon lifetime
 * so we keep the registry global. Matches krit-types' lifecycle.
 */
internal object PluginRuleRegistry {

    private val classloaders = mutableListOf<URLClassLoader>()
    private val loadedJars = linkedSetOf<String>()
    private val ruleIdsByJar = linkedMapOf<String, MutableList<String>>()
    private val rules = linkedMapOf<String, LoadedPluginRule>()
    private val diagnostics: MutableList<PluginLoadDiagnostic> = mutableListOf()

    @Synchronized
    fun load(jars: List<String>) {
        for (jar in jars) {
            val file = File(jar).canonicalFile
            if (!file.isFile) {
                throw IllegalArgumentException("plugin jar not found: ${file.path}")
            }
            if (file.path in loadedJars) {
                continue
            }
            val sdkVersion = readSdkVersion(file)
            val sdkDiagnostic = SdkCompatibility.check(file.path, sdkVersion)
            sdkDiagnostic?.let { diagnostics.add(it) }
            if (sdkDiagnostic?.level == PluginLoadDiagnostic.Level.ERROR) {
                loadedJars.add(file.path)
                continue
            }
            val loader = URLClassLoader(arrayOf(file.toURI().toURL()), KritRule::class.java.classLoader)
            val staged: List<LoadedPluginRule>
            try {
                staged = stageRules(loader, sdkVersion)
            } catch (t: Throwable) {
                loader.closeQuietly()
                throw t
            }
            val violations = staged.mapNotNull { loaded ->
                val unsupported = PluginCapabilities.unsupported(loaded.descriptor.needs)
                if (unsupported.isEmpty()) null else CapabilityViolation(loaded.descriptor.ruleId, unsupported)
            }
            if (violations.isNotEmpty()) {
                diagnostics.add(
                    PluginCapabilities.buildLoadDiagnostic(
                        jar = file.path,
                        ruleSdkVersion = sdkVersion,
                        daemonSdkVersion = SdkCompatibility.DAEMON_SDK_VERSION,
                        violations = violations,
                    ),
                )
                loadedJars.add(file.path)
                loader.closeQuietly()
                continue
            }
            val jarRuleIds = ruleIdsByJar.getOrPut(file.path) { mutableListOf() }
            for (loaded in staged) {
                jarRuleIds.add(loaded.descriptor.ruleId)
                rules[loaded.descriptor.ruleId] = loaded
            }
            loadedJars.add(file.path)
            classloaders.add(loader)
        }
    }

    // Loader close is best-effort: failing to release JAR file
    // handles shouldn't mask the diagnostic or error we're about to
    // surface, since the diagnostic is the user-actionable signal.
    private fun URLClassLoader.closeQuietly() {
        try {
            close()
        } catch (_: Exception) {
        }
    }

    private fun stageRules(loader: URLClassLoader, sdkVersion: String): List<LoadedPluginRule> {
        val staged = mutableListOf<LoadedPluginRule>()
        for (rule in ServiceLoader.load(KritRule::class.java, loader)) {
            val info = rule.javaClass.getAnnotation(KritRuleInfo::class.java)
            val descriptor = if (info != null) {
                PluginRuleDescriptor(
                    ruleId = info.id,
                    category = info.category,
                    severity = info.severity.name.lowercase(),
                    maturity = info.maturity.name.lowercase(),
                    languages = info.languages.map { it.name.lowercase() },
                    needs = info.needs.map { it.name },
                    sdkVersion = sdkVersion,
                )
            } else {
                PluginRuleDescriptor(
                    ruleId = rule.javaClass.name,
                    category = "custom",
                    severity = "warning",
                    maturity = "experimental",
                    languages = listOf("kotlin"),
                    needs = emptyList(),
                    sdkVersion = sdkVersion,
                )
            }
            staged.add(LoadedPluginRule(rule, descriptor))
        }
        return staged
    }

    @Synchronized
    fun diagnosticsForJars(jars: List<String>): List<PluginLoadDiagnostic> {
        if (jars.isEmpty()) return emptyList()
        val wanted = jars.map { File(it).canonicalFile.path }.toSet()
        return diagnostics.filter { it.jar in wanted }
    }

    @Synchronized
    fun descriptors(ruleIds: List<String>? = null): List<PluginRuleDescriptor> {
        val wanted = ruleIds?.toSet()
        return rules.values
            .asSequence()
            .filter { wanted == null || it.descriptor.ruleId in wanted }
            .map { it.descriptor }
            .sortedBy { it.ruleId }
            .toList()
    }

    @Synchronized
    fun descriptorsForJars(jars: List<String>): List<PluginRuleDescriptor> {
        if (jars.isEmpty()) {
            return descriptors()
        }
        val wanted = linkedSetOf<String>()
        for (jar in jars) {
            val path = File(jar).canonicalFile.path
            wanted.addAll(ruleIdsByJar[path].orEmpty())
        }
        return descriptors(wanted.toList())
    }

    @Synchronized
    fun selected(ruleIds: List<String>?): List<LoadedPluginRule> {
        val wanted = ruleIds?.toSet()
        return rules.values
            .asSequence()
            .filter { wanted == null || it.descriptor.ruleId in wanted }
            .sortedBy { it.descriptor.ruleId }
            .toList()
    }

    /** Drop all loaded state. Test-only — production code never resets. */
    @Synchronized
    internal fun resetForTesting() {
        for (loader in classloaders) loader.closeQuietly()
        classloaders.clear()
        loadedJars.clear()
        ruleIdsByJar.clear()
        rules.clear()
        diagnostics.clear()
    }

    private fun readSdkVersion(file: File): String {
        return try {
            JarFile(file).use { jar ->
                jar.manifest?.mainAttributes?.getValue("Krit-SDK-Version").orEmpty()
            }
        } catch (_: Exception) {
            ""
        }
    }
}
