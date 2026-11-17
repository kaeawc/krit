package dev.jasonpearson.krit.fir.checkers.support

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.isNonNothingSubtypeOf
import dev.jasonpearson.krit.fir.support.nullableClassType
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.DeclarationCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.FirPropertyChecker
import org.jetbrains.kotlin.fir.declarations.FirProperty
import org.jetbrains.kotlin.fir.types.coneType
import org.jetbrains.kotlin.fir.types.isSubtypeOf
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name

/**
 * Test-only rule that exercises isNonNothingSubtypeOf on real property types.
 *
 * For each property it reports `name plain=<flag> guarded=<flag>`, where `plain` is the raw `isSubtypeOf` against
 * `CharSequence?` and `guarded` is the shared helper.
 */
object SubtypeProbe : FirRule {
    override val ruleId = "SubtypeProbe"

    private val nullableCharSequence = ClassId(FqName("kotlin"), Name.identifier("CharSequence")).nullableClassType()

    private object Properties : FirPropertyChecker(MppCheckerKind.Common) {
        context(context: CheckerContext, reporter: DiagnosticReporter)
        override fun check(declaration: FirProperty) {
            val type = declaration.returnTypeRef.coneType
            val plain = type.isSubtypeOf(nullableCharSequence, context.session)
            val guarded = type.isNonNothingSubtypeOf(nullableCharSequence)
            report(declaration.source, "${declaration.name.asString()} plain=$plain guarded=$guarded")
        }
    }

    override val declarationCheckers = object : DeclarationCheckers() {
        override val propertyCheckers = setOf(Properties)
    }
}
