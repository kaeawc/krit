package dev.jasonpearson.krit.fir.checkers.androidlint

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.isInTestFile
import dev.jasonpearson.krit.fir.report
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.declarations.FirAnonymousFunction
import org.jetbrains.kotlin.fir.declarations.FirFile
import org.jetbrains.kotlin.fir.declarations.FirNamedFunction
import org.jetbrains.kotlin.fir.declarations.FirRegularClass
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirQualifiedAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirWhenExpression
import org.jetbrains.kotlin.fir.expressions.unwrapSmartcastExpression
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid
import org.jetbrains.kotlin.name.CallableId
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name

/**
 * Flags an android.util.Log level call (`v`, `d`, `i`, `w`, `e`) that is not
 * guarded by `Log.isLoggable(...)` or `BuildConfig.DEBUG`, on the first line of
 * the call expression (the receiver's line for `Log.d(...)`), with Go's
 * message. Test files are skipped, as in Go. `wtf` and `println` are not level
 * calls here, as in Go.
 *
 * Like the Go rule, a call is guarded when some enclosing `if` between it and
 * the nearest named function, lambda or anonymous function, or named class or
 * object (an anonymous object is passed through, as Go passes `object_literal`)
 * either
 * - has the bare condition `BuildConfig.DEBUG` (parentheses allowed, not
 *   negated, not combined with `&&`/`||`), checked on the `if` whose branch
 *   holds the call and on every earlier `if` of an `else if` chain; or
 * - contains a `Log.isLoggable(...)` call anywhere in the whole `if` (its
 *   condition, either branch, or a nested scope inside them), as Go walks the
 *   whole `if_expression` subtree. A `when` is not a guard.
 *
 * Deliberate differences from Go, each pinned in the golden data:
 * - Recall: the call is identified by resolution, so a fully qualified
 *   `android.util.Log.d(...)` without an import, an import alias or typealias
 *   of Log, a star import, and a statically imported `d(...)` are reported
 *   (LogConditionalRecall, LogConditionalStarImport). Go needs the receiver spelled `Log` and an explicit
 *   `import android.util.Log`. A file that also declares a nested class named
 *   `Log` still calls android.util.Log through its explicit import outside
 *   that class; Go lets the same-file class shadow the import and drops those
 *   findings (LogConditionalNestedLog).
 * - Precision: a local variable, parameter, or project class named `Log`
 *   whose `d(...)` is not android.util.Log is not reported
 *   (LogConditionalLookalike); Go only checks that the file imports
 *   android.util.Log. A guard is recognized by resolution too: a statically
 *   imported or aliased `isLoggable(...)`, and a `DEBUG` field of a class
 *   named BuildConfig read through a static import or an import alias, guard
 *   the call (LogConditionalDivergence); Go needs the text `Log.isLoggable`
 *   and `BuildConfig.DEBUG`.
 */
internal object LogConditional : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "LogConditional"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(LogConditional)
    }

    private const val MESSAGE =
        "Unconditional logging call. Wrap in Log.isLoggable() or BuildConfig.DEBUG for performance."

    private val logClassId = ClassId(FqName("android.util"), Name.identifier("Log"))
    private val levelNames = setOf("v", "d", "i", "w", "e").map(Name::identifier).toSet()
    private val isLoggable = CallableId(logClassId, Name.identifier("isLoggable"))
    private val buildConfigName = Name.identifier("BuildConfig")
    private val debugName = Name.identifier("DEBUG")

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val callableId = expression.calleeReference.toResolvedCallableSymbol()?.callableId ?: return
        if (callableId.classId != logClassId || callableId.callableName !in levelNames) return
        if (isInTestFile()) return
        if (isGuarded(expression)) return
        report(expression.source, MESSAGE)
    }

    // Walks the enclosing elements outward from the call up to the nearest
    // scope boundary Go stops at (function_declaration, class_declaration,
    // object_declaration, lambda_literal, anonymous_function, source_file).
    context(context: CheckerContext)
    private fun isGuarded(call: FirFunctionCall): Boolean {
        val elements = context.containingElements
        for (i in elements.indices.reversed()) {
            when (val element = elements[i]) {
                is FirNamedFunction, is FirAnonymousFunction, is FirRegularClass, is FirFile -> return false
                is FirWhenExpression -> if (isIf(element) && ifGuards(element, elements)) return true
                else -> Unit
            }
        }
        return false
    }

    private fun isIf(expression: FirWhenExpression): Boolean {
        val source = expression.source ?: return false
        return source.kind is KtRealSourceElementKind && source.elementType == KtNodeTypes.IF
    }

    // Go checks each enclosing `if_expression` on its own. K2 may fold an
    // `else if` chain into one `when` with a branch per `if`, so the `if`s
    // that enclose the call are the branches up to the one holding it: each
    // of their conditions is checked for BuildConfig.DEBUG. An isLoggable
    // call anywhere in the outermost `if` covers every `if` of the chain.
    private fun ifGuards(expression: FirWhenExpression, elements: List<FirElement>): Boolean {
        val holding = expression.branches.indexOfFirst { branch -> elements.any { it === branch } }
        val last = if (holding < 0) expression.branches.lastIndex else holding
        for (index in 0..last) {
            // An `else` branch's synthetic condition is not a property read.
            if (isBuildConfigDebug(expression.branches[index].condition)) return true
        }
        return containsIsLoggable(expression)
    }

    // The bare condition `BuildConfig.DEBUG`: a read of a field or property
    // named DEBUG declared in a class named BuildConfig (the app's generated
    // class, in any package), or read through a receiver spelled BuildConfig.
    private fun isBuildConfigDebug(condition: FirExpression): Boolean {
        val access = condition.unwrapSmartcastExpression() as? FirQualifiedAccessExpression ?: return false
        if (access is FirFunctionCall) return false
        val symbol = access.calleeReference.toResolvedCallableSymbol() ?: return false
        if (symbol.name != debugName) return false
        if (symbol.callableId?.classId?.shortClassName == buildConfigName) return true
        // A local variable has no callable id, so read the receiver's name
        // from its symbol.
        val receiver = access.explicitReceiver?.unwrapSmartcastExpression() as? FirQualifiedAccessExpression
            ?: return false
        return receiver !is FirFunctionCall &&
            receiver.calleeReference.toResolvedCallableSymbol()?.name == buildConfigName
    }

    private fun containsIsLoggable(root: FirElement): Boolean {
        var found = false
        root.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                if (found) return
                if (element is FirFunctionCall &&
                    element.calleeReference.toResolvedCallableSymbol()?.callableId == isLoggable
                ) {
                    found = true
                    return
                }
                element.acceptChildren(this)
            }
        })
        return found
    }
}
