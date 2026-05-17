package com.example.krit

import dev.jasonpearson.krit.api.Finding
import dev.jasonpearson.krit.api.KritFile
import dev.jasonpearson.krit.api.KritRule
import dev.jasonpearson.krit.api.KritRuleInfo
import dev.jasonpearson.krit.api.RuleContext
import dev.jasonpearson.krit.api.Severity

/**
 * Flags top-level `val`/`const val` declarations whose name looks like it
 * holds a credential (PASSWORD / SECRET / TOKEN / API_KEY) and whose value is
 * a non-empty string literal. Test sources are intentionally exempted.
 *
 * This catches the classic "committed `JWT_SECRET = "..."`" mistake at lint
 * time without depending on type information.
 */
@KritRuleInfo(
    id = "playground.NoHardcodedSecret",
    category = "playground-security",
    severity = Severity.ERROR,
)
class NoHardcodedSecretRule : KritRule {

    // Names are matched on segment boundaries (start/end of string or `_`) so
    // `JWT_SECRET` and `DATABASE_PASSWORD` both match. `\b` would not, because
    // `_` is a Java word-character.
    private val secretName = Regex(
        """(?:^|_)(PASSWORD|PASSWD|SECRET|TOKEN|APIKEY|API_KEY|PRIVATEKEY|PRIVATE_KEY)(?:_|$)"""
    )

    private val declaration =
        Regex("""^\s*(?:const\s+)?val\s+([A-Z][A-Z0-9_]*)\s*(?::\s*\w+\s*)?=\s*"([^"]+)"""")

    override fun check(file: KritFile, ctx: RuleContext): List<Finding> {
        if (isTestSource(file.path)) return emptyList()

        val findings = mutableListOf<Finding>()
        file.text.lineSequence().forEachIndexed { idx, raw ->
            val line = raw.substringBefore("//")
            val match = declaration.find(line) ?: return@forEachIndexed
            val name = match.groupValues[1]
            if (!secretName.containsMatchIn(name)) return@forEachIndexed
            val literal = match.groupValues[2]
            if (literal.isBlank()) return@forEachIndexed
            findings.add(
                Finding(
                    message = "Hardcoded credential in `$name`. " +
                        "Move secrets to environment variables or a secret manager.",
                    line = idx + 1,
                    column = raw.indexOf(name) + 1,
                    confidence = 0.9,
                )
            )
        }
        return findings
    }

    private fun isTestSource(path: String): Boolean =
        path.contains("/src/test/") || path.contains("/src/androidTest/")
}
