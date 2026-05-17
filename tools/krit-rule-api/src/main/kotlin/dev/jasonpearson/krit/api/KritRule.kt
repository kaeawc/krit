package dev.jasonpearson.krit.api

import org.jetbrains.kotlin.psi.KtCallExpression
import org.jetbrains.kotlin.psi.KtExpression
import org.jetbrains.kotlin.psi.KtFile
import org.jetbrains.kotlin.psi.KtLambdaExpression

/**
 * ServiceLoader entry point implemented by Kotlin-authored Krit rules.
 */
interface KritRule {
    /**
     * Analyze one Kotlin source file and return findings in source order.
     */
    fun check(file: KritFile, ctx: RuleContext): List<Finding>
}

/**
 * Runtime metadata for a Kotlin-authored Krit rule.
 *
 * The ServiceLoader contract uses [KritRule] as the implementation type, so
 * the annotation intentionally has a distinct name.
 */
@Target(AnnotationTarget.CLASS)
@Retention(AnnotationRetention.RUNTIME)
annotation class KritRuleInfo(
    val id: String,
    val category: String,
    val severity: Severity = Severity.WARNING,
    val languages: Array<Language> = [Language.KOTLIN],
    val needs: Array<Capability> = [],
    val maturity: Maturity = Maturity.EXPERIMENTAL,
)

/**
 * Kotlin source file view exposed to custom rules.
 *
 * `ktFile` is the Kotlin compiler PSI root for the source. It is `null`
 * when the daemon could not load the file (e.g. parse errors) or when
 * the rule was invoked through a path that does not parse Kotlin source.
 */
class KritFile(
    val path: String,
    val text: String,
    val ktFile: KtFile? = null,
)

/**
 * Narrow, type-aware queries backed by the Kotlin Analysis API.
 *
 * Available on [RuleContext.resolver] when the rule declares
 * [Capability.NEEDS_RESOLVER] and the daemon successfully built a
 * resolver-backed session for the file. Each method opens (or re-uses)
 * a Kotlin Analysis API session under the hood; returning typed
 * primitives avoids leaking JetBrains-internal types onto the rule API
 * surface, so rule jars do not need to depend on the Analysis API
 * artifacts directly.
 *
 * The bridge methods are intentionally minimal — additions are
 * source-compatible (default implementations may evolve in minor
 * releases), but the surface is deliberately small to keep the API
 * stable. See `docs/external-rules.md` for the long-form contract.
 */
interface Resolver {
    /** Returns true if the call's resolved target is a `suspend` function. */
    fun isSuspendCall(call: KtCallExpression): Boolean

    /**
     * Returns the fully-qualified name of the resolved callable target
     * for [call] (e.g. `kotlinx.coroutines.delay`), or `null` if the
     * target is unresolved.
     */
    fun resolvedCallFqName(call: KtCallExpression): String?

    /**
     * Returns true when [lambda]'s functional type is a
     * `kotlin.coroutines.SuspendFunction*` — i.e. the lambda body
     * itself is treated as a `suspend` block. Returns false when the
     * lambda's type is unresolved.
     */
    fun isLambdaSuspend(lambda: KtLambdaExpression): Boolean

    /**
     * Returns the rendered fully-qualified type of [expression], or
     * `null` when the type is unresolved. The exact rendering format is
     * implementation-defined; prefer this for diagnostic messages, not
     * for parsing.
     */
    fun expressionType(expression: KtExpression): String?
}

/**
 * Per-invocation context passed to custom rules.
 *
 * `config` is the per-rule options map from the consumer's `krit.yml`:
 *
 *   pluginRules:
 *     acme.NoTodo:
 *       options:
 *         maxLineLength: 100
 *
 * Values come straight from the YAML parser, so use the typed accessors
 * (`intOption`, `boolOption`, `stringOption`, `stringListOption`) rather
 * than casting. The map is empty when the user did not configure the rule.
 */
class RuleContext(
    val ruleId: String,
    val config: Map<String, Any?> = emptyMap(),
    /**
     * Type-aware queries backed by the Kotlin Analysis API. Non-null
     * only when the rule declared [Capability.NEEDS_RESOLVER] and the
     * daemon successfully prepared a session for the current file.
     */
    val resolver: Resolver? = null,
) {
    /** Returns the [key] option as a String, or [default] if absent / wrong type. */
    fun stringOption(key: String, default: String = ""): String =
        (config[key] as? String) ?: default

    /** Returns the [key] option as a Boolean, or [default] if absent / wrong type. */
    fun boolOption(key: String, default: Boolean = false): Boolean =
        (config[key] as? Boolean) ?: default

    /**
     * Returns the [key] option as an Int, or [default] if absent or
     * not parseable as a 32-bit integer. Accepts numeric YAML values
     * (Int, Long, Double) and decimal string literals.
     */
    fun intOption(key: String, default: Int = 0): Int {
        return when (val v = config[key]) {
            is Int -> v
            is Long -> v.toInt()
            is Double -> v.toInt()
            is String -> v.toIntOrNull() ?: default
            else -> default
        }
    }

    /**
     * Returns the [key] option as a list of strings, dropping any
     * non-string entries. Returns an empty list when the option is
     * absent or not a list.
     */
    fun stringListOption(key: String): List<String> {
        val raw = config[key] as? List<*> ?: return emptyList()
        return raw.mapNotNull { it as? String }
    }
}

/**
 * A finding emitted by a custom Kotlin rule.
 */
data class Finding(
    val message: String,
    val line: Int,
    val column: Int = 1,
    val startByte: Int = 0,
    val endByte: Int = 0,
    val confidence: Double = 0.75,
    val fix: Fix? = null,
)

/**
 * Text edit offered as an autofix for a finding.
 */
data class Fix(
    val startLine: Int,
    val endLine: Int,
    val replacement: String,
    val safety: FixSafety = FixSafety.IDIOMATIC,
)

/** Supported source languages for custom rule dispatch. */
enum class Language { KOTLIN, JAVA, XML }

/** User-facing severity for a finding. */
enum class Severity { ERROR, WARNING, INFO }

/** Rule maturity used for default activation and experimental gating. */
enum class Maturity { STABLE, EXPERIMENTAL, DEPRECATED }

/**
 * Capabilities a custom rule needs from the Krit daemon.
 *
 * Each value documents which `RuleContext` hook the daemon wires up when
 * the rule declares it. Capabilities marked `@Deprecated` are not yet
 * delivered to plugin rules — declaring one of those fails the jar at
 * load time with a clear message (see `PluginRules.kt`). The
 * load-time gate exists so a typo or a too-optimistic declaration
 * cannot silently degrade to "rule runs without the facts it asked
 * for". Tracked on https://github.com/kaeawc/krit/issues/308.
 *
 * Adding a new capability is a minor-version change (additive, default
 * not-required). Promoting a deprecated entry to "supported" is also a
 * minor-version change. Removing a deprecated entry is a major-version
 * change.
 */
enum class Capability {
    /**
     * Populates [RuleContext.resolver] with a [Resolver] backed by per-
     * call Kotlin Analysis API sessions. Honored when the daemon
     * successfully prepares a session for the current file.
     */
    NEEDS_RESOLVER,

    @Deprecated(
        message = "NEEDS_CROSS_FILE is not yet delivered to plugin rules. " +
            "Declaring it causes the rule jar to fail at load time. Tracked " +
            "on https://github.com/kaeawc/krit/issues/308.",
        level = DeprecationLevel.WARNING,
    )
    NEEDS_CROSS_FILE,

    @Deprecated(
        message = "NEEDS_MODULE_INDEX is not yet delivered to plugin rules. " +
            "Declaring it causes the rule jar to fail at load time. Tracked " +
            "on https://github.com/kaeawc/krit/issues/308.",
        level = DeprecationLevel.WARNING,
    )
    NEEDS_MODULE_INDEX,

    /**
     * Indicates the rule walks a parsed Kotlin file. Always satisfied
     * for Kotlin plugin rules — the daemon parses the source before
     * invoking `check()` and exposes the result on [KritFile.ktFile].
     * Declaring this capability is a forward-compatible hint; omitting
     * it changes nothing today.
     */
    NEEDS_PARSED_FILES,

    @Deprecated(
        message = "NEEDS_MANIFEST is not yet delivered to plugin rules. " +
            "Declaring it causes the rule jar to fail at load time. Tracked " +
            "on https://github.com/kaeawc/krit/issues/308.",
        level = DeprecationLevel.WARNING,
    )
    NEEDS_MANIFEST,

    @Deprecated(
        message = "NEEDS_RESOURCES is not yet delivered to plugin rules. " +
            "Declaring it causes the rule jar to fail at load time. Tracked " +
            "on https://github.com/kaeawc/krit/issues/308.",
        level = DeprecationLevel.WARNING,
    )
    NEEDS_RESOURCES,

    @Deprecated(
        message = "NEEDS_GRADLE is not yet delivered to plugin rules. " +
            "Declaring it causes the rule jar to fail at load time. Tracked " +
            "on https://github.com/kaeawc/krit/issues/308.",
        level = DeprecationLevel.WARNING,
    )
    NEEDS_GRADLE,
}

/** Safety tier for a custom rule autofix. */
enum class FixSafety { COSMETIC, IDIOMATIC, SEMANTIC }
