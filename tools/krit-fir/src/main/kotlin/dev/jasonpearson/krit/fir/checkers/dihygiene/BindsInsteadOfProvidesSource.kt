package dev.jasonpearson.krit.fir.checkers.dihygiene

import dev.jasonpearson.krit.fir.support.lightText
import dev.jasonpearson.krit.fir.support.importsInCheckedFile
import dev.jasonpearson.krit.fir.support.significantChildren
import dev.jasonpearson.krit.fir.support.simpleName
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.declarations.FirNamedFunction
import org.jetbrains.kotlin.fir.expressions.FirAnnotation
import org.jetbrains.kotlin.lexer.KtTokens
import org.jetbrains.kotlin.text

internal object BindsInsteadOfProvidesSource {
    private val scopes = setOf(
        "Singleton", "Reusable", "ApplicationScoped", "ActivityScoped", "ActivityRetainedScoped",
        "FragmentScoped", "ViewScoped", "ViewModelScoped", "ServiceScoped", "SessionScoped",
        "RequestScoped", "UserScoped",
    )

    fun eligible(declaration: FirNamedFunction, context: CheckerContext): Boolean {
        val parameter = declaration.valueParameters.singleOrNull() ?: return false
        return bareExpression(declaration) && !importsKotlinInject(context) &&
            declaration.annotations.none { it.simpleName() in scopes } &&
            annotationNames(declaration.annotations) - "Provides" == annotationNames(parameter.annotations)
    }

    private fun importsKotlinInject(context: CheckerContext): Boolean = importsInCheckedFile(context).any {
        it.importedFqName?.asString() == "me.tatarka.inject.annotations.Provides" ||
            (it.isAllUnder && it.importedFqName?.asString() == "me.tatarka.inject.annotations")
    }

    private fun bareExpression(declaration: FirNamedFunction): Boolean {
        val source = declaration.source ?: return false
        val children = significantChildren(source, source.lighterASTNode)
        val expression = children.dropWhile { it.tokenType != KtTokens.EQ }.drop(1).singleOrNull()
        val parameter = declaration.valueParameters.singleOrNull() ?: return false
        return expression?.tokenType == KtNodeTypes.REFERENCE_EXPRESSION &&
            lightText(source, expression).trim().removeSurrounding("`") == parameter.name.asString()
    }

    private fun annotationNames(annotations: List<FirAnnotation>): Set<String> = annotations.mapTo(mutableSetOf()) {
        it.source?.text?.toString().orEmpty().trim().removePrefix("@").removePrefix("param:")
            .takeWhile { character -> character != '(' && !character.isWhitespace() }
    }
}
