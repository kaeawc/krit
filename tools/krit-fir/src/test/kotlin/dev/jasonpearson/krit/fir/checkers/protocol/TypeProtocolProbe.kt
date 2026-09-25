package dev.jasonpearson.krit.fir.checkers.protocol

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.type.FirTypeChecker
import org.jetbrains.kotlin.fir.analysis.checkers.type.TypeCheckers
import org.jetbrains.kotlin.fir.types.FirResolvedTypeRef
import org.jetbrains.kotlin.fir.types.classId

/** Test-only rule contributing a type checker, proving the TypeCheckers family is wired. */
object TypeProtocolProbe : FirTypeChecker<FirResolvedTypeRef>(MppCheckerKind.Common), FirRule {
    override val ruleId = "TypeProtocolProbe"
    override val typeCheckers = object : TypeCheckers() {
        override val resolvedTypeRefCheckers = setOf(TypeProtocolProbe)
    }

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(typeRef: FirResolvedTypeRef) {
        if (typeRef.coneType.classId?.shortClassName?.asString() == "ProtocolProbeType") {
            report(typeRef.source, "type probe")
        }
    }
}
