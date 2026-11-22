package dev.jasonpearson.krit.fir.checkers.androidlint

import dev.jasonpearson.krit.fir.checkers.followableInitializer
import dev.jasonpearson.krit.fir.support.classIdOf
import dev.jasonpearson.krit.fir.support.unwrapArgument
import org.jetbrains.kotlin.descriptors.Modality
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.declarations.utils.isConst
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirLiteralExpression
import org.jetbrains.kotlin.fir.expressions.FirPropertyAccessExpression
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirPropertySymbol
import org.jetbrains.kotlin.name.StandardClassIds
import org.jetbrains.kotlin.types.ConstantValueKind

internal object PendingIntentFlagValue {
    private val pendingIntent = classIdOf("android.app", "PendingIntent")
    private val platformFlags = mapOf(
        "FLAG_ONE_SHOT" to (1 shl 30),
        "FLAG_NO_CREATE" to (1 shl 29),
        "FLAG_CANCEL_CURRENT" to (1 shl 28),
        "FLAG_UPDATE_CURRENT" to (1 shl 27),
        "FLAG_IMMUTABLE" to (1 shl 26),
        "FLAG_MUTABLE" to (1 shl 25),
        "FLAG_ALLOW_UNSAFE_IMPLICIT_INTENT" to (1 shl 24),
    )

    context(context: CheckerContext)
    fun read(argument: FirExpression, depth: Int = 0): Int? {
        if (depth > 16) return null
        val unwrapped = unwrapArgument(argument)
        return when (unwrapped) {
            is FirLiteralExpression -> literal(unwrapped)
            is FirPropertyAccessExpression -> constant(unwrapped, depth)
            is FirFunctionCall -> readOperator(unwrapped, depth)
            else -> null
        }
    }

    private fun literal(value: FirLiteralExpression): Int? = when (value.kind) {
        ConstantValueKind.Int, ConstantValueKind.IntegerLiteral -> (value.value as? Number)?.toInt()
        else -> null
    }

    context(context: CheckerContext)
    private fun constant(access: FirPropertyAccessExpression, constantDepth: Int): Int? {
        val symbol = access.calleeReference.toResolvedCallableSymbol() ?: return null
        val callableId = symbol.callableId
        if (callableId?.classId == pendingIntent) return platformFlags[callableId.callableName.asString()]
        val property = symbol as? FirPropertySymbol ?: return null
        if (!stable(property)) return null
        return property.followableInitializer()?.let { read(it, constantDepth + 1) }
    }

    private fun stable(property: FirPropertySymbol): Boolean {
        if (property.isConst) return true
        if (!property.isVal || property.hasDelegate || property.getterSymbol?.isDefault == false) return false
        if (property.resolvedStatus.isOverride) return false
        return property.isLocal || property.resolvedStatus.modality == Modality.FINAL
    }

    context(context: CheckerContext)
    private fun readOperator(call: FirFunctionCall, operatorDepth: Int): Int? {
        val callableId = call.calleeReference.toResolvedCallableSymbol()?.callableId ?: return null
        if (callableId.classId != StandardClassIds.Int) return null
        val receiver = call.explicitReceiver ?: return null
        val left = read(receiver, operatorDepth + 1) ?: return null
        val name = callableId.callableName.asString()
        val arguments = call.argumentList.arguments
        return when (arguments.size) {
            0 -> PendingIntentFlagOperators.unary(name, left)
            1 -> read(arguments.single(), operatorDepth + 1)?.let { PendingIntentFlagOperators.binary(name, left, it) }
            else -> null
        }
    }
}
