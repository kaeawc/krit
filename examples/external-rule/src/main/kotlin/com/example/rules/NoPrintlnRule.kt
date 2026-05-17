package com.example.rules

import dev.jasonpearson.krit.api.Finding
import dev.jasonpearson.krit.api.KritFile
import dev.jasonpearson.krit.api.KritRule
import dev.jasonpearson.krit.api.KritRuleInfo
import dev.jasonpearson.krit.api.RuleContext
import dev.jasonpearson.krit.api.Severity

/**
 * Example custom Krit rule: flag bare `println(` invocations.
 *
 * Intentionally uses a line scan rather than an AST walk so the
 * example stays self-contained. A production rule would consult
 * `file.ktFile` (PSI) and skip occurrences inside strings, comments,
 * and KDoc — see the project's "Rule Implementation Guardrails" in
 * CLAUDE.md.
 */
@KritRuleInfo(
    id = "example.NoPrintln",
    category = "custom",
    severity = Severity.WARNING,
)
class NoPrintlnRule : KritRule {
    override fun check(file: KritFile, ctx: RuleContext): List<Finding> {
        val findings = mutableListOf<Finding>()
        file.text.lineSequence().forEachIndexed { index, line ->
            val column = line.indexOf("println(")
            if (column >= 0) {
                findings.add(
                    Finding(
                        message = "Avoid println; use a logger instead.",
                        line = index + 1,
                        column = column + 1,
                    ),
                )
            }
        }
        return findings
    }
}
