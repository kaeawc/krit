package dev.jasonpearson.krit.fir

import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.diagnostics.reportOn
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.DeclarationCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.type.TypeCheckers

/** Built-in FIR rule contract; separate from the external krit-rule-api KritRule. */
interface FirRule {
    val ruleId: String
    val expressionCheckers: ExpressionCheckers? get() = null
    val declarationCheckers: DeclarationCheckers? get() = null
    val typeCheckers: TypeCheckers? get() = null
    fun config(): Map<String, Any?> = FirRuleContext.current()?.ruleConfigs?.get(ruleId).orEmpty()
}

context(context: CheckerContext, reporter: DiagnosticReporter)
fun FirRule.report(source: KtSourceElement?, message: String) {
    if (source != null) reporter.reportOn(source, KritDiagnostics.KRIT_RULE, ruleId, message)
}

/** Null context means a direct compiler/test-harness invocation: enable all rules. */
data class FirRuleCompileContext(
    val enabledRuleIds: Set<String> = emptySet(),
    val ruleConfigs: Map<String, Map<String, Any?>> = emptyMap(),
    val noneEnabled: Boolean = false,
)

object FirRuleContext {
    private val active = ThreadLocal<FirRuleCompileContext?>()
    fun begin(context: FirRuleCompileContext) { active.set(context) }
    fun current(): FirRuleCompileContext? = active.get()
    fun end() { active.remove() }
}
