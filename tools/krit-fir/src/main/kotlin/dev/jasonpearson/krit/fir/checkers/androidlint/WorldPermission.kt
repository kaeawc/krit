package dev.jasonpearson.krit.fir.checkers.androidlint

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.checkers.followableInitializer
import dev.jasonpearson.krit.fir.isInTestFile
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.classIdOf
import dev.jasonpearson.krit.fir.support.isOrInheritsFrom
import dev.jasonpearson.krit.fir.support.lightSourceOf
import dev.jasonpearson.krit.fir.support.qualifiedCall
import dev.jasonpearson.krit.fir.support.unwrapArgument
import dev.jasonpearson.krit.fir.support.unwrapSmartCasts
import org.jetbrains.kotlin.contracts.description.LogicOperationKind
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.expressions.FirBooleanOperatorExpression
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirLiteralExpression
import org.jetbrains.kotlin.fir.expressions.FirPropertyAccessExpression
import org.jetbrains.kotlin.fir.expressions.resolvedArgumentMapping
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.resolve.getContainingClassSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirClassSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirNamedFunctionSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirPropertySymbol
import org.jetbrains.kotlin.fir.types.ConeKotlinType
import org.jetbrains.kotlin.fir.types.classId
import org.jetbrains.kotlin.fir.types.isMarkedNullable
import org.jetbrains.kotlin.fir.types.lowerBoundIfFlexible
import org.jetbrains.kotlin.fir.unwrapFakeOverrides
import org.jetbrains.kotlin.name.CallableId
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.name.StandardClassIds
import org.jetbrains.kotlin.types.ConstantValueKind

// Shared by SetWorldReadable and SetWorldWritable, which differ only in the `java.io.File` method they watch.

private const val ARGUMENT_COUNT = 2
private const val PERMISSION_INDEX = 0
private const val OWNER_ONLY_INDEX = 1
private const val MAX_CONSTANT_DEPTH = 6

private val javaIoFile = classIdOf("java.io", "File")
private val booleanNot = CallableId(StandardClassIds.Boolean, Name.identifier("not"))

/**
 * Reports `file.<method>(true, false)`, which grants the permission to everyone. The call is a written call of
 * [method] on an explicit `java.io.File` receiver with two arguments, the permission argument folding to `true` and
 * `ownerOnly` to `false`. As in Android Lint, a File subclass's override of [method] counts. The finding is on the
 * first line of the call, receiver included.
 */
context(context: CheckerContext, reporter: DiagnosticReporter)
internal fun FirRule.reportWorldPermission(call: FirFunctionCall, method: Name, message: String) {
    if (call.calleeReference.name != method || call.argumentList.arguments.size != ARGUMENT_COUNT) return
    if (call.explicitReceiver == null || isInTestFile()) return
    val callee = call.calleeReference.toResolvedCallableSymbol() as? FirNamedFunctionSymbol ?: return
    if (!isFileMethod(callee)) return
    if (argumentAt(call, callee, PERMISSION_INDEX)?.let { booleanValue(it, 0) } != true) return
    if (argumentAt(call, callee, OWNER_ONLY_INDEX)?.let { booleanValue(it, 0) } != false) return
    val source = call.source ?: return
    report(qualifiedCall(source)?.let { lightSourceOf(it, source) } ?: source, message)
}

// `java.io.File`'s method, inherited or overridden by a subclass. A subclass method with the name and two
// non-null Boolean parameters can only be the override; the parameters are checked instead of the `override`
// modifier, which a class file does not keep.
context(context: CheckerContext)
private fun isFileMethod(callee: FirNamedFunctionSymbol): Boolean {
    val declared = callee.unwrapFakeOverrides()
    if (declared.callableId?.classId == javaIoFile) return true
    val parameters = declared.valueParameterSymbols
    if (parameters.size != ARGUMENT_COUNT || parameters.any { !isBoolean(it.resolvedReturnType) }) return false
    val owner = declared.getContainingClassSymbol() as? FirClassSymbol<*> ?: return false
    return isOrInheritsFrom(owner, javaIoFile, context.session, lookupInterfaces = false)
}

private fun isBoolean(type: ConeKotlinType): Boolean {
    val bound = type.lowerBoundIfFlexible()
    return bound.classId == StandardClassIds.Boolean && !bound.isMarkedNullable
}

// The argument bound to the callee's value parameter at [index].
private fun argumentAt(call: FirFunctionCall, callee: FirNamedFunctionSymbol, index: Int): FirExpression? {
    val parameter = callee.valueParameterSymbols.getOrNull(index) ?: return null
    return call.resolvedArgumentMapping?.entries?.firstOrNull { it.value.symbol == parameter }?.key
}

// The Boolean a literal, a `!`, `&&` or `||` of ones, or a `val` of this file (or a `const val`) initialized with one
// stands for.
context(context: CheckerContext)
private fun booleanValue(expression: FirExpression, depth: Int): Boolean? {
    if (depth > MAX_CONSTANT_DEPTH) return null
    return when (val value = unwrapSmartCasts(unwrapArgument(expression))) {
        is FirLiteralExpression -> if (value.kind == ConstantValueKind.Boolean) value.value as? Boolean else null
        is FirPropertyAccessExpression -> {
            val property = value.calleeReference.toResolvedCallableSymbol() as? FirPropertySymbol
            if (property == null || !property.isVal || property.hasDelegate) {
                null
            } else {
                property.followableInitializer()?.let { booleanValue(it, depth + 1) }
            }
        }
        is FirBooleanOperatorExpression -> {
            val left = booleanValue(value.leftOperand, depth + 1) ?: return null
            val right = booleanValue(value.rightOperand, depth + 1) ?: return null
            if (value.kind == LogicOperationKind.AND) left && right else left || right
        }
        is FirFunctionCall -> {
            val operand = value.explicitReceiver
            if (operand != null && value.calleeReference.toResolvedCallableSymbol()?.callableId == booleanNot) {
                booleanValue(operand, depth + 1)?.not()
            } else {
                null
            }
        }
        else -> null
    }
}
