package dev.jasonpearson.krit.custom

import org.gradle.api.provider.Property

/**
 * DSL for the `dev.jasonpearson.krit.custom` plugin.
 *
 * Example:
 * ```
 * kritCustomRules {
 *     vendorId.set("acme")
 *     defaultSeverity.set("warning")
 * }
 * ```
 *
 * All properties have sensible defaults — leaving the block empty is valid.
 */
interface KritCustomRulesExtension {
    /**
     * Maven coordinate version of `dev.jasonpearson.krit:krit-rule-api` added
     * to the consumer's `implementation` configuration. Defaults to the plugin's
     * own version.
     */
    val ruleApiVersion: Property<String>

    /**
     * Value written into the `Krit-SDK-Version` manifest attribute of the
     * produced rule jar. Defaults to [ruleApiVersion].
     */
    val sdkVersion: Property<String>

    /**
     * Vendor identifier used as the default rule namespace prefix. Recorded in
     * the manifest as `Krit-Vendor-Id` for downstream tooling. Defaults to
     * `"custom"`.
     */
    val vendorId: Property<String>

    /**
     * Default rule severity (`"error"`, `"warning"`, or `"info"`) recorded in
     * the manifest as `Krit-Default-Severity`. Individual rules can still
     * override via `@KritRuleInfo(severity = ...)`. Defaults to `"warning"`.
     */
    val defaultSeverity: Property<String>
}
