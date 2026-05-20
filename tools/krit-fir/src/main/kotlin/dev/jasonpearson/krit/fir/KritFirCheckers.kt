package dev.jasonpearson.krit.fir

import dev.jasonpearson.krit.fir.checkers.ComposeRememberWithoutKey
import dev.jasonpearson.krit.fir.checkers.FlowCollectInOnCreate
import dev.jasonpearson.krit.fir.checkers.InjectDispatcher
import dev.jasonpearson.krit.fir.checkers.UnsafeCastWhenNullable
import dev.jasonpearson.krit.fir.oracle.OracleClassChecker
import dev.jasonpearson.krit.fir.oracle.OracleExpressionChecker
import dev.jasonpearson.krit.fir.oracle.OracleQualifiedAccessChecker
import dev.jasonpearson.krit.fir.rules.SmokeChecker
import org.jetbrains.kotlin.fir.FirSession
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.DeclarationCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.extra.UnreachableCodeChecker
import org.jetbrains.kotlin.fir.analysis.extensions.FirAdditionalCheckersExtension

class KritFirCheckers(session: FirSession) : FirAdditionalCheckersExtension(session) {
    // The oracle expression checker is included unconditionally and gates
    // itself on `OracleCollectorRegistry.current() != null`, so non-oracle
    // paths (diagnostic `check` command) pay only a thread-local lookup
    // per call expression.
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(
            FlowCollectInOnCreate,
            ComposeRememberWithoutKey,
            InjectDispatcher,
            OracleExpressionChecker,
        )
        override val typeOperatorCallCheckers = setOf(
            UnsafeCastWhenNullable,
        )
        // OracleQualifiedAccessChecker fills in the expression-type
        // gap that OracleExpressionChecker leaves: property reads,
        // variable references, receiver expressions in non-call
        // chains. The checker bypasses FirFunctionCall (which is also
        // a FirQualifiedAccessExpression) so call-site entries from
        // OracleExpressionChecker remain authoritative on collisions.
        override val qualifiedAccessExpressionCheckers = setOf(
            OracleQualifiedAccessChecker,
        )
    }

    // Same self-gating story for OracleClassChecker on the declaration
    // side: non-oracle paths pay only the thread-local probe per
    // declaration. Per-lambda suspend status is captured at the call
    // site inside `OracleExpressionChecker` — see its
    // `recordLambdaSuspendArguments` for the rationale.
    //
    // `UnreachableCodeChecker` lives in K2's `extra` package and isn't
    // wired into the default checker pipeline; registering it here
    // lets the oracle's `OracleDiagnosticMessageCollector` surface the
    // UNREACHABLE_CODE factory alongside USELESS_ELVIS and
    // CAST_NEVER_SUCCEEDS. The class is a static object on K2's side,
    // so adding it costs nothing extra at JVM init.
    override val declarationCheckers = object : DeclarationCheckers() {
        override val classCheckers = setOf(SmokeChecker, OracleClassChecker)
        override val controlFlowAnalyserCheckers = setOf(UnreachableCodeChecker)
    }
}
