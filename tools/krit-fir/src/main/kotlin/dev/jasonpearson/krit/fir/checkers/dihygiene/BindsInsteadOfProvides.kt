package dev.jasonpearson.krit.fir.checkers.dihygiene

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.classIdOf
import dev.jasonpearson.krit.fir.support.identifierAnchor
import dev.jasonpearson.krit.fir.support.identifierText
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.descriptors.ClassKind
import org.jetbrains.kotlin.descriptors.Modality
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.DeclarationCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.FirDeclarationChecker
import org.jetbrains.kotlin.fir.declarations.FirNamedFunction
import org.jetbrains.kotlin.fir.declarations.FirValueParameter
import org.jetbrains.kotlin.fir.declarations.toAnnotationClassId
import org.jetbrains.kotlin.fir.expressions.FirQualifiedAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirReturnExpression
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirAnonymousInitializerSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirCallableSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirClassSymbol
import org.jetbrains.kotlin.fir.types.coneType
import org.jetbrains.kotlin.fir.types.isMarkedNullable

internal object BindsInsteadOfProvides : FirDeclarationChecker<FirNamedFunction>(MppCheckerKind.Common), FirRule {
    override val ruleId = "BindsInsteadOfProvides"
    override val declarationCheckers = object : DeclarationCheckers() {
        override val namedFunctionCheckers = setOf(BindsInsteadOfProvides)
    }

    private val provides = classIdOf("dagger", "Provides")
    private val module = classIdOf("dagger", "Module")

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(declaration: FirNamedFunction) {
        val source = declaration.source ?: return
        val parameter = candidate(declaration, context) ?: return
        val advice = advice(declaration, context) ?: return
        report(identifierAnchor(source) ?: source, message(declaration, parameter, advice))
    }

    private fun candidate(declaration: FirNamedFunction, context: CheckerContext): FirValueParameter? {
        val parameter = declaration.valueParameters.singleOrNull() ?: return null
        if (declaration.annotations.none { it.toAnnotationClassId(context.session) == provides }) return null
        return parameter.takeIf {
            declaration.source?.kind is KtRealSourceElementKind &&
                BindsInsteadOfProvidesSource.eligible(declaration, context) &&
                unchangedParameter(declaration, parameter)
        }
    }

    private fun unchangedParameter(declaration: FirNamedFunction, parameter: FirValueParameter): Boolean {
        val returned = declaration.body?.statements?.singleOrNull() as? FirReturnExpression
        val access = returned?.result as? FirQualifiedAccessExpression
        return !parameter.returnTypeRef.coneType.isMarkedNullable &&
            !declaration.returnTypeRef.coneType.isMarkedNullable &&
            access?.calleeReference?.toResolvedCallableSymbol() == parameter.symbol
    }

    private fun advice(declaration: FirNamedFunction, context: CheckerContext): String? {
        val enclosing = context.containingDeclarations.filter { it != declaration.symbol }.asReversed()
        if (enclosing.takeWhile { it !is FirClassSymbol<*> }.any {
                it is FirCallableSymbol<*> || it is FirAnonymousInitializerSymbol
            }) return null
        val hosts = enclosing.filterIsInstance<FirClassSymbol<*>>()
        val companion = hosts.firstOrNull()?.resolvedStatus?.isCompanion == true
        val host = hosts.firstOrNull { !it.resolvedStatus.isCompanion } ?: return null
        if (!isModuleHost(host)) return null
        return if (companion) {
            "a companion object cannot hold @Binds, so move it to an abstract @Binds member of the enclosing module."
        } else {
            "replace with @Binds for a cheaper abstract binding."
        }
    }

    private fun isModuleHost(host: FirClassSymbol<*>): Boolean =
        (host.classKind == ClassKind.INTERFACE || host.resolvedStatus.modality == Modality.ABSTRACT) &&
            module in host.resolvedAnnotationClassIds

    private fun message(declaration: FirNamedFunction, parameter: FirValueParameter, advice: String): String {
        val name = declaration.source?.let(::identifierText) ?: declaration.name.asString()
        val parameterName = parameter.source?.let(::identifierText) ?: parameter.name.asString()
        return "@Provides function '$name' returns its single parameter '$parameterName' unchanged; $advice"
    }
}
