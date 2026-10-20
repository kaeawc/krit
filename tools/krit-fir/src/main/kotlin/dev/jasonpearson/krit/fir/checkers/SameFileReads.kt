package dev.jasonpearson.krit.fir.checkers

import org.jetbrains.kotlin.fir.FirEvaluatorResult
import org.jetbrains.kotlin.fir.FirSession
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.containingClassLookupTag
import org.jetbrains.kotlin.fir.declarations.FirDeclarationOrigin
import org.jetbrains.kotlin.fir.declarations.utils.isConst
import org.jetbrains.kotlin.fir.expressions.FirBlock
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirExpressionEvaluator
import org.jetbrains.kotlin.fir.resolve.providers.firProvider
import org.jetbrains.kotlin.fir.resolve.providers.getContainingFile
import org.jetbrains.kotlin.fir.symbols.FirBasedSymbol
import org.jetbrains.kotlin.fir.symbols.SymbolInternals
import org.jetbrains.kotlin.fir.symbols.impl.FirAnonymousFunctionSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirCallableSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirClassLikeSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirFileSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirFunctionSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirPropertyAccessorSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirPropertySymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirValueParameterSymbol

// The only way a rule that is not tiered (`FirRule.wholeCompilationTier`) may read another declaration's
// body or initializer. An incremental slice compiles the changed files from source and takes the rest of the
// module from class files, which keep signatures, inferred types, `const` values and annotations but no
// bodies and no initializers. A rule gives the same verdict on a slice and on a full compile only if it
// follows a body or initializer solely where the declaration lies in the file under check (always source in
// both), or where the value is a `const` (kept in class files, see [followableInitializer]). Every other
// declaration is treated as a class file declaration would be: its body and initializer are unknown.
//
// The functions here hold the reads, so the rules' own sources do not; the whole-compilation gate
// (`FirRuleWholeCompilationGateTest`) lists the reads below with their proof.

/**
 * Whether this symbol is declared in [file], the file under check: a local declaration (it lies in the file
 * that uses it), or a source declaration whose containing file is [file]. A declaration of another file, a
 * deserialized one, and a fake override are not: their body and initializer are not read.
 */
fun FirBasedSymbol<*>.isDeclaredInFile(file: FirFileSymbol?, session: FirSession): Boolean {
    if (file == null) return false
    return when (this) {
        // A parameter lies where its function does; an accessor where its property does.
        is FirValueParameterSymbol -> containingDeclarationSymbol.isDeclaredInFile(file, session)
        is FirPropertyAccessorSymbol -> propertySymbol.isDeclaredInFile(file, session)
        else -> isLocalDeclaration() ||
            (origin == FirDeclarationOrigin.Source && session.firProvider.getContainingFile(this)?.symbol == file)
    }
}

/** [isDeclaredInFile] for the file [context] is checking. */
context(context: CheckerContext)
fun FirBasedSymbol<*>.isDeclaredInCheckedFile(): Boolean = isDeclaredInFile(context.containingFileSymbol, context.session)

// A local variable, function, or class, or a member of a local class: it lies in the file that uses it.
private fun FirBasedSymbol<*>.isLocalDeclaration(): Boolean = when (this) {
    is FirPropertySymbol -> isLocal || containingClassLookupTag()?.classId?.isLocal == true
    is FirAnonymousFunctionSymbol -> true
    is FirCallableSymbol<*> -> callableId?.isLocal == true || containingClassLookupTag()?.classId?.isLocal == true
    is FirClassLikeSymbol<*> -> classId.isLocal
    else -> false
}

/**
 * The initializer of this property when it lies in [file]; null otherwise, as for a property read from a class
 * file. LOCAL: guarded by [isDeclaredInFile].
 */
@OptIn(SymbolInternals::class)
fun FirPropertySymbol.initializerIn(file: FirFileSymbol?, session: FirSession): FirExpression? =
    if (isDeclaredInFile(file, session)) resolvedInitializer else null

/** [initializerIn] for the file [context] is checking. */
context(context: CheckerContext)
fun FirPropertySymbol.initializerInCheckedFile(): FirExpression? =
    initializerIn(context.containingFileSymbol, context.session)

/**
 * What a rule may follow to find this property's value: the evaluated value of a `const`, wherever it is
 * declared (class files keep it), and otherwise the property's own initializer when it lies in the file under
 * check. The evaluated value is what a class file holds, so a `const val X = 2 * 1000L` is a literal 2000 on a
 * slice and on a full compile alike, and in the file under check as well. Null for every other property of
 * another file: its initializer is unknown.
 */
@OptIn(SymbolInternals::class)
fun FirPropertySymbol.followableInitializer(file: FirFileSymbol?, session: FirSession): FirExpression? {
    // A const is evaluated wherever it is declared, so a rule reads the same value from the file under check and
    // from another file (`const val X = 2 * 1000L` is a literal 2000 in both).
    if (isConst) {
        val evaluated = (FirExpressionEvaluator.evaluatePropertyInitializer(fir, session) as? FirEvaluatorResult.Evaluated)
            ?.result as? FirExpression
        return evaluated ?: resolvedInitializer
    }
    if (isDeclaredInFile(file, session)) return resolvedInitializer
    return null
}

/** [followableInitializer] for the file [context] is checking. */
context(context: CheckerContext)
fun FirPropertySymbol.followableInitializer(): FirExpression? =
    followableInitializer(context.containingFileSymbol, context.session)

/** The delegate expression of this property when it lies in [file]; null otherwise. LOCAL: guarded. */
@OptIn(SymbolInternals::class)
fun FirPropertySymbol.delegateIn(file: FirFileSymbol?, session: FirSession): FirExpression? =
    if (isDeclaredInFile(file, session)) fir.delegate else null

/** [delegateIn] for the file [context] is checking. */
context(context: CheckerContext)
fun FirPropertySymbol.delegateInCheckedFile(): FirExpression? = delegateIn(context.containingFileSymbol, context.session)

/** The body of this function or accessor when it lies in [file]; null otherwise. LOCAL: guarded. */
@OptIn(SymbolInternals::class)
fun FirFunctionSymbol<*>.bodyIn(file: FirFileSymbol?, session: FirSession): FirBlock? =
    if (isDeclaredInFile(file, session)) fir.body else null

/** [bodyIn] for the file [context] is checking. */
context(context: CheckerContext)
fun FirFunctionSymbol<*>.bodyInCheckedFile(): FirBlock? = bodyIn(context.containingFileSymbol, context.session)
