package dev.krit.fir.rules

import dev.krit.fir.KritDiagnostics
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.diagnostics.reportOn
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.FirClassChecker
import org.jetbrains.kotlin.fir.declarations.FirClass
import org.jetbrains.kotlin.fir.declarations.FirRegularClass

// Smoke-test checker: flags any class named "Smoke" so the runner can be verified end-to-end.
internal object SmokeChecker : FirClassChecker(MppCheckerKind.Common) {
    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(declaration: FirClass) {
        val source = declaration.source ?: return
        val name = (declaration as? FirRegularClass)?.name?.asString() ?: return
        if (name != "Smoke") return
        reporter.reportOn(source, KritDiagnostics.SMOKE_CLASS)
    }
}
