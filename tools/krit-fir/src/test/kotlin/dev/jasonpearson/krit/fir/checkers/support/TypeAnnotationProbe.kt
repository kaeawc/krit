package dev.jasonpearson.krit.fir.checkers.support

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.classIdOf
import dev.jasonpearson.krit.fir.support.expandedBound
import dev.jasonpearson.krit.fir.support.hasTypeAnnotation
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.DeclarationCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.FirPropertyChecker
import org.jetbrains.kotlin.fir.declarations.FirProperty
import org.jetbrains.kotlin.fir.types.coneType

object TypeAnnotationProbe : FirPropertyChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "TypeAnnotationProbe"
    override val declarationCheckers = object : DeclarationCheckers() {
        override val propertyCheckers = setOf(TypeAnnotationProbe)
    }

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(declaration: FirProperty) {
        val annotated = declaration.returnTypeRef.coneType.expandedBound().hasTypeAnnotation(classIdOf("sample", "Marker"))
        report(declaration.source, "${declaration.name.asString()} $annotated")
    }
}
