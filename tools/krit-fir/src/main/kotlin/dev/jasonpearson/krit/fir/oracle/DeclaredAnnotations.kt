package dev.jasonpearson.krit.fir.oracle

import org.jetbrains.kotlin.fir.FirSession
import org.jetbrains.kotlin.fir.declarations.DirectDeclarationsAccess
import org.jetbrains.kotlin.fir.declarations.FirConstructor
import org.jetbrains.kotlin.fir.declarations.FirDeclaration
import org.jetbrains.kotlin.fir.declarations.FirNamedFunction
import org.jetbrains.kotlin.fir.declarations.FirRegularClass
import org.jetbrains.kotlin.fir.declarations.toAnnotationClassId
import org.jetbrains.kotlin.fir.symbols.SymbolInternals
import org.jetbrains.kotlin.fir.symbols.impl.FirCallableSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirFunctionSymbol
import org.jetbrains.kotlin.fir.unwrapFakeOverrides

/**
 * The annotations a call's `annotations` field lists for [symbol], as FQNs: what [OracleExpressionChecker] reports.
 */
@OptIn(SymbolInternals::class)
internal fun calleeAnnotationFqns(symbol: FirCallableSymbol<*>, session: FirSession): List<String> =
    symbol.annotations.mapNotNull { it.toAnnotationClassId(session)?.asSingleFqName()?.asString() }

/**
 * A key for a callable that is the same whether the compile reads its declaration from source or from a class file:
 * its callable id, context parameter, receiver and value parameter types, of the declaration a fake override
 * stands for.
 */
internal fun calleeSignature(symbol: FirCallableSymbol<*>): String? {
    val original = symbol.unwrapFakeOverrides()
    val id = original.callableId ?: return null
    val context = original.contextParameterSymbols.joinToString(",") { it.resolvedReturnType.renderFqn() }
    val receiver = original.resolvedReceiverType?.renderFqn().orEmpty()
    val params = (original as? FirFunctionSymbol<*>)?.valueParameterSymbols
        ?.joinToString(",") { it.resolvedReturnType.renderFqn() }.orEmpty()
    return "$id|$context|$receiver|$params"
}

/**
 * Records, for every function and constructor [declarations] declare (members of nested classes included), the
 * annotations a call to it reports, when there are any. A slice compile reads the module's other files as class
 * files, which drop SOURCE-retention annotations (`@Suppress`, say); these source-side lists put them back (#740).
 */
@OptIn(DirectDeclarationsAccess::class)
internal fun recordDeclaredAnnotations(
    collector: OracleCollector, filePath: String, declarations: List<FirDeclaration>, session: FirSession,
) {
    for (declaration in declarations) {
        val symbol = when (declaration) {
            is FirNamedFunction -> declaration.symbol
            is FirConstructor -> declaration.symbol
            is FirRegularClass -> { recordDeclaredAnnotations(collector, filePath, declaration.declarations, session); null }
            else -> null
        } ?: continue
        val annotations = calleeAnnotationFqns(symbol, session)
        if (annotations.isEmpty()) continue
        val signature = calleeSignature(symbol) ?: continue
        collector.addDeclaredAnnotations(filePath, signature, annotations)
    }
}
