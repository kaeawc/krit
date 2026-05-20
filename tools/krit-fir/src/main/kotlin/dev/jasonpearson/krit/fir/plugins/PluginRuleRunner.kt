package dev.jasonpearson.krit.fir.plugins

import dev.jasonpearson.krit.api.Finding
import dev.jasonpearson.krit.api.FixSafety
import dev.jasonpearson.krit.api.KritFile
import dev.jasonpearson.krit.api.RuleContext

/**
 * Driver for the `analyzeFileWithPlugins` RPC. Wires loaded plugin
 * rules ([PluginRuleRegistry]) through the [RuleContext] / [Finding]
 * surface that lives in `krit-rule-api`, then produces a wire-shape
 * response matching krit-types' `buildAnalyzeFileResponse`.
 *
 * Current scope (PR 3.2):
 *
 * - Rules run with `ktFile = null` and `resolver = null`. Tree-sitter-
 *   style line-scanner rules work today; rules that opt into
 *   `NEEDS_RESOLVER` or expect PSI get the corresponding fields as
 *   null and either degrade gracefully or no-op.
 * - PSI parsing and a real FIR-backed [Resolver] land in PR 3.3.
 * - Project-scope payload contexts (`gradle`, `manifest`, `resources`,
 *   `moduleIndex`, `crossFile`) also land in PR 3.3 alongside the
 *   request parsers for them.
 */
internal object PluginRuleRunner {

    /**
     * Execute the selected plugin rules against [file] and return both
     * the emitted findings and any per-rule execution failures
     * (caught Throwables) keyed by rule ID.
     */
    fun run(
        file: KritFile,
        ruleIds: List<String>?,
        ruleConfigs: Map<String, Map<String, Any?>>?,
    ): RunResult {
        val findings = mutableListOf<PluginFinding>()
        val errors = linkedMapOf<String, String>()
        for (loaded in PluginRuleRegistry.selected(ruleIds)) {
            val options = ruleConfigs?.get(loaded.descriptor.ruleId).orEmpty()
            // resolver and the project-scope context fields stay null
            // on the krit-fir backend today. A rule that declared the
            // matching `NEEDS_*` capability passed the load-time gate;
            // we just don't provide the fact yet. PSI parsing and the
            // FIR-backed Resolver land in PR 3.3.
            val ctx = RuleContext(
                ruleId = loaded.descriptor.ruleId,
                config = options,
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
