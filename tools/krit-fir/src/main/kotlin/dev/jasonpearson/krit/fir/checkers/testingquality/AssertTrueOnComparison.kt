package dev.jasonpearson.krit.fir.checkers.testingquality

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.classIdOf
import dev.jasonpearson.krit.fir.support.isNullLiteral
import dev.jasonpearson.krit.fir.support.unwrapArgument
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.expressions.FirEqualityOperatorCall
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirLiteralExpression
import org.jetbrains.kotlin.fir.expressions.FirOperation
import org.jetbrains.kotlin.fir.expressions.arguments
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.name.CallableId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.types.ConstantValueKind

/**
 * Flags `assertTrue(a == b)` (and `a != null`) that should use `assertEquals`, `assertNull` or `assertNotNull`.
 *
 * Mirrors the Go rule: a one-argument `assertTrue` whose argument is a `==` or `!=` comparison. A `null` operand
 * reports the dedicated null assertion (for `==` and `!=`); otherwise only `==` is reported, and an operand that
 * is a `true` / `false` literal is a null-safe boolean idiom, not value equality. A call with a message argument
 * is not reported.
 *
 * Deliberate differences from Go, each pinned in the golden data:
 * - Precision: the call must resolve to kotlin.test, JUnit 4, JUnit 3 or JUnit Jupiter. Go reports a
 *   receiverless `assertTrue` unless the file declares one or imports it from another package, so a helper
 *   that reuses the name in another file of the same package is not reported here.
 * - Recall: the declared name is matched, so an import alias of `assertTrue` is reported, and so is a comparison
 *   in redundant parentheses.
 */
internal object AssertTrueOnComparison : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "AssertTrueOnComparison"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(AssertTrueOnComparison)
    }

    private val assertionOwners = setOf(
        classIdOf("org.junit", "Assert"),
        classIdOf("junit.framework", "Assert"),
        classIdOf("junit.framework", "TestCase"),
        classIdOf("org.junit.jupiter.api", "Assertions"),
    )
    private val kotlinTestPackage = FqName("kotlin.test")

    private const val EQUALS_MESSAGE = "Use `assertEquals` instead of `assertTrue` for equality comparisons."
    private const val NULL_MESSAGE = "Use `assertNull` instead of `assertTrue` for null comparisons."
    private const val NOT_NULL_MESSAGE = "Use `assertNotNull` instead of `assertTrue` for null comparisons."

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val source = expression.source ?: return
        if (source.kind !is KtRealSourceElementKind) return
        val callee = expression.calleeReference.toResolvedCallableSymbol() ?: return
        val callableId = callee.callableId ?: return
        if (!isLibraryAssertTrue(callableId)) return
        val argument = expression.arguments.singleOrNull()?.let(::unwrapArgument) as? FirEqualityOperatorCall ?: return
        val message = messageFor(argument) ?: return
        report(expression.calleeReference.source ?: source, message)
    }

    private fun isLibraryAssertTrue(id: CallableId): Boolean {
        if (id.callableName.asString() != "assertTrue") return false
        val owner = id.classId ?: return id.packageName == kotlinTestPackage
        return owner in assertionOwners
    }

    private fun messageFor(comparison: FirEqualityOperatorCall): String? {
        val isEquals = comparison.operation == FirOperation.EQ
        if (!isEquals && comparison.operation != FirOperation.NOT_EQ) return null
        val operands = comparison.arguments.map(::unwrapArgument).takeIf { it.size == 2 } ?: return null
        if (operands.any(::isNullLiteral)) return if (isEquals) NULL_MESSAGE else NOT_NULL_MESSAGE
        if (!isEquals || operands.any(::isBooleanLiteral)) return null
        return EQUALS_MESSAGE
    }

    private fun isBooleanLiteral(expression: FirExpression): Boolean =
        expression is FirLiteralExpression && expression.kind == ConstantValueKind.Boolean
}
