package dev.jasonpearson.krit.fir.checkers.security

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.checkers.followableInitializer
import dev.jasonpearson.krit.fir.containingScanPath
import dev.jasonpearson.krit.fir.containingTargetSdk
import dev.jasonpearson.krit.fir.isInTestFile
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.unwrapArgument
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.declarations.utils.isConst
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirLiteralExpression
import org.jetbrains.kotlin.fir.expressions.FirPropertyAccessExpression
import org.jetbrains.kotlin.fir.expressions.resolvedArgumentMapping
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirFunctionSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirPropertySymbol
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name

// Flags android.app.PendingIntent.getActivity, getActivities, getBroadcast and getService when the flags
// argument is an explicit flag expression with no FLAG_IMMUTABLE or FLAG_MUTABLE: Android 12 (API 31)
// requires one of them.
//
// Like Go, the flags are read only when every part of the expression is visible: integer literals and
// PendingIntent.FLAG_* constants joined by `or`, `and`, `xor` or `plus`. A parameter, a local, a helper
// call or an `if` makes the flags opaque and leaves the call alone; a `const val` is read through its
// value. Go skips test files, testFixtures sources and test-fixture modules, and a module whose
// targetSdk is known and below 31.
//
// Differences from Go, pinned in the golden data:
// - Recall: the call is identified by resolution, so an import alias of PendingIntent and a statically
//   imported `getActivity(...)` are reported; Go needs the receiver spelled `PendingIntent`.
// - Precision: a project class named PendingIntent is not android.app.PendingIntent. A literal that holds
//   the FLAG_IMMUTABLE or FLAG_MUTABLE bit counts as a mutability flag, and a non-const FLAG_* property is
//   opaque; Go reads any identifier containing FLAG_ as a flag, so it reports `Other.FLAG_X`.
internal object ImplicitPendingIntent : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "ImplicitPendingIntent"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(ImplicitPendingIntent)
    }

    private const val MESSAGE =
        "PendingIntent flags omit FLAG_IMMUTABLE or FLAG_MUTABLE. Add an explicit mutability flag for Android 12+."

    // The first release that requires an explicit mutability flag.
    private const val MUTABILITY_REQUIRED_API = 31

    // Every overload takes the flags as its fourth parameter.
    private const val FLAGS_PARAMETER_INDEX = 3

    // FLAG_IMMUTABLE (1 shl 26) and FLAG_MUTABLE (1 shl 25).
    private const val MUTABILITY_BITS = 0x06000000L

    private val pendingIntentId = ClassId(FqName("android.app"), Name.identifier("PendingIntent"))
    private val intId = ClassId(FqName("kotlin"), Name.identifier("Int"))
    private val factoryNames = setOf("getActivity", "getBroadcast", "getService", "getActivities")
    private val mutabilityConstants = setOf("FLAG_IMMUTABLE", "FLAG_MUTABLE")
    private val flagOperators = setOf("or", "and", "xor", "plus")

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val callee = expression.calleeReference.toResolvedCallableSymbol() as? FirFunctionSymbol<*> ?: return
        val callableId = callee.callableId
        if (callableId.classId != pendingIntentId || callableId.callableName.asString() !in factoryNames) return
        if (isInTestFile() || isTestFixturePath(containingScanPath())) return
        if (containingTargetSdk() in 1 until MUTABILITY_REQUIRED_API) return
        val flagsParameter = callee.valueParameterSymbols.getOrNull(FLAGS_PARAMETER_INDEX) ?: return
        val flags = expression.resolvedArgumentMapping
            ?.entries
            ?.firstOrNull { it.value.symbol == flagsParameter }
            ?.key
            ?: return
        val scan = FlagScan { it.followableInitializer() }
        if (!scan.read(flags) || !scan.sawConstant || scan.hasMutability) return
        report(expression.source, MESSAGE)
    }

    // Go skips `/testFixtures/` directories and any directory named like a test module.
    private fun isTestFixturePath(path: String?): Boolean {
        if (path == null) return false
        if ("/testFixtures/" in path) return true
        return path.lowercase().split('/').dropLast(1).any {
            it.endsWith("-test") || it.endsWith("-testing") || it.endsWith("test-fixtures") || it.endsWith("testfixtures")
        }
    }

    private class FlagScan(private val constantValue: (FirPropertySymbol) -> FirExpression?) {
        var sawConstant = false
        var hasMutability = false

        // False when some part of the flags cannot be read statically.
        fun read(argument: FirExpression): Boolean = when (val expression = unwrapArgument(argument)) {
            is FirLiteralExpression -> readLiteral(expression)
            is FirPropertyAccessExpression -> readConstant(expression)
            is FirFunctionCall -> readOperator(expression)
            else -> false
        }

        private fun readLiteral(literal: FirLiteralExpression): Boolean {
            val value = (literal.value as? Number)?.toLong() ?: return false
            sawConstant = true
            if (value and MUTABILITY_BITS != 0L) hasMutability = true
            return true
        }

        private fun readConstant(access: FirPropertyAccessExpression): Boolean {
            val symbol = access.calleeReference.toResolvedCallableSymbol() ?: return false
            val callableId = symbol.callableId ?: return false
            if (callableId.classId != pendingIntentId) {
                // A `const val` of any other class is read through its evaluated value.
                val value = (symbol as? FirPropertySymbol)?.takeIf { it.isConst }?.let(constantValue) ?: return false
                return read(value)
            }
            val name = callableId.callableName.asString()
            if (!name.startsWith("FLAG_")) return false
            sawConstant = true
            if (name in mutabilityConstants) hasMutability = true
            return true
        }

        private fun readOperator(call: FirFunctionCall): Boolean {
            val callableId = call.calleeReference.toResolvedCallableSymbol()?.callableId ?: return false
            if (callableId.classId != intId || callableId.callableName.asString() !in flagOperators) return false
            val receiver = call.explicitReceiver ?: return false
            return read(receiver) && call.argumentList.arguments.all { read(it) }
        }
    }
}
