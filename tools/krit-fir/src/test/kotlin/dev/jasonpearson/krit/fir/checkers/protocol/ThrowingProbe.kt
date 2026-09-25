package dev.jasonpearson.krit.fir.checkers.protocol

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.DeclarationCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.FirClassChecker
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.analysis.checkers.type.FirTypeChecker
import org.jetbrains.kotlin.fir.analysis.checkers.type.TypeCheckers
import org.jetbrains.kotlin.fir.declarations.FirClass
import org.jetbrains.kotlin.fir.declarations.FirRegularClass
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.types.FirResolvedTypeRef
import org.jetbrains.kotlin.fir.types.classId
import java.util.concurrent.atomic.AtomicInteger

/**
 * Test-only rule whose checkers throw on chosen constructs, one per checker
 * family, to prove a checker exception is isolated to its rule and file:
 *  - a call to `throwingProbeReport()` is reported normally;
 *  - a call to `throwingProbeCrash()` throws from the expression checker;
 *  - a class named `ThrowingProbeCrashClass` throws from the declaration checker;
 *  - a reference to a type named `ThrowingProbeCrashType` throws from the type checker.
 *
 * tests/parity/fir_rule_isolation_test.go copies these classes into the
 * production jar and runs the probe under a real Go rule id (Go only sends
 * catalog ids to krit-fir), passed as the [RULE_ID_PROPERTY] system property.
 */
object ThrowingProbe : FirRule {
    const val RULE_ID_PROPERTY = "krit.fir.test.throwingProbeRuleId"
    override val ruleId: String = System.getProperty(RULE_ID_PROPERTY) ?: "ThrowingProbe"
    const val MESSAGE = "probe \"boom\" \\ on purpose"
    val expressionThrows = AtomicInteger()
    val declarationThrows = AtomicInteger()
    val typeThrows = AtomicInteger()

    private object Calls : FirFunctionCallChecker(MppCheckerKind.Common) {
        context(context: CheckerContext, reporter: DiagnosticReporter)
        override fun check(expression: FirFunctionCall) {
            when (expression.calleeReference.toResolvedCallableSymbol()?.name?.asString()) {
                "throwingProbeReport" -> report(expression.source, "reported")
                "throwingProbeCrash" -> {
                    expressionThrows.incrementAndGet()
                    throw IllegalStateException("$MESSAGE\nsecond line")
                }
            }
        }
    }

    private object Classes : FirClassChecker(MppCheckerKind.Common) {
        context(context: CheckerContext, reporter: DiagnosticReporter)
        override fun check(declaration: FirClass) {
            if ((declaration as? FirRegularClass)?.name?.asString() == "ThrowingProbeCrashClass") {
                declarationThrows.incrementAndGet()
                throw UnsupportedOperationException("declaration $MESSAGE")
            }
        }
    }

    private object Types : FirTypeChecker<FirResolvedTypeRef>(MppCheckerKind.Common) {
        context(context: CheckerContext, reporter: DiagnosticReporter)
        override fun check(typeRef: FirResolvedTypeRef) {
            if (typeRef.coneType.classId?.shortClassName?.asString() == "ThrowingProbeCrashType") {
                typeThrows.incrementAndGet()
                throw StackOverflowError("type $MESSAGE")
            }
        }
    }

    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(Calls)
    }
    override val declarationCheckers = object : DeclarationCheckers() {
        override val classCheckers = setOf(Classes)
    }
    override val typeCheckers = object : TypeCheckers() {
        override val resolvedTypeRefCheckers = setOf(Types)
    }

    fun reset() {
        expressionThrows.set(0)
        declarationThrows.set(0)
        typeThrows.set(0)
    }
}
