package dev.jasonpearson.krit.fir.support

import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.declarations.FirImport
import org.jetbrains.kotlin.fir.symbols.SymbolInternals

@OptIn(SymbolInternals::class)
fun importsInCheckedFile(context: CheckerContext): List<FirImport> =
    context.containingFileSymbol?.fir?.imports.orEmpty()
