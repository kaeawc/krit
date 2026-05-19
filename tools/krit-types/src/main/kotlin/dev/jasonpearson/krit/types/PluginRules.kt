package dev.jasonpearson.krit.types

import dev.jasonpearson.krit.api.Capability
import dev.jasonpearson.krit.api.CrossFileContext
import dev.jasonpearson.krit.api.CrossFileDeclaration
import dev.jasonpearson.krit.api.Finding
import dev.jasonpearson.krit.api.FixSafety
import dev.jasonpearson.krit.api.GradleContext
import dev.jasonpearson.krit.api.KritFile
import dev.jasonpearson.krit.api.KritRule
import dev.jasonpearson.krit.api.KritRuleInfo
import dev.jasonpearson.krit.api.ManifestContext
import dev.jasonpearson.krit.api.ModuleIndexContext
import dev.jasonpearson.krit.api.Resolver
import dev.jasonpearson.krit.api.ResourcesContext
import dev.jasonpearson.krit.api.RuleApiVersion
import dev.jasonpearson.krit.api.RuleContext
import org.jetbrains.kotlin.analysis.api.KaExperimentalApi
import org.jetbrains.kotlin.analysis.api.analyze
import org.jetbrains.kotlin.analysis.api.projectStructure.KaSourceModule
import org.jetbrains.kotlin.analysis.api.resolution.KaCallableMemberCall
import org.jetbrains.kotlin.analysis.api.resolution.singleFunctionCallOrNull
import org.jetbrains.kotlin.analysis.api.resolution.singleVariableAccessCall
import org.jetbrains.kotlin.analysis.api.resolution.symbol
import org.jetbrains.kotlin.analysis.api.symbols.KaFunctionSymbol
import org.jetbrains.kotlin.analysis.api.symbols.KaNamedFunctionSymbol
import org.jetbrains.kotlin.analysis.api.types.KaClassType
import org.jetbrains.kotlin.analysis.api.types.KaFunctionType
import org.jetbrains.kotlin.psi.KtCallExpression
import org.jetbrains.kotlin.psi.KtExpression
import org.jetbrains.kotlin.psi.KtFile
import org.jetbrains.kotlin.psi.KtLabeledExpression
import org.jetbrains.kotlin.psi.KtLambdaArgument
import org.jetbrains.kotlin.psi.KtLambdaExpression
import org.jetbrains.kotlin.psi.KtValueArgument
import org.jetbrains.kotlin.psi.KtValueArgumentList
import java.io.File
import java.net.URLClassLoader
import java.util.IdentityHashMap
import java.util.ServiceLoader
import java.util.jar.JarFile

private data class LoadedPluginRule(
    val rule: KritRule,
    val descriptor: PluginRuleDescriptor,
)

private data class PluginRuleDescriptor(
    val ruleId: String,
    val category: String,
    val severity: String,
    val maturity: String,
    val languages: List<String>,
    val needs: List<String>,
    val sdkVersion: String,
)

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
 * Semver-based compatibility gate for the rule SDK. The compatibility
 * matrix and the rationale for each verdict live in
 * `docs/external-rules.md#compatibility-matrix` — keep that in sync when
 * changing this policy.
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
        // metadata is ignored for compatibility comparisons — the policy is
        // expressed in terms of MAJOR.MINOR only.
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
 * Single source of truth for the set of [Capability] values the daemon
 * can actually deliver into [RuleContext] today. Anything outside this
 * set must be rejected at jar load time so a rule cannot silently run
 * without the facts it asked for. See `docs/external-rules.md#capability-semantics`
 * for the user-facing matrix and the policy.
 */
internal object PluginCapabilities {
    const val TRACKING_ISSUE_URL: String = "https://github.com/kaeawc/krit/issues/357"

    val SUPPORTED: Set<String> = setOf(
        Capability.NEEDS_RESOLVER.name,
        // The daemon always parses the Kotlin file before invoking the
        // rule; declaring NEEDS_PARSED_FILES is honored as a no-op hint
        // rather than rejected, so existing rules that opt in stay
        // forward-compatible.
        Capability.NEEDS_PARSED_FILES.name,
        // Project-scope facts are delivered through the per-call
        // analyzeFile payload by `runKotlinPluginRulesAndMerge` in
        // internal/pipeline/custom_kotlin_rules.go. A rule that
        // declares one of these but runs against a project that has
        // no matching fact (e.g. NEEDS_MANIFEST on a pure-Kotlin
        // library) sees the corresponding RuleContext field as null.
        Capability.NEEDS_GRADLE.name,
        Capability.NEEDS_MANIFEST.name,
        Capability.NEEDS_RESOURCES.name,
        Capability.NEEDS_MODULE_INDEX.name,
        Capability.NEEDS_CROSS_FILE.name,
    )

    /**
     * Capabilities that exist on the rule SPI but are intentionally not
     * supported on the krit-types (KAA) backend — declaring them is a
     * load-time opt-in that the rule needs a different backend. The
     * `everyCapabilityIsClassified` test invariant treats a value here
     * as deliberately-not-in-[SUPPORTED] rather than an accidental
     * omission.
     */
    val FIR_ONLY: Set<String> = setOf(
        Capability.NEEDS_FIR.name,
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
        val firRestricted = sorted.any { v -> v.unsupported.any { it in FIR_ONLY } }
        val message = if (firRestricted) {
            "rule jar declares FIR-only capabilities the krit-types (KAA) backend " +
                "cannot provide. Run with `--oracle-backend=fir` so the krit-fir " +
                "backend hosts the rule, or remove the FIR-only declaration if the " +
                "rule does not actually need it. Unsupported: [$rendered]"
        } else {
            "rule jar declares capabilities the daemon does not yet provide " +
                "to plugin rules; the rule would run without the facts it asked for. " +
                "Remove the declaration(s) or wait for support (tracked on " +
                "$TRACKING_ISSUE_URL). Unsupported: [$rendered]"
        }
        return PluginLoadDiagnostic(
            jar = jar,
            level = PluginLoadDiagnostic.Level.ERROR,
            ruleSdkVersion = ruleSdkVersion,
            daemonSdkVersion = daemonSdkVersion,
            message = message,
        )
    }
}

private object PluginRuleRegistry {
    private val classloaders = mutableListOf<URLClassLoader>()
    private val loadedJars = linkedSetOf<String>()
    private val ruleIdsByJar = linkedMapOf<String, MutableList<String>>()
    private val rules = linkedMapOf<String, LoadedPluginRule>()
    // Multiple diagnostics per jar: a jar can carry both an SDK-compat
    // warn AND a capability-error in the same load. Keeping them as a
    // flat ordered list preserves emission order for deterministic
    // listPlugins responses.
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
                // Mark the jar as visited so a repeat listPlugins call
                // doesn't reopen an incompatible jar; the diagnostic itself
                // surfaces in the response.
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

    // Loader close is best-effort: a failure to release JAR file
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

fun DaemonSession.handleListPlugins(request: DaemonRequest): String {
    val jars = request.pluginJars.orEmpty()
    return try {
        PluginRuleRegistry.load(jars)
        buildListPluginsResponse(
            id = request.id,
            descriptors = PluginRuleRegistry.descriptorsForJars(jars),
            diagnostics = PluginRuleRegistry.diagnosticsForJars(jars),
        )
    } catch (t: Throwable) {
        """{"id":${request.id},"error":"${escJsonStr(t.message ?: "listPlugins failed")}"}"""
    }
}

fun DaemonSession.handleAnalyzeFileWithPlugins(request: DaemonRequest): String {
    val path = request.path
    if (path.isNullOrBlank()) {
        return """{"id":${request.id},"error":"analyzeFile requires path"}"""
    }
    return try {
        PluginRuleRegistry.load(request.pluginJars.orEmpty())
        val ktFile = findKtFile(sourceModule, path)
        val text = ktFile?.text ?: request.source.orEmpty()
        if (text.isBlank()) {
            return """{"id":${request.id},"error":"analyzeFile could not resolve source for ${escJsonStr(path)}"}"""
        }
        val file = KritFile(path = ktFile?.virtualFilePath ?: path, text = text, ktFile = ktFile)
        val findings = mutableListOf<PluginFinding>()
        val errors = linkedMapOf<String, String>()
        val gradleContext = request.gradleProfile?.let(::PayloadGradleContext)
        val manifestContext = request.manifestProfile?.let(::PayloadManifestContext)
        val resourcesContext = request.resourcesProfile?.let(::PayloadResourcesContext)
        val moduleIndexContext = request.modulesProfile?.let(::PayloadModuleIndexContext)
        val crossFileContext = request.crossFileProfile?.let(::PayloadCrossFileContext)
        for (loaded in PluginRuleRegistry.selected(request.ruleIds)) {
            try {
                val options = request.ruleConfigs?.get(loaded.descriptor.ruleId).orEmpty()
                val needs = loaded.descriptor.needs
                val resolver = if (ktFile != null && needs.contains(Capability.NEEDS_RESOLVER.name)) {
                    AnalysisApiResolver(ktFile)
                } else {
                    null
                }
                val ctx = RuleContext(
                    ruleId = loaded.descriptor.ruleId,
                    config = options,
                    resolver = resolver,
                    gradle = gradleContext.takeIf { needs.contains(Capability.NEEDS_GRADLE.name) },
                    manifest = manifestContext.takeIf { needs.contains(Capability.NEEDS_MANIFEST.name) },
                    resources = resourcesContext.takeIf { needs.contains(Capability.NEEDS_RESOURCES.name) },
                    moduleIndex = moduleIndexContext.takeIf { needs.contains(Capability.NEEDS_MODULE_INDEX.name) },
                    crossFile = crossFileContext.takeIf { needs.contains(Capability.NEEDS_CROSS_FILE.name) },
                )
                for (finding in loaded.rule.check(file, ctx)) {
                    findings.add(toPluginFinding(file.path, loaded.descriptor, finding))
                }
            } catch (t: Throwable) {
                errors[loaded.descriptor.ruleId] = t.message ?: "rule failed"
            }
        }
        buildAnalyzeFileResponse(
            id = request.id,
            findings = findings,
            errors = errors,
        )
    } catch (t: Throwable) {
        """{"id":${request.id},"error":"${escJsonStr(t.message ?: "analyzeFile failed")}"}"""
    }
}

/**
 * `Resolver` bridge backed by per-call Kotlin Analysis API sessions,
 * memoized on the requesting PSI element. Each public method opens
 * `analyze(ktFile) {}` and swallows KAA exceptions — KAA throws on
 * half-resolved bodies fairly liberally; bubbling those up would turn
 * one malformed expression into a whole-rule failure. Mirrors the
 * resilience pattern used in `analyzeKtFile`'s call resolver.
 *
 * Identity-keyed caches avoid re-resolving the same call twice when a
 * rule asks `isSuspendCall` and then `resolvedCallFqName` on the same
 * `KtCallExpression`, or asks `isLambdaSuspend` for an outer lambda
 * once per inner suspend call.
 */
private class AnalysisApiResolver(private val ktFile: KtFile) : Resolver {
    private val callTargetCache = IdentityHashMap<KtCallExpression, CallTarget>()
    private val lambdaSuspendCache = IdentityHashMap<KtLambdaExpression, Boolean>()

    override fun isSuspendCall(call: KtCallExpression): Boolean =
        resolveCallTarget(call).isSuspend

    override fun resolvedCallFqName(call: KtCallExpression): String? =
        resolveCallTarget(call).fqName

    override fun isLambdaSuspend(lambda: KtLambdaExpression): Boolean {
        if (!lambda.isInModule()) return false
        lambdaSuspendCache[lambda]?.let { return it }
        val answer = try {
            analyze(ktFile) {
                val call = lambda.enclosingValueCall()
                if (call != null && callHasSuspendFunctionalParam(call)) return@analyze true
                // Fall back to the lambda's own inferred type — covers
                // explicit `suspend { ... }` block expressions whose
                // inferred type already carries the suspend modifier.
                val type = lambda.expressionType
                (type is KaFunctionType) && type.isSuspend
            }
        } catch (_: Throwable) {
            false
        }
        lambdaSuspendCache[lambda] = answer
        return answer
    }

    @OptIn(KaExperimentalApi::class)
    override fun expressionType(expression: KtExpression): String? {
        if (!expression.isInModule()) return null
        return try {
            analyze(ktFile) {
                val type = expression.expressionType ?: return@analyze null
                (type as? KaClassType)?.classId?.asFqNameString() ?: type.toString()
            }
        } catch (_: Throwable) {
            null
        }
    }

    private data class CallTarget(val isSuspend: Boolean, val fqName: String?) {
        companion object {
            val UNKNOWN = CallTarget(isSuspend = false, fqName = null)
        }
    }

    private fun resolveCallTarget(call: KtCallExpression): CallTarget {
        if (!call.isInModule()) return CallTarget.UNKNOWN
        callTargetCache[call]?.let { return it }
        val target = try {
            analyze(ktFile) {
                val callInfo = call.resolveToCall() ?: return@analyze CallTarget.UNKNOWN
                val callable: KaCallableMemberCall<*, *> =
                    callInfo.singleFunctionCallOrNull()
                        ?: callInfo.singleVariableAccessCall()
                        ?: return@analyze CallTarget.UNKNOWN
                val symbol = callable.partiallyAppliedSymbol.symbol
                val suspend = (symbol is KaNamedFunctionSymbol) && symbol.isSuspend
                val fqn = symbol.callableId?.asSingleFqName()?.asString()
                CallTarget(suspend, fqn)
            }
        } catch (_: Throwable) {
            CallTarget.UNKNOWN
        }
        callTargetCache[call] = target
        return target
    }

    // FIR represents a lambda literal passed to a `suspend () -> R`
    // parameter with the lambda's own type unchanged — the suspend
    // conversion happens at the argument boundary. Walk up through the
    // wrapping arg/label/list nodes to find the enclosing call so we
    // can ask the resolved function symbol whether any value parameter
    // is a suspend functional type.
    private fun KtLambdaExpression.enclosingValueCall(): KtCallExpression? {
        var anc: com.intellij.psi.PsiElement? = parent
        while (anc != null) {
            if (anc is KtCallExpression) return anc
            if (anc !is KtLambdaArgument &&
                anc !is KtValueArgument &&
                anc !is KtValueArgumentList &&
                anc !is KtLabeledExpression
            ) {
                return null
            }
            anc = anc.parent
        }
        return null
    }

    // Heuristic — single-lambda-parameter calls are unambiguous; calls
    // with multiple functional parameters where only some are suspend
    // (rare) will be over-approximated as suspend-allowed. Trading a
    // small false-negative cost (suspend-in-non-suspend-lambda misses)
    // for cheap, mapping-independent logic.
    private fun org.jetbrains.kotlin.analysis.api.KaSession.callHasSuspendFunctionalParam(
        call: KtCallExpression,
    ): Boolean {
        val memberCall = call.resolveToCall()?.singleFunctionCallOrNull() ?: return false
        val symbol = memberCall.partiallyAppliedSymbol.symbol as? KaFunctionSymbol ?: return false
        return symbol.valueParameters.any { (it.returnType as? KaFunctionType)?.isSuspend == true }
    }

    private fun org.jetbrains.kotlin.psi.KtElement.isInModule(): Boolean =
        containingKtFile === ktFile
}

/**
 * [GradleContext] view backed by the [GradleProfilePayload] forwarded
 * over the wire. The dependency lookup is built lazily on first call —
 * most rules query SDK/tool versions and never touch deps, so paying
 * the map-construction cost only when needed keeps the common path
 * cheap.
 */
internal class PayloadGradleContext(payload: GradleProfilePayload) : GradleContext {
    override val minSdk: Int? = payload.minSdk
    override val targetSdk: Int? = payload.targetSdk
    override val compileSdk: Int? = payload.compileSdk
    override val kotlinVersion: String? = payload.kotlinVersion
    override val javaTargetVersion: String? = payload.javaTargetVersion
    override val agpVersion: String? = payload.agpVersion

    // group:name -> version (last-write-wins on duplicate coords; the
    // Go-side caller already de-dupes, but Kotlin's map semantics
    // protect against a malformed payload).
    private val versionByCoord: Map<String, String> by lazy {
        val out = HashMap<String, String>(payload.deps.size)
        for (entry in payload.deps) {
            val firstColon = entry.indexOf(':')
            if (firstColon <= 0) continue
            val secondColon = entry.indexOf(':', firstColon + 1)
            if (secondColon <= firstColon + 1) continue
            val coord = entry.substring(0, secondColon)
            val version = entry.substring(secondColon + 1)
            if (version.isNotEmpty()) {
                out[coord] = version
            }
        }
        out
    }

    override fun hasDependency(group: String, name: String): Boolean =
        versionByCoord.containsKey("$group:$name")

    override fun dependencyVersion(group: String, name: String): String? =
        versionByCoord["$group:$name"]
}

/**
 * [ManifestContext] view backed by the [ManifestProfilePayload]
 * forwarded over the wire. Per-component lookup sets are built lazily
 * on first query — rules that only read scalar attributes (package,
 * SDK versions) skip the set construction entirely.
 */
internal class PayloadManifestContext(payload: ManifestProfilePayload) : ManifestContext {
    override val packageName: String? = payload.packageName
    override val minSdk: Int? = payload.minSdk
    override val targetSdk: Int? = payload.targetSdk

    private val permissionSet: Set<String> by lazy { payload.permissions.toHashSet() }
    private val activitySet: Set<String> by lazy { payload.activities.toHashSet() }
    private val exportedActivitySet: Set<String> by lazy { payload.exportedActivities.toHashSet() }
    private val serviceSet: Set<String> by lazy { payload.services.toHashSet() }
    private val exportedServiceSet: Set<String> by lazy { payload.exportedServices.toHashSet() }
    private val receiverSet: Set<String> by lazy { payload.receivers.toHashSet() }
    private val exportedReceiverSet: Set<String> by lazy { payload.exportedReceivers.toHashSet() }

    override fun hasPermission(name: String): Boolean = name in permissionSet
    override fun hasActivity(name: String): Boolean = name in activitySet
    override fun isActivityExported(name: String): Boolean = name in exportedActivitySet
    override fun hasService(name: String): Boolean = name in serviceSet
    override fun isServiceExported(name: String): Boolean = name in exportedServiceSet
    override fun hasReceiver(name: String): Boolean = name in receiverSet
    override fun isReceiverExported(name: String): Boolean = name in exportedReceiverSet
}

/**
 * [ResourcesContext] view backed by the [ResourcesProfilePayload]
 * forwarded over the wire. Maps are built lazily by parsing the flat
 * `"name=value"` wire format — splitting on the first `=` so values
 * containing `=` (e.g. URL-encoded strings) round-trip cleanly.
 */
internal class PayloadResourcesContext(payload: ResourcesProfilePayload) : ResourcesContext {
    private val stringMap: Map<String, String> by lazy { parseNameValueList(payload.strings) }
    private val colorMap: Map<String, String> by lazy { parseNameValueList(payload.colors) }
    private val dimensionMap: Map<String, String> by lazy { parseNameValueList(payload.dimensions) }
    private val drawableSet: Set<String> by lazy { payload.drawables.toHashSet() }
    private val layoutSet: Set<String> by lazy { payload.layouts.toHashSet() }
    private val idSet: Set<String> by lazy { payload.ids.toHashSet() }

    override fun stringValue(name: String): String? = stringMap[name]
    override fun hasString(name: String): Boolean = stringMap.containsKey(name)
    override fun hasDrawable(name: String): Boolean = name in drawableSet
    override fun hasLayout(name: String): Boolean = name in layoutSet
    override fun colorValue(name: String): String? = colorMap[name]
    override fun hasColor(name: String): Boolean = colorMap.containsKey(name)
    override fun dimensionValue(name: String): String? = dimensionMap[name]
    override fun hasDimension(name: String): Boolean = dimensionMap.containsKey(name)
    override fun hasId(name: String): Boolean = name in idSet

    private fun parseNameValueList(entries: List<String>): Map<String, String> {
        val out = HashMap<String, String>(entries.size)
        for (entry in entries) {
            val eq = entry.indexOf('=')
            if (eq <= 0) continue
            out[entry.substring(0, eq)] = entry.substring(eq + 1)
        }
        return out
    }
}

/**
 * [ModuleIndexContext] view backed by the [ModulesProfilePayload]
 * forwarded over the wire. Each module is encoded as the
 * `path|directory|dependsOn-comma-list|sourceRoots-comma-list` shape
 * documented on the payload; the parser splits and indexes on first
 * query.
 */
internal class PayloadModuleIndexContext(payload: ModulesProfilePayload) : ModuleIndexContext {
    private data class Entry(
        val directory: String,
        val dependsOn: List<String>,
        val sourceRoots: List<String>,
    )

    private val byPath: Map<String, Entry> by lazy {
        val out = LinkedHashMap<String, Entry>(payload.modules.size)
        for (line in payload.modules) {
            val parts = line.split('|')
            if (parts.size < 4 || parts[0].isEmpty()) continue
            val deps = parts[2].takeIf { it.isNotEmpty() }?.split(',') ?: emptyList()
            val roots = parts[3].takeIf { it.isNotEmpty() }?.split(',') ?: emptyList()
            out[parts[0]] = Entry(parts[1], deps, roots)
        }
        out
    }

    override val modulePaths: List<String> get() = byPath.keys.toList()
    override fun directoryOf(modulePath: String): String? = byPath[modulePath]?.directory
    override fun dependenciesOf(modulePath: String): List<String> =
        byPath[modulePath]?.dependsOn.orEmpty()
    override fun sourceRootsOf(modulePath: String): List<String> =
        byPath[modulePath]?.sourceRoots.orEmpty()
}

/**
 * [CrossFileContext] view backed by the [CrossFileProfilePayload]
 * forwarded over the wire. Both the FQN→declaration map and the
 * name→reference-files map are built lazily — many cross-file rules
 * only query one direction.
 */
internal class PayloadCrossFileContext(payload: CrossFileProfilePayload) : CrossFileContext {
    private val byFqn: Map<String, CrossFileDeclaration> by lazy {
        val out = HashMap<String, CrossFileDeclaration>(payload.declarations.size)
        for (entry in payload.declarations) {
            val parts = entry.split('|')
            if (parts.size < 4 || parts[0].isEmpty()) continue
            val line = parts[3].toIntOrNull() ?: continue
            val visibility = parts.getOrNull(4)?.takeIf { it.isNotEmpty() }
            out[parts[0]] = CrossFileDeclaration(parts[0], parts[1], parts[2], line, visibility)
        }
        out
    }

    private val refFilesByName: Map<String, List<String>> by lazy {
        val out = HashMap<String, List<String>>(payload.nonCommentRefsByName.size)
        for (entry in payload.nonCommentRefsByName) {
            val sep = entry.indexOf('|')
            if (sep <= 0) continue
            val name = entry.substring(0, sep)
            val files = entry.substring(sep + 1)
                .split(',')
                .filter { it.isNotEmpty() }
            if (files.isNotEmpty()) {
                out[name] = files
            }
        }
        out
    }

    override fun declarationByFqn(fqn: String): CrossFileDeclaration? = byFqn[fqn]
    override fun referenceFiles(name: String): List<String> = refFilesByName[name].orEmpty()
    override fun isReferenced(name: String): Boolean = refFilesByName.containsKey(name)
}

@OptIn(KaExperimentalApi::class)
private fun findKtFile(sourceModule: KaSourceModule, requestedPath: String): KtFile? {
    val resolved = try {
        File(requestedPath).canonicalPath
    } catch (_: Exception) {
        requestedPath
    }
    return sourceModule.psiRoots.filterIsInstance<KtFile>().find { file ->
        file.virtualFilePath == resolved ||
            file.virtualFilePath == requestedPath ||
            file.virtualFilePath.endsWith(requestedPath)
    }
}

private data class PluginFinding(
    val file: String,
    val line: Int,
    val column: Int,
    val startByte: Int,
    val endByte: Int,
    val ruleSet: String,
    val ruleId: String,
    val severity: String,
    val message: String,
    val confidence: Double,
    val fix: PluginFix?,
)

private data class PluginFix(
    val startLine: Int,
    val endLine: Int,
    val replacement: String,
    val safety: String,
)

private fun toPluginFinding(path: String, descriptor: PluginRuleDescriptor, finding: Finding): PluginFinding {
    return PluginFinding(
        file = path,
        line = finding.line,
        column = finding.column,
        startByte = finding.startByte,
        endByte = finding.endByte,
        ruleSet = descriptor.category,
        ruleId = descriptor.ruleId,
        severity = descriptor.severity,
        message = finding.message,
        confidence = finding.confidence,
        fix = finding.fix?.let {
            PluginFix(
                startLine = it.startLine,
                endLine = it.endLine,
                replacement = it.replacement,
                safety = when (it.safety) {
                    FixSafety.COSMETIC -> "cosmetic"
                    FixSafety.IDIOMATIC -> "idiomatic"
                    FixSafety.SEMANTIC -> "semantic"
                },
            )
        },
    )
}

private fun buildListPluginsResponse(
    id: Long,
    descriptors: List<PluginRuleDescriptor>,
    diagnostics: List<PluginLoadDiagnostic>,
): String {
    val sb = StringBuilder()
    sb.append("""{"id":""").append(id).append(""","result":{"rules":[""")
    var first = true
    for (descriptor in descriptors) {
        if (!first) sb.append(',')
        first = false
        sb.append('{')
        sb.append("\"ruleId\":").append(esc(descriptor.ruleId)).append(',')
        sb.append("\"category\":").append(esc(descriptor.category)).append(',')
        sb.append("\"severity\":").append(esc(descriptor.severity)).append(',')
        sb.append("\"maturity\":").append(esc(descriptor.maturity)).append(',')
        appendStringArray(sb, "languages", descriptor.languages)
        sb.append(',')
        appendStringArray(sb, "needs", descriptor.needs)
        if (descriptor.sdkVersion.isNotBlank()) {
            sb.append(',').append("\"sdkVersion\":").append(esc(descriptor.sdkVersion))
        }
        sb.append('}')
    }
    sb.append(']')
    appendDiagnostics(sb, diagnostics)
    sb.append('}').append('}')
    return sb.toString()
}

private fun appendDiagnostics(sb: StringBuilder, diagnostics: List<PluginLoadDiagnostic>) {
    if (diagnostics.isEmpty()) return
    sb.append(",\"diagnostics\":[")
    var first = true
    for (d in diagnostics) {
        if (!first) sb.append(',')
        first = false
        sb.append('{')
        sb.append("\"jar\":").append(esc(d.jar)).append(',')
        sb.append("\"level\":").append(esc(d.level.name.lowercase())).append(',')
        sb.append("\"ruleSdkVersion\":").append(esc(d.ruleSdkVersion)).append(',')
        sb.append("\"daemonSdkVersion\":").append(esc(d.daemonSdkVersion)).append(',')
        sb.append("\"message\":").append(esc(d.message))
        sb.append('}')
    }
    sb.append(']')
}

private fun buildAnalyzeFileResponse(
    id: Long,
    findings: List<PluginFinding>,
    errors: Map<String, String>,
): String {
    val sb = StringBuilder()
    sb.append("""{"id":""").append(id).append(""","result":{"findings":[""")
    var first = true
    for (finding in findings) {
        if (!first) sb.append(',')
        first = false
        sb.append('{')
        sb.append("\"file\":").append(esc(finding.file)).append(',')
        sb.append("\"line\":").append(finding.line).append(',')
        sb.append("\"column\":").append(finding.column).append(',')
        sb.append("\"startByte\":").append(finding.startByte).append(',')
        sb.append("\"endByte\":").append(finding.endByte).append(',')
        sb.append("\"ruleSet\":").append(esc(finding.ruleSet)).append(',')
        sb.append("\"ruleId\":").append(esc(finding.ruleId)).append(',')
        sb.append("\"severity\":").append(esc(finding.severity)).append(',')
        sb.append("\"message\":").append(esc(finding.message)).append(',')
        sb.append("\"confidence\":").append(finding.confidence)
        if (finding.fix != null) {
            sb.append(",\"fix\":{")
            sb.append("\"startLine\":").append(finding.fix.startLine).append(',')
            sb.append("\"endLine\":").append(finding.fix.endLine).append(',')
            sb.append("\"replacement\":").append(esc(finding.fix.replacement)).append(',')
            sb.append("\"safety\":").append(esc(finding.fix.safety))
            sb.append('}')
        }
        sb.append('}')
    }
    sb.append(']')
    if (errors.isNotEmpty()) {
        sb.append(",\"errors\":{")
        var firstErr = true
        for ((ruleId, message) in errors) {
            if (!firstErr) sb.append(',')
            firstErr = false
            sb.append(esc(ruleId)).append(':').append(esc(message))
        }
        sb.append('}')
    }
    sb.append("}}")
    return sb.toString()
}

private fun appendStringArray(sb: StringBuilder, key: String, values: List<String>) {
    sb.append('"').append(key).append("\":[")
    var first = true
    for (value in values) {
        if (!first) sb.append(',')
        first = false
        sb.append(esc(value))
    }
    sb.append(']')
}
