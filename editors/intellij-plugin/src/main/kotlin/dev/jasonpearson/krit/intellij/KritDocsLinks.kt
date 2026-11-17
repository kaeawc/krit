package dev.jasonpearson.krit.intellij

/**
 * Per-rule docs links (the `docsUrl` of `krit --list-rules --format json`)
 * and the annotation tooltip that shows them. Free of IntelliJ classes.
 */
object KritDocsLinks {
    /** Rule id to docs link for the rules that have one. */
    fun byRule(rules: List<KritRuleInfo>): Map<String, String> =
        rules.mapNotNull { rule -> rule.docsLink?.let { rule.id to it } }.toMap()

    /**
     * The tooltip HTML for [finding]: its message, plus a link to the rule's
     * docs when [docsLink] is known. Null without a link, so the annotation
     * keeps the platform's default tooltip. The editor opens http(s) links
     * in the browser.
     */
    fun tooltip(finding: KritFinding, docsLink: String?): String? {
        val link = docsLink ?: return null
        return "<html>${escape(finding.displayMessage)}<br>" +
            "<a href=\"${escape(link)}\">${escape(finding.rule)} documentation</a></html>"
    }

    fun escape(s: String): String =
        s.replace("&", "&amp;").replace("<", "&lt;").replace(">", "&gt;").replace("\"", "&quot;")
}
