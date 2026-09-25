package dev.jasonpearson.krit.fir

import com.intellij.openapi.diagnostic.ControlFlowException
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.analysis.cfa.AbstractFirPropertyInitializationChecker
import org.jetbrains.kotlin.fir.analysis.cfa.util.VariableInitializationInfoData
import org.jetbrains.kotlin.fir.analysis.checkers.cfa.FirControlFlowChecker
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.FirDeclarationChecker
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirExpressionChecker
import org.jetbrains.kotlin.fir.analysis.checkers.type.FirTypeChecker
import org.jetbrains.kotlin.fir.declarations.FirDeclaration
import org.jetbrains.kotlin.fir.declarations.FirFile
import org.jetbrains.kotlin.fir.expressions.FirStatement
import org.jetbrains.kotlin.fir.resolve.dfa.cfg.ControlFlowGraph
import org.jetbrains.kotlin.fir.types.FirTypeRef
import java.util.concurrent.CancellationException

/**
 * Checker exceptions recorded during one check compile, keyed by rule id, then
 * by the compiler's spelling of the file being checked ("" when the checker
 * context had no file). The first message per (rule, file) wins.
 *
 * Thread-safe: the recorder is captured when the checker extension is built,
 * so it does not depend on which thread K2 runs checkers on.
 */
class FirRuleErrorRecorder {
    private val errors = LinkedHashMap<String, LinkedHashMap<String, String>>()

    @Synchronized
    fun record(ruleId: String, path: String?, message: String) {
        errors.getOrPut(ruleId) { linkedMapOf() }.putIfAbsent(path.orEmpty(), message)
    }

    @Synchronized
    fun snapshot(): Map<String, Map<String, String>> = errors.mapValues { LinkedHashMap(it.value) }
}

/**
 * The recorder for the check compile running on this thread. Only a check
 * compile opens one; without it (oracle compiles, direct compiler or
 * test-harness runs) rule checkers are merged unwrapped and an exception
 * surfaces as a compiler crash, as before.
 */
object FirRuleErrors {
    private val active = ThreadLocal<FirRuleErrorRecorder?>()
    fun begin(recorder: FirRuleErrorRecorder) { active.set(recorder) }
    fun current(): FirRuleErrorRecorder? = active.get()
    fun end() { active.remove() }
}

/**
 * Runs [body] for [ruleId]'s checker. Any exception other than compiler
 * control flow (cancellation) or a JVM-level failure is recorded against
 * (rule, file) and swallowed, so the compile and every other rule continue.
 */
internal inline fun isolateRule(
    recorder: FirRuleErrorRecorder, ruleId: String, path: () -> String?, body: () -> Unit,
) {
    try {
        body()
    } catch (t: Throwable) {
        if (!isIsolatable(t)) throw t
        recorder.record(ruleId, runCatching(path).getOrNull(), summarize(t))
    }
}

// ProcessCanceledException (a CancellationException and a ControlFlowException)
// is how the IntelliJ/K2 infrastructure unwinds a cancelled analysis; it must
// propagate. OutOfMemoryError and other VirtualMachineErrors leave the JVM in a
// state no rule-level fallback can trust. A StackOverflowError, by contrast,
// is local to the checker's own frames and has already unwound.
internal fun isIsolatable(t: Throwable): Boolean = when (t) {
    is ControlFlowException, is CancellationException, is InterruptedException -> false
    is StackOverflowError -> true
    is VirtualMachineError -> false
    else -> true
}

internal fun summarize(t: Throwable): String {
    val frame = t.stackTrace.firstOrNull { it.className.startsWith(RULE_PACKAGE) } ?: t.stackTrace.firstOrNull()
    val where = frame?.let { " at ${it.className}.${it.methodName}(${it.fileName}:${it.lineNumber})" }.orEmpty()
    val message = t.message?.lineSequence()?.firstOrNull()?.take(MAX_MESSAGE)
    return "krit-fir: checker threw ${t.javaClass.name}" + (message?.let { ": $it" } ?: "") + where
}

private const val RULE_PACKAGE = "dev.jasonpearson.krit.fir.checkers."
private const val MAX_MESSAGE = 300

context(context: CheckerContext)
private fun checkedPath(element: FirElement?): String? =
    (element as? FirFile)?.sourceFile?.path ?: context.containingFile?.path

internal class IsolatedExpressionChecker<E : FirStatement>(
    val ruleId: String, val delegate: FirExpressionChecker<E>, private val recorder: FirRuleErrorRecorder,
) : FirExpressionChecker<E>(delegate.mppKind) {
    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: E) =
        isolateRule(recorder, ruleId, { checkedPath(expression) }) { delegate.check(expression) }
}

internal class IsolatedDeclarationChecker<D : FirDeclaration>(
    val ruleId: String, val delegate: FirDeclarationChecker<D>, private val recorder: FirRuleErrorRecorder,
) : FirDeclarationChecker<D>(delegate.mppKind) {
    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(declaration: D) =
        isolateRule(recorder, ruleId, { checkedPath(declaration) }) { delegate.check(declaration) }
}

internal class IsolatedTypeChecker<T : FirTypeRef>(
    val ruleId: String, val delegate: FirTypeChecker<T>, private val recorder: FirRuleErrorRecorder,
) : FirTypeChecker<T>(delegate.mppKind) {
    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(typeRef: T) =
        isolateRule(recorder, ruleId, { checkedPath(typeRef) }) { delegate.check(typeRef) }
}

internal class IsolatedControlFlowChecker(
    val ruleId: String, val delegate: FirControlFlowChecker, private val recorder: FirRuleErrorRecorder,
) : FirControlFlowChecker(delegate.mppKind) {
    context(reporter: DiagnosticReporter, context: CheckerContext)
    override fun analyze(graph: ControlFlowGraph) =
        isolateRule(recorder, ruleId, { checkedPath(null) }) { delegate.analyze(graph) }
}

internal class IsolatedPropertyInitializationChecker(
    val ruleId: String, val delegate: AbstractFirPropertyInitializationChecker, private val recorder: FirRuleErrorRecorder,
) : AbstractFirPropertyInitializationChecker(delegate.mppKind) {
    context(reporter: DiagnosticReporter, context: CheckerContext)
    override fun analyze(data: VariableInitializationInfoData) =
        isolateRule(recorder, ruleId, { checkedPath(null) }) { delegate.analyze(data) }
}

/** The checker a rule contributed, looking through an isolation wrapper. */
internal fun unwrapChecker(checker: Any): Any = when (checker) {
    is IsolatedExpressionChecker<*> -> checker.delegate
    is IsolatedDeclarationChecker<*> -> checker.delegate
    is IsolatedTypeChecker<*> -> checker.delegate
    is IsolatedControlFlowChecker -> checker.delegate
    is IsolatedPropertyInitializationChecker -> checker.delegate
    else -> checker
}
