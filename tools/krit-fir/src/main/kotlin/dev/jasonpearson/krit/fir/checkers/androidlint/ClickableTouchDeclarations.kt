package dev.jasonpearson.krit.fir.checkers.androidlint

import dev.jasonpearson.krit.fir.support.firstLineAnchor
import dev.jasonpearson.krit.fir.support.identifierText
import dev.jasonpearson.krit.fir.support.lightText
import org.jetbrains.kotlin.descriptors.Modality
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.declarations.DirectDeclarationsAccess
import org.jetbrains.kotlin.fir.declarations.FirNamedFunction
import org.jetbrains.kotlin.fir.declarations.FirRegularClass
import org.jetbrains.kotlin.fir.resolve.getContainingClassSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirNamedFunctionSymbol
import org.jetbrains.kotlin.lexer.KtTokens

internal object ClickableTouchDeclarations {
    context(context: CheckerContext)
    fun message(declaration: FirNamedFunction): String? {
        val name = declaration.name.asString()
        if (name !in setOf("onTouchEvent", "performClick")) return null
        if (!ClickableTouchSymbols.viewOverride(declaration.symbol)) return null
        val enclosing = context.containingElements.filterIsInstance<FirRegularClass>().lastOrNull() ?: return null
        if (enclosing.status.modality in setOf(Modality.ABSTRACT, Modality.SEALED) ||
            declaration.symbol.getContainingClassSymbol() != enclosing.symbol
        ) return null
        val className = enclosing.source?.let(::identifierText) ?: return null
        return when (name) {
            "onTouchEvent" -> touchMessage(declaration, enclosing, className)
            "performClick" -> clickMessage(declaration, className)
            else -> null
        }
    }

    @OptIn(DirectDeclarationsAccess::class)
    private fun touchMessage(declaration: FirNamedFunction, enclosing: FirRegularClass, name: String): String? {
        val hasClick = enclosing.symbol.declarationSymbols.filterIsInstance<FirNamedFunctionSymbol>()
            .any { it.name.asString() == "performClick" }
        return when {
            !hasClick -> "Custom view `$name` overrides `onTouchEvent` but not `performClick`"
            ClickableTouchBodies.callsClick(declaration) -> null
            else -> "`$name#onTouchEvent` should call `$name#performClick` when a click is detected"
        }
    }

    private fun clickMessage(declaration: FirNamedFunction, name: String): String? {
        val source = declaration.source ?: return null
        val start = firstLineAnchor(source, KtTokens.FUN_KEYWORD) ?: source
        val text = lightText(source, source.lighterASTNode).drop(start.startOffset - source.startOffset)
        return if (superClick.containsMatchIn(text)) null else "`$name#performClick` should call `super#performClick`"
    }

    private val superClick = Regex("""\bsuper\s*(?:<[^>]*>)?\s*(?:@\w+)?\s*\.\s*performClick\s*\(""")
}
