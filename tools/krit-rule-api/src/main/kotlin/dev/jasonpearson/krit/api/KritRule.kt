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
 * Project-wide Gradle facts derived from build files and the version
 * catalog. Available on [RuleContext.gradle] when the rule declares
 * [Capability.NEEDS_GRADLE] and the daemon has Gradle facts for the
 * project (e.g. running on a bare Kotlin directory still leaves this
 * null).
 *
 * The surface is deliberately small — the underlying daemon-side
 * profile is much larger, but additive growth here is the way to keep
 * rule jars binary-compatible across Krit releases. See
 * `docs/external-rules.md#capability-semantics` for the rationale and
 * the long-form contract.
 */
interface GradleContext {
    /** Android `minSdkVersion`, or null if unknown / not an Android project. */
    val minSdk: Int?

    /** Android `targetSdkVersion`, or null if unknown / not an Android project. */
    val targetSdk: Int?

    /** Android `compileSdkVersion`, or null if unknown / not an Android project. */
    val compileSdk: Int?

    /**
     * Effective Kotlin compiler version (e.g. `"2.0.0"`), or null when
     * the project has no Kotlin tooling configured.
     */
    val kotlinVersion: String?

    /**
     * Effective JVM bytecode target the project compiles for (e.g.
     * `"17"`), or null when undeclared.
     */
    val javaTargetVersion: String?

    /** Android Gradle Plugin version (e.g. `"8.5.0"`), or null when AGP is not applied. */
    val agpVersion: String?

    /**
     * Returns true when the project declares a dependency on
     * `[group]:[name]` (any version, any configuration). The current
     * implementation matches the canonical Maven coordinate exactly;
     * normalization (case folding, alias resolution) may be added in
     * a future minor release without changing this contract.
     */
    fun hasDependency(group: String, name: String): Boolean

    /**
     * Returns the declared version for `[group]:[name]`, or null when
     * the dependency is absent or its version was not resolved (e.g.
     * inherited from a BOM the lightweight parser cannot follow).
     */
    fun dependencyVersion(group: String, name: String): String?
}

/**
 * Parsed `AndroidManifest.xml` view. Available on [RuleContext.manifest]
 * when the rule declares [Capability.NEEDS_MANIFEST] and the daemon
 * detected an `AndroidManifest.xml` in the project (a pure Kotlin
 * library project leaves this null).
 *
 * The surface is intentionally narrow: scalar attributes for the most
 * common access patterns, plus query methods for the unbounded
 * collections (`<uses-permission>`, components). Walk the file directly
 * via `KritFile` if a rule needs richer manifest evidence than this
 * exposes.
 */
interface ManifestContext {
    /** `<manifest package="...">`, or null when unparseable. */
    val packageName: String?

    /** `<uses-sdk android:minSdkVersion="...">`, or null when unset. */
    val minSdk: Int?

    /** `<uses-sdk android:targetSdkVersion="...">`, or null when unset. */
    val targetSdk: Int?

    /** Returns true when the manifest lists `<uses-permission android:name="[name]"/>`. */
    fun hasPermission(name: String): Boolean

    /** Returns true when an `<activity android:name="[name]"/>` is declared. */
    fun hasActivity(name: String): Boolean

    /**
     * Returns true when the named activity is exported. A component is
     * considered exported when `android:exported="true"`, or when the
     * attribute is unset and the component declares at least one
     * `<intent-filter>` (the pre-API-31 implicit-export default).
     * Returns false when the activity is not declared.
     */
    fun isActivityExported(name: String): Boolean

    /** Returns true when a `<service android:name="[name]"/>` is declared. */
    fun hasService(name: String): Boolean

    /**
     * Returns true when the named service is exported (same semantics
     * as [isActivityExported]).
     */
    fun isServiceExported(name: String): Boolean

    /** Returns true when a `<receiver android:name="[name]"/>` is declared. */
    fun hasReceiver(name: String): Boolean

    /**
     * Returns true when the named receiver is exported (same semantics
     * as [isActivityExported]).
     */
    fun isReceiverExported(name: String): Boolean
}

/**
 * Parsed `res/` tree. Available on [RuleContext.resources] when the
 * rule declares [Capability.NEEDS_RESOURCES] and the daemon detected
 * at least one Android `res/` directory.
 *
 * Lookups are by resource name (the identifier you'd reference as
 * `R.string.foo` / `R.drawable.bar` from code). Values are the rendered
 * resource value (translatable string, hex color, dimension literal).
 * `hasXxx(name)` is cheaper than `xxxValue(name) != null` because it
 * skips the rendered-value lookup.
 */
interface ResourcesContext {
    /** Returns the `@string/[name]` value, or null when undeclared. */
    fun stringValue(name: String): String?

    /** Returns true when `@string/[name]` is declared. */
    fun hasString(name: String): Boolean

    /** Returns true when `@drawable/[name]` is declared. */
    fun hasDrawable(name: String): Boolean

    /** Returns true when `@layout/[name]` is declared. */
    fun hasLayout(name: String): Boolean

    /** Returns the `@color/[name]` value (hex), or null when undeclared. */
    fun colorValue(name: String): String?

    /** Returns true when `@color/[name]` is declared. */
    fun hasColor(name: String): Boolean

    /** Returns the `@dimen/[name]` value (e.g. `"16dp"`), or null when undeclared. */
    fun dimensionValue(name: String): String?

    /** Returns true when `@dimen/[name]` is declared. */
    fun hasDimension(name: String): Boolean

    /** Returns true when `@+id/[name]` is declared in any layout. */
    fun hasId(name: String): Boolean
}

/**
 * Gradle module identity + per-module dependency graph. Available on
 * [RuleContext.moduleIndex] when the rule declares
 * [Capability.NEEDS_MODULE_INDEX] and the daemon discovered at least
 * one Gradle module.
 *
 * `modulePaths` is the list of Gradle paths (`:app`, `:core:util`) in
 * the order the daemon discovered them. Use the lookup methods rather
 * than walking the list directly when you only care about one module.
 */
interface ModuleIndexContext {
    /** Discovered Gradle module paths, in daemon-discovery order. */
    val modulePaths: List<String>

    /** Returns the absolute filesystem directory for `[modulePath]`, or null. */
    fun directoryOf(modulePath: String): String?

    /**
     * Returns the Gradle paths the named module declares as project
     * dependencies (any configuration). Empty list when the module has
     * no project deps or is not in the index.
     */
    fun dependenciesOf(modulePath: String): List<String>

    /**
     * Returns the source-root directories for `[modulePath]`. Empty
     * list when the module has no declared source roots or is not in
     * the index.
     */
    fun sourceRootsOf(modulePath: String): List<String>
}

/**
 * Cross-file declaration / reference index. Available on
 * [RuleContext.crossFile] when the rule declares
 * [Capability.NEEDS_CROSS_FILE] and the daemon's cross-file pass ran.
 *
 * The wire payload can be sizable on large projects — declare this
 * capability only when the rule genuinely needs whole-project
 * visibility. The query API stays narrow: FQN → declaration site, and
 * unqualified name → list of files that mention it.
 */
interface CrossFileContext {
    /** Returns the declaration site for `[fqn]` (e.g. `com.acme.Foo`), or null. */
    fun declarationByFqn(fqn: String): CrossFileDeclaration?

    /**
     * Returns the list of files that contain at least one non-comment
     * reference to the unqualified identifier `[name]`. Returns an
     * empty list when the name is unreferenced.
     */
    fun referenceFiles(name: String): List<String>

    /**
     * Returns true when at least one non-comment file references the
     * unqualified identifier `[name]`.
     */
    fun isReferenced(name: String): Boolean
}

/**
 * One declaration site surfaced through [CrossFileContext]. `kind` is
 * one of `class`, `interface`, `object`, `function`, `property`.
 */
data class CrossFileDeclaration(
    val fqn: String,
    val kind: String,
    val file: String,
    val line: Int,
    val visibility: String? = null,
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
    /**
     * Project-wide Gradle facts. Non-null only when the rule declared
     * [Capability.NEEDS_GRADLE] and the daemon could derive Gradle
     * facts for the project.
     */
    val gradle: GradleContext? = null,
    /**
     * Parsed `AndroidManifest.xml` view. Non-null only when the rule
     * declared [Capability.NEEDS_MANIFEST] and the project ships a
     * parseable `AndroidManifest.xml`.
     */
    val manifest: ManifestContext? = null,
    /**
     * Parsed `res/` tree. Non-null only when the rule declared
     * [Capability.NEEDS_RESOURCES] and the project ships at least one
     * Android `res/` directory.
     */
    val resources: ResourcesContext? = null,
    /**
     * Gradle module identity + dependency graph. Non-null only when
     * the rule declared [Capability.NEEDS_MODULE_INDEX] and the daemon
     * discovered at least one Gradle module.
     */
    val moduleIndex: ModuleIndexContext? = null,
    /**
     * Cross-file declaration / reference index. Non-null only when the
     * rule declared [Capability.NEEDS_CROSS_FILE] and the daemon's
     * cross-file pass ran.
     */
    val crossFile: CrossFileContext? = null,
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
 * Each value documents which `RuleContext` hook the daemon wires up
 * when the rule declares it. The daemon either delivers the fact into
 * `RuleContext` or refuses to load the jar — there is no third
 * "advisory" state. See `docs/external-rules.md#capability-semantics`
 * for the user-facing matrix.
 *
 * Adding a new capability is a minor-version change (additive, default
 * not-required). Removing a capability is a major-version change.
 */
enum class Capability {
    /**
     * Populates [RuleContext.resolver] with a [Resolver] backed by per-
     * call Kotlin Analysis API sessions. Honored when the daemon
     * successfully prepares a session for the current file.
     */
    NEEDS_RESOLVER,

    /**
     * Populates [RuleContext.crossFile] with a [CrossFileContext] view
     * of declarations and references indexed across the project.
     */
    NEEDS_CROSS_FILE,

    /**
     * Populates [RuleContext.moduleIndex] with a [ModuleIndexContext]
     * view of every discovered Gradle module and its declared
     * project-dependency edges.
     */
    NEEDS_MODULE_INDEX,

    /**
     * Indicates the rule walks a parsed Kotlin file. Always satisfied
     * for Kotlin plugin rules — the daemon parses the source before
     * invoking `check()` and exposes the result on [KritFile.ktFile].
     * Declaring this capability is a forward-compatible hint; omitting
     * it changes nothing today.
     */
    NEEDS_PARSED_FILES,

    /**
     * Populates [RuleContext.manifest] with a [ManifestContext] view
     * of the project's parsed `AndroidManifest.xml`.
     */
    NEEDS_MANIFEST,

    /**
     * Populates [RuleContext.resources] with a [ResourcesContext] view
     * of the project's parsed `res/` tree (strings, drawables,
     * layouts, colors, dimens, ids).
     */
    NEEDS_RESOURCES,

    /**
     * Populates [RuleContext.gradle] with a [GradleContext] view of
     * the project's parsed Gradle facts (SDK versions, tool versions,
     * declared dependencies). Honored when the daemon has Gradle facts
     * for the project; null when running on a bare Kotlin directory.
     */
    NEEDS_GRADLE,

    /**
     * Declares the rule requires FIR-backend facilities — methods on
     * [Resolver] that are only implementable against the K2 compiler's
     * frontend IR. Without this declaration, calling a FIR-only
     * `Resolver` method throws `NotImplementedError` at runtime against
     * a non-FIR backend; declaring it moves the failure to deterministic
     * load-time refusal with a diagnostic pointing at
     * `--oracle-backend=fir`.
     */
    NEEDS_FIR,
}

/** Safety tier for a custom rule autofix. */
enum class FixSafety { COSMETIC, IDIOMATIC, SEMANTIC }
