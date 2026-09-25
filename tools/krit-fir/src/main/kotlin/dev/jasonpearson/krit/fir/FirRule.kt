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
}

/** [FirRule.isTestFile] for the file [context] is checking. */
context(context: CheckerContext)
fun FirRule.isInTestFile(): Boolean = isTestFile(context.containingFile?.path)

/** [FirRule.scanPath] for the file [context] is checking. */
context(context: CheckerContext)
fun FirRule.containingScanPath(): String? = scanPath(context.containingFile?.path)

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
