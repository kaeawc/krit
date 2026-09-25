package dev.jasonpearson.krit.fir.oracle

import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.FirFileChecker
import org.jetbrains.kotlin.fir.declarations.FirFile

/**
 * Records every compiled file's package, so the oracle result has an entry
 * for every file in the compilation, including one that declares no class
 * and makes no call (a typealias or a constant, say). The Go cache relies on
 * that: a krit-fir run must refresh every file's cached facts, and a file
 * missing from the result would keep an entry from an earlier compilation.
 */
internal object OracleFileChecker : FirFileChecker(MppCheckerKind.Common) {
    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(declaration: FirFile) {
        val collector = OracleCollectorRegistry.current() ?: return
        val filePath = declaration.sourceFile?.path ?: return
        collector.setPackage(filePath, declaration.packageDirective.packageFqName.asString())
    }
}
