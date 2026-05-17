package dev.jasonpearson.krit.types

import dev.jasonpearson.krit.api.Capability
import dev.jasonpearson.krit.api.Finding
import dev.jasonpearson.krit.api.FixSafety
import dev.jasonpearson.krit.api.KritFile
import dev.jasonpearson.krit.api.KritRule
import dev.jasonpearson.krit.api.KritRuleInfo
import dev.jasonpearson.krit.api.Resolver
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

private object PluginRuleRegistry {
    private val classloaders = mutableListOf<URLClassLoader>()
    private val loadedJars = linkedSetOf<String>()
    private val ruleIdsByJar = linkedMapOf<String, MutableList<String>>()
    private val rules = linkedMapOf<String, LoadedPluginRule>()
    private val diagnosticsByJar = linkedMapOf<String, PluginLoadDiagnostic>()

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
            val diagnostic = SdkCompatibility.check(file.path, sdkVersion)
            diagnostic?.let { diagnosticsByJar[file.path] = it }
            if (diagnostic?.level == PluginLoadDiagnostic.Level.ERROR) {
                // Mark the jar as visited so a repeat listPlugins call
                // doesn't reopen an incompatible jar; the diagnostic itself
                // surfaces in the response.
                loadedJars.add(file.path)
                continue
            }
            val loader = URLClassLoader(arrayOf(file.toURI().toURL()), KritRule::class.java.classLoader)
            try {
                val serviceRules = ServiceLoader.load(KritRule::class.java, loader).iterator()
                val jarRuleIds = ruleIdsByJar.getOrPut(file.path) { mutableListOf() }
                while (serviceRules.hasNext()) {
                    val rule = serviceRules.next()
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
                    jarRuleIds.add(descriptor.ruleId)
                    rules[descriptor.ruleId] = LoadedPluginRule(rule, descriptor)
                }
                loadedJars.add(file.path)
                classloaders.add(loader)
            } catch (t: Throwable) {
                ruleIdsByJar.remove(file.path)
                try {
                    loader.close()
                } catch (_: Exception) {
                }
                throw t
            }
        }
    }

    @Synchronized
    fun diagnosticsForJars(jars: List<String>): List<PluginLoadDiagnostic> {
        if (jars.isEmpty()) return emptyList()
        val wanted = jars.map { File(it).canonicalFile.path }.toSet()
        return diagnosticsByJar.values.filter { it.jar in wanted }
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
        for (loaded in PluginRuleRegistry.selected(request.ruleIds)) {
            try {
                val options = request.ruleConfigs?.get(loaded.descriptor.ruleId).orEmpty()
                val resolver = if (ktFile != null && loaded.descriptor.needs.contains(Capability.NEEDS_RESOLVER.name)) {
                    AnalysisApiResolver(ktFile)
                } else {
                    null
                }
                val ctx = RuleContext(loaded.descriptor.ruleId, options, resolver)
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
