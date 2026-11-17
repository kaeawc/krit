package dev.jasonpearson.krit.fir.checkers.support

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.expandedBound
import dev.jasonpearson.krit.fir.support.expandedClassType
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.DeclarationCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.FirPropertyChecker
import org.jetbrains.kotlin.fir.declarations.FirProperty
import org.jetbrains.kotlin.fir.types.ConeClassLikeType
import org.jetbrains.kotlin.fir.types.coneType
import org.jetbrains.kotlin.fir.types.isMarkedNullable

/**
 * Test-only rule that exercises the shared expanded-type helpers on real property types.
 *
 * For each property it reports `name class=<class id or none> bound=<Class|Other> nullable=<flag>`, where
 * `class` comes from expandedClassType and `bound` records whether expandedBound is itself a class type.
 */
object ExpandedTypeProbe : FirRule {
    override val ruleId = "ExpandedTypeProbe"

    private object Properties : FirPropertyChecker(MppCheckerKind.Common) {
        context(context: CheckerContext, reporter: DiagnosticReporter)
        override fun check(declaration: FirProperty) {
            val type = declaration.returnTypeRef.coneType
            val classId = type.expandedClassType()?.lookupTag?.classId?.asString() ?: "none"
            val bound = if (type.expandedBound() is ConeClassLikeType) "Class" else "Other"
            report(
                declaration.source,
                "${declaration.name.asString()} class=$classId bound=$bound nullable=${type.expandedBound().isMarkedNullable}",
            )
        }
    }

    override val declarationCheckers = object : DeclarationCheckers() {
        override val propertyCheckers = setOf(Properties)
    }
}
