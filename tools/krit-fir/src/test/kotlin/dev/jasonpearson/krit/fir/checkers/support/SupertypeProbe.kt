package dev.jasonpearson.krit.fir.checkers.support

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.classIdOf
import dev.jasonpearson.krit.fir.support.inheritsFrom
import dev.jasonpearson.krit.fir.support.inheritsFromAny
import dev.jasonpearson.krit.fir.support.isOrInheritsFrom
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.DeclarationCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.FirClassChecker
import org.jetbrains.kotlin.fir.declarations.FirClass
import org.jetbrains.kotlin.fir.declarations.FirRegularClass

/**
 * Test-only rule that exercises the shared supertype helpers on real symbols.
 *
 * For each class it reports `Name isOr[Base,Iface]=a,b inh[Base,Iface]=c,d any=e local=f` for the
 * superclass-only walk (`s`) and the interface-inclusive walk (`i`). `local` is true when a
 * supertype is named `LocalBase`, which cannot be spelled as a top-level class id.
 */
object SupertypeProbe : FirRule {
    override val ruleId = "SupertypeProbe"
    private val base = classIdOf("", "Base")
    private val iface = classIdOf("", "Iface")

    private object Classes : FirClassChecker(MppCheckerKind.Common) {
        context(context: CheckerContext, reporter: DiagnosticReporter)
        override fun check(declaration: FirClass) {
            val name = (declaration as? FirRegularClass)?.name?.asString() ?: return
            val session = context.session
            val symbol = declaration.symbol
            fun flags(interfaces: Boolean): String = buildString {
                append("isOr=").append(isOrInheritsFrom(symbol, base, session, interfaces))
                append(',').append(isOrInheritsFrom(symbol, iface, session, interfaces))
                append(" any=").append(inheritsFromAny(symbol, setOf(base, iface), session, interfaces))
                append(" local=").append(inheritsFrom(symbol, session, interfaces) { it.shortClassName.asString() == "LocalBase" })
            }
            report(declaration.source, "$name s[${flags(false)}] i[${flags(true)}]")
        }
    }

    override val declarationCheckers = object : DeclarationCheckers() {
        override val classCheckers = setOf(Classes)
    }
}
