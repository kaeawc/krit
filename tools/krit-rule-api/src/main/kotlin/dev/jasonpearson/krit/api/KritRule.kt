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
 * Base class for type-aware rules: unpacks [KritFile.ktFile] and
 * [RuleContext.resolver] so the rule body sees non-null PSI and
 * resolver. Subclasses must declare [Capability.NEEDS_RESOLVER] on
 * their [KritRuleInfo] — when the resolver isn't available (parse
 * failure, non-Kotlin source, or `NEEDS_RESOLVER` not declared) the
 * rule produces no findings.
 */
abstract class TypeAwareRule : KritRule {
    final override fun check(file: KritFile, ctx: RuleContext): List<Finding> {
        val ktFile = file.ktFile ?: return emptyList()
        val resolver = ctx.resolver ?: return emptyList()
        return check(file, ctx, ktFile, resolver)
    }

    /**
     * Analyze [ktFile] with full PSI + [resolver] access. Both
     * arguments are guaranteed non-null.
     */
    abstract fun check(
        file: KritFile,
        ctx: RuleContext,
        ktFile: KtFile,
        resolver: Resolver,
    ): List<Finding>
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
 * resolver-backed session for the file. Returning typed primitives
 * (rather than JetBrains-internal symbol types) keeps rule jars off
 * the Analysis API artifacts.
 *
 * The surface is deliberately small. Additions are source-compatible;
 * underlying KAA calls may shift between Krit releases — see
 * `docs/external-rules.md` for the stability contract.
 */
interface Resolver {
    /**
     * Resolves [call] to its target callable. Returns `null` when the
     * call is unresolved (e.g. references a missing import or a
     * member of an unresolved receiver). Non-null results carry every
     * call-level fact in [CallResolution] so callers don't pay for
     * the resolution twice.
     */
    fun resolveCall(call: KtCallExpression): CallResolution?

    /**
     * Returns true when [lambda] is bound to a
     * `kotlin.coroutines.SuspendFunction*` type — either because the
     * lambda literal is itself `suspend { ... }` or because it was
     * passed to a `suspend () -> R` parameter at a call site. Returns
     * false when the lambda is unresolved.
     */
    fun isLambdaSuspend(lambda: KtLambdaExpression): Boolean

    /**
     * Returns the rendered fully-qualified type of [expression], or
     * `null` when the type is unresolved. The rendering format is
     * implementation-defined; prefer this for diagnostic messages,
     * not for parsing.
     */
    fun expressionType(expression: KtExpression): String?
}

/**
 * Successfully-resolved call target. Fields hold every call-level
 * fact the resolver can answer cheaply once a call has been resolved.
 * Returned by [Resolver.resolveCall]; rules that only need a single
 * fact still get the rest for free.
 */
data class CallResolution(
    /**
     * Fully-qualified name of the resolved callable target
     * (e.g. `kotlinx.coroutines.delay`).
     */
    val fqName: String,
    /** True when the target is a `suspend` function. */
    val isSuspend: Boolean,
)

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

/** Capabilities a custom rule needs from the Krit daemon. */
enum class Capability {
    NEEDS_RESOLVER,
    NEEDS_CROSS_FILE,
    NEEDS_MODULE_INDEX,
    NEEDS_PARSED_FILES,
    NEEDS_MANIFEST,
    NEEDS_RESOURCES,
    NEEDS_GRADLE,
}

/** Safety tier for a custom rule autofix. */
enum class FixSafety { COSMETIC, IDIOMATIC, SEMANTIC }
