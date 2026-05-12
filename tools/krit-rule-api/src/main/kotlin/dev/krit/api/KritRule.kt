package dev.krit.api

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
 */
class KritFile(
    val path: String,
    val text: String,
    val ktFile: Any? = null,
)

/**
 * Per-invocation context passed to custom rules.
 */
class RuleContext(
    val ruleId: String,
)

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
