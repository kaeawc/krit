package dev.jasonpearson.krit.fir

import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.diagnostics.reportOn
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.DeclarationCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.type.TypeCheckers
import java.io.File
import java.util.concurrent.ConcurrentHashMap

/** Built-in FIR rule contract; separate from the external krit-rule-api KritRule. */
interface FirRule {
    val ruleId: String
    val expressionCheckers: ExpressionCheckers? get() = null
    val declarationCheckers: DeclarationCheckers? get() = null
    val typeCheckers: TypeCheckers? get() = null

    /**
     * Non-null puts the rule on the full-compile tier, with the kind of read
     * that put it there and the reason. Build it with [compilationFiles] or
     * [otherDeclarationBodies].
     *
     * The incremental runner (#865) will compile only the dirty files and take
     * the rest of the module from a binary snapshot, and will check tiered
     * rules only on a full compile. Nothing consumes the tier yet.
     *
     * Set it when the rule iterates the compilation's files
     * (`session.firProvider.getFirFiles*`), or reads another declaration's body
     * or initializer (`resolvedInitializer`, `symbol.fir.body`,
     * `symbol.fir.initializer`, `delegate`, `defaultValue`, `symbol.source`,
     * `getContainingFile`) on a symbol that may be non-local. A declaration
     * from a snapshot has none of these, and no signature-based dirty set
     * tracks them. Leave null for rules that look only at the file under check
     * and resolve other symbols through `symbolProvider` (class and signature
     * metadata, which a snapshot keeps).
     *
     * Prefer narrowing the rule to tiering it: follow an initializer, body, or
     * delegate only for a declaration of the file under check, through the
     * helpers in `checkers/SameFileReads.kt` (`initializerInCheckedFile`,
     * `bodyInCheckedFile`, `followableInitializer` for a `const`), and treat
     * every other declaration as a class file shows it. Such a rule needs no
     * tier and runs on incremental slices.
     *
     * `FirRuleWholeCompilationGateTest` keeps this in step with the rule's
     * source, and the split differential tests check it against slice compiles.
     * Known gap neither can see: SOURCE-retention annotations are absent from
     * class files, so a rule reading them through `resolvedAnnotationClassIds`
     * can differ on a slice (IntoMapMissingKey; findings section 8.2).
     */
    val wholeCompilationTier: WholeCompilationTier? get() = null

    /** True when [wholeCompilationTier] is set. */
    val requiresWholeCompilation: Boolean get() = wholeCompilationTier != null
    fun config(): Map<String, Any?> = FirRuleContext.current()?.ruleConfigs?.get(ruleId).orEmpty()

    /**
     * True when krit classifies the source file at [path] as a test file:
     * the check request's `testFiles`, computed on the Go side with the same
     * `scanner.IsTestFile` classification (configured test paths included)
     * the Go rules use. Never guess from the path here. False outside a
     * check request (oracle compiles, direct compiler/test-harness runs).
     */
    fun isTestFile(path: String?): Boolean = FirRuleContext.current()?.isTestFile(path) ?: false

    /**
     * The path of the source file at [path] (the compiler's spelling) as the
     * Go rules see it: the scan's own spelling, usually relative to the
     * directory krit ran in (`samples/proj/src/X.kt` for `krit samples/proj`),
     * sent by Go in the check request's `scanPaths`. Apply a Go rule's path
     * heuristic (`/samples/`, `.kts`, ...) to this string, never to the
     * compiler's absolute path, which also holds the directories above the
     * scan root. Falls back to the path as requested, then to [path] itself
     * outside a check request (oracle compiles, direct compiler/test-harness
     * runs).
     */
    fun scanPath(path: String?): String? = FirRuleContext.current()?.scanPath(path) ?: path

    /**
     * The minSdk of the Android module that owns the source file at [path]
     * (the compiler's spelling), or 0 when it is unknown. Go resolves it with
     * the lookup its own rules use (the nearest `build.gradle(.kts)` or
     * `AndroidManifest.xml` above the file that declares an SDK level) and
     * sends it in the check request's `sdkLevels`. Never read build files
     * here. 0 for a file under no Android module and outside a check request
     * (oracle compiles, direct compiler/test-harness runs).
     */
    fun minSdkFor(path: String?): Int = FirRuleContext.current()?.sdkLevels(path)?.minSdk ?: 0

    /** The targetSdk counterpart of [minSdkFor]; 0 when unknown. */
    fun targetSdkFor(path: String?): Int = FirRuleContext.current()?.sdkLevels(path)?.targetSdk ?: 0
}

/** A source file's resolved Android SDK levels; 0 means unknown, as in Go. */
data class SdkLevels(val minSdk: Int = 0, val targetSdk: Int = 0)

/** [FirRule.isTestFile] for the file [context] is checking. */
context(context: CheckerContext)
fun FirRule.isInTestFile(): Boolean = isTestFile(context.containingFile?.path)

/** [FirRule.scanPath] for the file [context] is checking. */
context(context: CheckerContext)
fun FirRule.containingScanPath(): String? = scanPath(context.containingFile?.path)

/** [FirRule.minSdkFor] for the file [context] is checking. */
context(context: CheckerContext)
fun FirRule.containingMinSdk(): Int = minSdkFor(context.containingFile?.path)

/** [FirRule.targetSdkFor] for the file [context] is checking. */
context(context: CheckerContext)
fun FirRule.containingTargetSdk(): Int = targetSdkFor(context.containingFile?.path)

context(context: CheckerContext, reporter: DiagnosticReporter)
fun FirRule.report(source: KtSourceElement?, message: String) {
    if (source != null) reporter.reportOn(source, KritDiagnostics.KRIT_RULE, ruleId, message)
}

/** Null context means a direct compiler/test-harness invocation: enable all rules. */
data class FirRuleCompileContext(
    val enabledRuleIds: Set<String> = emptySet(),
    val ruleConfigs: Map<String, Map<String, Any?>> = emptyMap(),
    val noneEnabled: Boolean = false,
    /** Requested files krit classifies as test files, spelled as in the request. */
    val testFiles: Set<String> = emptySet(),
    /** The requested files, spelled as in the request. */
    val files: Set<String> = emptySet(),
    /** Requested file -> the scan's own spelling of it, where the two differ. */
    val scanPaths: Map<String, String> = emptyMap(),
    /** Requested file -> its resolved SDK levels, for the files with a known level. */
    val sdkLevels: Map<String, SdkLevels> = emptyMap(),
) {
    // The compiler may spell a file differently from the request (absolute,
    // symlinks resolved), so fall back to canonical paths, memoized per path.
    private val canonicalTestFiles: Set<String> by lazy { testFiles.mapTo(HashSet()) { canonical(it) } }
    private val verdicts = ConcurrentHashMap<String, Boolean>()
    private val canonicalFiles: Map<String, String> by lazy { files.associateBy { canonical(it) } }
    private val requestPaths = ConcurrentHashMap<String, String>()

    fun isTestFile(path: String?): Boolean {
        if (path == null || testFiles.isEmpty()) return false
        if (path in testFiles) return true
        return verdicts.computeIfAbsent(path) { canonical(it) in canonicalTestFiles }
    }

    /**
     * The requested file the compiler's [path] names, spelled as in the
     * request, matched as spelled and then in canonical form like
     * [isTestFile]; null when [path] is not a requested file.
     */
    fun requestPath(path: String?): String? {
        if (path == null || files.isEmpty()) return null
        if (path in files) return path
        return requestPaths.computeIfAbsent(path) { canonicalFiles[canonical(it)] ?: NOT_REQUESTED }
            .takeUnless { it == NOT_REQUESTED }
    }

    /** See [FirRule.scanPath]; null when [path] is not a requested file. */
    fun scanPath(path: String?): String? = requestPath(path)?.let { scanPaths[it] ?: it }

    /** See [FirRule.minSdkFor]; null when no level is known for [path]. */
    fun sdkLevels(path: String?): SdkLevels? =
        if (sdkLevels.isEmpty()) null else requestPath(path)?.let { sdkLevels[it] }

    private fun canonical(path: String): String = try {
        File(path).canonicalPath
    } catch (_: java.io.IOException) {
        File(path).absolutePath
    }

    private companion object {
        // ConcurrentHashMap holds no nulls; marks a path that is not
        // requested (no path contains a NUL character).
        const val NOT_REQUESTED = "\u0000"
    }
}

object FirRuleContext {
    private val active = ThreadLocal<FirRuleCompileContext?>()
    fun begin(context: FirRuleCompileContext) { active.set(context) }
    fun current(): FirRuleCompileContext? = active.get()
    fun end() { active.remove() }
}

/** Why a rule needs a full compile. */
enum class WholeCompilationKind {
    /** The rule iterates the compilation's files (`firProvider`): it cannot run on a slice at all. */
    COMPILATION_FILES,

    /** The rule reads other declarations' bodies or initializers: a slice sees them only as binaries. */
    OTHER_DECLARATION_BODIES,
}

data class WholeCompilationTier(val kind: WholeCompilationKind, val reason: String)

fun compilationFiles(reason: String) = WholeCompilationTier(WholeCompilationKind.COMPILATION_FILES, reason)

fun otherDeclarationBodies(reason: String) = WholeCompilationTier(WholeCompilationKind.OTHER_DECLARATION_BODIES, reason)
