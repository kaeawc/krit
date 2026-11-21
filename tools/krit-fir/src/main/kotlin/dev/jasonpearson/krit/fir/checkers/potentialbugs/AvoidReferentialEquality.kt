package dev.jasonpearson.krit.fir.checkers.potentialbugs

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.checkers.bodyInCheckedFile
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.expandedClassType
import dev.jasonpearson.krit.fir.support.isNullLiteral
import dev.jasonpearson.krit.fir.support.pathTo
import dev.jasonpearson.krit.fir.support.trimGoSpace
import dev.jasonpearson.krit.fir.support.unwrapArgument
import dev.jasonpearson.krit.fir.support.unwrapSmartCasts
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.descriptors.ClassKind
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirEqualityOperatorCallChecker
import org.jetbrains.kotlin.fir.expressions.FirBooleanOperatorExpression
import org.jetbrains.kotlin.fir.expressions.FirEqualityOperatorCall
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirLiteralExpression
import org.jetbrains.kotlin.fir.expressions.FirOperation
import org.jetbrains.kotlin.fir.expressions.FirPropertyAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirReturnExpression
import org.jetbrains.kotlin.fir.expressions.FirSafeCallExpression
import org.jetbrains.kotlin.fir.expressions.FirThisReceiverExpression
import org.jetbrains.kotlin.fir.expressions.FirWhenExpression
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.resolve.toClassSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirNamedFunctionSymbol
import org.jetbrains.kotlin.fir.types.ConeErrorType
import org.jetbrains.kotlin.fir.types.resolvedType
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid
import org.jetbrains.kotlin.text

internal object AvoidReferentialEquality : FirEqualityOperatorCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "AvoidReferentialEquality"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val equalityOperatorCallCheckers = setOf(AvoidReferentialEquality)
    }

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirEqualityOperatorCall) {
        if (expression.operation != FirOperation.IDENTITY && expression.operation != FirOperation.NOT_IDENTITY) return
        val source = expression.source ?: return
        if (source.kind != KtRealSourceElementKind || !shouldReport(expression)) return

        report(source, "Referential equality (===, !==) should be avoided. Use structural equality (==, !=) instead.")
    }

    context(context: CheckerContext)
    private fun shouldReport(expression: FirEqualityOperatorCall): Boolean {
        val operands = expression.argumentList.arguments
        if (operands.size != 2 || hasExcludedOperand(operands) || callTakesOtherOperand(operands)) return false

        if (!hasConfiguredType(operands)) return false
        return !hasContextExemption(expression, operands)
    }

    private fun hasExcludedOperand(operands: List<FirExpression>): Boolean {
        return operands.any { isNullLiteral(unwrapSmartCasts(it)) } ||
            operands.any { unwrapSmartCasts(it) is FirThisReceiverExpression }
    }

    private fun callTakesOtherOperand(operands: List<FirExpression>): Boolean {
        return ReferentialEqualitySyntax.callTakesOther(operands[0], operands[1]) ||
            ReferentialEqualitySyntax.callTakesOther(operands[1], operands[0])
    }

    context(context: CheckerContext)
    private fun hasConfiguredType(operands: List<FirExpression>): Boolean {
        val patterns = ReferentialEqualityPatterns.configuredPatterns(config())
        val types = operands.mapNotNull { it.resolvedType.expandedClassType() }.filterNot { it is ConeErrorType }
        val matchesPattern = types.any { type -> patterns.any { it.matches(type.lookupTag.classId.asFqNameString()) } }
        if (!matchesPattern) return false
        return types.none {
            it.lookupTag.toClassSymbol(context.session)?.classKind in setOf(ClassKind.ENUM_CLASS, ClassKind.OBJECT)
        }
    }

    context(context: CheckerContext)
    private fun hasContextExemption(expression: FirEqualityOperatorCall, operands: List<FirExpression>): Boolean {
        val path = pathTo(expression).orEmpty()
        val parent = path.getOrNull(path.lastIndex - 1)
        val equalsGuard = parent is FirBooleanOperatorExpression &&
            ReferentialEqualitySyntax.containsCall(parent, ReferentialEqualitySyntax.equalsFamily)
        return equalsGuard || ReferentialEqualityExemptions.comparatorFastPath(expression, path) ||
            ReferentialEqualityExemptions.cleanupGuard(expression, path, operands)
    }
}

private object ReferentialEqualityPatterns {
    private val patternCache = ThreadLocal<Pair<Map<String, Any?>, List<Regex>>>()

    /** Matches Go's scalar/list and value-with-reason options, caching the current config per checker thread. */
    fun configuredPatterns(options: Map<String, Any?>): List<Regex> {
        patternCache.get()?.takeIf { it.first == options }?.let { return it.second }
        val raw = if (!options.containsKey("forbiddenTypePatterns")) listOf("kotlin.String") else {
            when (val configured = options["forbiddenTypePatterns"]) {
                is String -> listOf(configured)
                is List<*> -> configured.map { item ->
                    if (item is Map<*, *> && item.containsKey("value")) item["value"]?.toString() ?: "<nil>"
                    else if (item == null) "<nil>" else item.toString()
                }
                else -> listOf("kotlin.String")
            }
        }
        return raw.mapNotNull(::compilePattern).also { patternCache.set(options to it) }
    }

    /** Compiles an anchored FQN glob rather than interpreting option text as a regular expression. */
    private fun compilePattern(raw: String): Regex? {
        val pattern = trimGoSpace(raw)
        if (pattern.isEmpty()) return null
        val regex = pattern.map { character ->
            when (character) {
                '*' -> ".*"
                '?' -> "."
                else -> Regex.escape(character.toString())
            }
        }.joinToString("")
        return Regex(regex)
    }
}

private object ReferentialEqualitySyntax {
    val equalsFamily = setOf("equals", "hasSameContent", "contentEquals", "contentDeepEquals", "sameContentAs")

    fun callTakesOther(operand: FirExpression, other: FirExpression): Boolean {
        val unwrapped = unwrapSmartCasts(operand, throughNotNull = true)
        if (unwrapped !is FirFunctionCall && unwrapped !is FirSafeCallExpression) return false
        var found = false
        unwrapped.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                if (!found) element.acceptChildren(this)
            }

            override fun visitFunctionCall(functionCall: FirFunctionCall) {
                val hasMatchingArgument = functionCall.argumentList.arguments.any { argument ->
                    argumentMatchesOther(unwrapArgument(argument), other)
                }
                if (hasMatchingArgument) found = true
                else visitElement(functionCall)
            }
        })
        return found
    }

    fun argumentMatchesOther(argument: FirExpression, other: FirExpression): Boolean {
        if (sameReference(argument, other)) return true
        val argumentText = argument.source?.text?.toString()?.let(::trimGoSpace)
        val otherText = other.source?.text?.toString()?.let(::trimGoSpace)
        return !argumentText.isNullOrEmpty() && argumentText == otherText
    }

    fun sameReference(left: FirExpression, right: FirExpression): Boolean {
        val leftOperand = unwrapSmartCasts(left)
        val rightOperand = unwrapSmartCasts(right)
        if (leftOperand is FirThisReceiverExpression && rightOperand is FirThisReceiverExpression) {
            return leftOperand.calleeReference.boundSymbol == rightOperand.calleeReference.boundSymbol
        }
        val leftAccess = leftOperand as? FirPropertyAccessExpression ?: return false
        val rightAccess = rightOperand as? FirPropertyAccessExpression ?: return false
        val symbol = leftAccess.calleeReference.toResolvedCallableSymbol() ?: return false
        if (symbol != rightAccess.calleeReference.toResolvedCallableSymbol()) return false
        val leftReceiver = leftAccess.explicitReceiver
        val rightReceiver = rightAccess.explicitReceiver
        if (leftReceiver == null || rightReceiver == null) return leftReceiver == null && rightReceiver == null
        return sameReference(leftReceiver, rightReceiver)
    }

    fun containsCall(
        element: FirElement,
        names: Set<String>,
        receiver: ((FirExpression) -> Boolean)? = null,
    ): Boolean {
        var found = false
        element.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                if (!found) element.acceptChildren(this)
            }

            override fun visitFunctionCall(functionCall: FirFunctionCall) {
                if (matchesCall(functionCall, names, receiver)) found = true
                else visitElement(functionCall)
            }
        })
        return found
    }

    private fun matchesCall(
        functionCall: FirFunctionCall,
        names: Set<String>,
        receiver: ((FirExpression) -> Boolean)?,
    ): Boolean {
        val name = functionCall.calleeReference.toResolvedCallableSymbol()?.name?.asString()
        if (name !in names) return false
        val target = functionCall.explicitReceiver
        return receiver == null || (target != null && receiver(target))
    }

    fun containsElement(root: FirElement, target: FirElement): Boolean {
        var found = false
        root.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                if (element === target) found = true
                else if (!found) element.acceptChildren(this)
            }
        })
        return found
    }
}

private object ReferentialEqualityExemptions {
    context(context: CheckerContext)
    fun comparatorFastPath(expression: FirEqualityOperatorCall, path: List<FirElement>): Boolean {
        val first = context.containingDeclarations
            .filterIsInstance<FirNamedFunctionSymbol>()
            .lastOrNull()
            ?.takeIf { it.name.asString() in setOf("compare", "compareTo") }
            ?.bodyInCheckedFile()
            ?.statements
            ?.firstOrNull() as? FirWhenExpression ?: return false
        if (first !in path || !firstBranchContains(first, expression)) return false
        return containsZeroReturn(first)
    }

    private fun firstBranchContains(expression: FirWhenExpression, target: FirElement): Boolean {
        val branch = expression.branches.firstOrNull() ?: return false
        return ReferentialEqualitySyntax.containsElement(branch.condition, target)
    }

    private fun containsZeroReturn(expression: FirWhenExpression): Boolean {
        var returnsZero = false
        expression.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                if (!returnsZero) element.acceptChildren(this)
            }

            override fun visitReturnExpression(returnExpression: FirReturnExpression) {
                val value = (returnExpression.result as? FirLiteralExpression)?.value as? Number
                if (value?.toLong() == 0L) returnsZero = true
                else visitElement(returnExpression)
            }
        })
        return returnsZero
    }

    context(context: CheckerContext)
    fun cleanupGuard(
        expression: FirEqualityOperatorCall,
        path: List<FirElement>,
        operands: List<FirExpression>,
    ): Boolean {
        if (expression.operation != FirOperation.NOT_IDENTITY) return false
        val branch = path.filterIsInstance<FirWhenExpression>().lastOrNull()?.branches?.firstOrNull() ?: return false
        if (!ReferentialEqualitySyntax.containsElement(branch.condition, expression)) return false
        return ReferentialEqualitySyntax.containsCall(branch.result, setOf("close", "recycle")) { target ->
            operands.any { ReferentialEqualitySyntax.sameReference(target, it) }
        }
    }
}
