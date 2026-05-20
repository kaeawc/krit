package dev.jasonpearson.krit.fir.plugins

import dev.jasonpearson.krit.api.Capability
import dev.jasonpearson.krit.api.Finding
import dev.jasonpearson.krit.api.FixSafety
import dev.jasonpearson.krit.api.KritFile
import dev.jasonpearson.krit.api.Resolver
import dev.jasonpearson.krit.api.RuleContext

/**
 * Driver for the `analyzeFile` RPC. Wires loaded plugin rules
 * ([PluginRuleRegistry]) through the [RuleContext] / [Finding]
 * surface that lives in `krit-rule-api`, then produces a wire-shape
 * response matching krit-types' `buildAnalyzeFileResponse`.
 *
 * A FIR-backed [Resolver] is supplied to rules that declared
 * [Capability.NEEDS_RESOLVER]; rules that didn't opt in see
 * `RuleContext.resolver` as null even when a resolver is available,
 * matching krit-types' opt-in semantics.
 *
 * The project-scope payload contexts (`gradle`, `manifest`,
 * `resources`, `moduleIndex`, `crossFile`) still ride as null —
 * request parsers for them land in a follow-up.
 */
internal object PluginRuleRunner {

    /**
     * Execute the selected plugin rules against [file] and return both
     * the emitted findings and any per-rule execution failures
     * (caught Throwables) keyed by rule ID. Pass [resolver] = null
     * (the default) for callers that don't want to provide one —
     * rules declaring [Capability.NEEDS_RESOLVER] then see
     * `RuleContext.resolver` as null.
     */
    fun run(
        file: KritFile,
        ruleIds: List<String>?,
        ruleConfigs: Map<String, Map<String, Any?>>?,
        resolver: Resolver? = null,
        projectPayloads: ProjectPayloads = ProjectPayloads.EMPTY,
    ): RunResult {
        val findings = mutableListOf<PluginFinding>()
        val errors = linkedMapOf<String, String>()
        // Build each project-scope context once per request and reuse
        // across every rule's RuleContext. Lazy maps in the wrappers
        // mean an unconsulted context costs nothing beyond construction.
        val gradle = projectPayloads.gradle?.let(::PayloadGradleContext)
        val manifest = projectPayloads.manifest?.let(::PayloadManifestContext)
        val resources = projectPayloads.resources?.let(::PayloadResourcesContext)
        val moduleIndex = projectPayloads.moduleIndex?.let(::PayloadModuleIndexContext)
        val crossFile = projectPayloads.crossFile?.let(::PayloadCrossFileContext)

        for (loaded in PluginRuleRegistry.selected(ruleIds)) {
            val options = ruleConfigs?.get(loaded.descriptor.ruleId).orEmpty()
            val needs = loaded.descriptor.needs
            val ctx = RuleContext(
                ruleId = loaded.descriptor.ruleId,
                config = options,
                resolver = resolver.takeIf { needs.contains(Capability.NEEDS_RESOLVER.name) },
                gradle = gradle.takeIf { needs.contains(Capability.NEEDS_GRADLE.name) },
                manifest = manifest.takeIf { needs.contains(Capability.NEEDS_MANIFEST.name) },
                resources = resources.takeIf { needs.contains(Capability.NEEDS_RESOURCES.name) },
                moduleIndex = moduleIndex.takeIf { needs.contains(Capability.NEEDS_MODULE_INDEX.name) },
                crossFile = crossFile.takeIf { needs.contains(Capability.NEEDS_CROSS_FILE.name) },
            )
            try {
                for (finding in loaded.rule.check(file, ctx)) {
                    findings.add(toPluginFinding(file.path, loaded.descriptor, finding))
                }
            } catch (t: Throwable) {
                errors[loaded.descriptor.ruleId] = t.message ?: "rule failed"
            }
        }
        return RunResult(findings, errors)
    }

    data class RunResult(
        val findings: List<PluginFinding>,
        val errors: Map<String, String>,
    )

    /**
     * Wire representation of a finding emitted by a plugin rule.
     * Shape matches krit-types' `PluginFinding` so the Go-side client
     * unmarshals either backend's response with one struct.
     */
    data class PluginFinding(
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

    data class PluginFix(
        val startLine: Int,
        val endLine: Int,
        val replacement: String,
        val safety: String,
    )

    private fun toPluginFinding(
        path: String,
        descriptor: PluginRuleDescriptor,
        finding: Finding,
    ): PluginFinding = PluginFinding(
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
