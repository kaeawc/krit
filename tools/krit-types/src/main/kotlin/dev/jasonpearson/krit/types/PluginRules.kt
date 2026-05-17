package dev.jasonpearson.krit.types

import dev.jasonpearson.krit.api.Finding
import dev.jasonpearson.krit.api.FixSafety
import dev.jasonpearson.krit.api.KritFile
import dev.jasonpearson.krit.api.KritRule
import dev.jasonpearson.krit.api.KritRuleInfo
import dev.jasonpearson.krit.api.RuleContext
import org.jetbrains.kotlin.analysis.api.KaExperimentalApi
import org.jetbrains.kotlin.analysis.api.projectStructure.KaSourceModule
import org.jetbrains.kotlin.psi.KtFile
import java.io.File
import java.net.URLClassLoader
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

private object PluginRuleRegistry {
    private val classloaders = mutableListOf<URLClassLoader>()
    private val loadedJars = linkedSetOf<String>()
    private val ruleIdsByJar = linkedMapOf<String, MutableList<String>>()
    private val rules = linkedMapOf<String, LoadedPluginRule>()

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
            val loader = URLClassLoader(arrayOf(file.toURI().toURL()), KritRule::class.java.classLoader)
            try {
                val sdkVersion = readSdkVersion(file)
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
        buildListPluginsResponse(request.id, PluginRuleRegistry.descriptorsForJars(jars))
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
                val ctx = RuleContext(loaded.descriptor.ruleId, options)
                for (finding in loaded.rule.check(file, ctx)) {
                    findings.add(toPluginFinding(file.path, loaded.descriptor, finding))
                }
            } catch (t: Throwable) {
                errors[loaded.descriptor.ruleId] = t.message ?: "rule failed"
            }
        }
        buildAnalyzeFileResponse(request.id, findings, errors)
    } catch (t: Throwable) {
        """{"id":${request.id},"error":"${escJsonStr(t.message ?: "analyzeFile failed")}"}"""
    }
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

private fun buildListPluginsResponse(id: Long, descriptors: List<PluginRuleDescriptor>): String {
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
    sb.append("]}}")
    return sb.toString()
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
