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
}

/** [FirRule.isTestFile] for the file [context] is checking. */
context(context: CheckerContext)
fun FirRule.isInTestFile(): Boolean = isTestFile(context.containingFile?.path)

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
) {
    // The compiler may spell a file differently from the request (absolute,
    // symlinks resolved), so fall back to canonical paths, memoized per path.
    private val canonicalTestFiles: Set<String> by lazy { testFiles.mapTo(HashSet()) { canonical(it) } }
    private val verdicts = ConcurrentHashMap<String, Boolean>()

    fun isTestFile(path: String?): Boolean {
        if (path == null || testFiles.isEmpty()) return false
        if (path in testFiles) return true
        return verdicts.computeIfAbsent(path) { canonical(it) in canonicalTestFiles }
    }

    private fun canonical(path: String): String = try {
        File(path).canonicalPath
    } catch (_: java.io.IOException) {
        File(path).absolutePath
    }
}

object FirRuleContext {
    private val active = ThreadLocal<FirRuleCompileContext?>()
    fun begin(context: FirRuleCompileContext) { active.set(context) }
    fun current(): FirRuleCompileContext? = active.get()
    fun end() { active.remove() }
}
