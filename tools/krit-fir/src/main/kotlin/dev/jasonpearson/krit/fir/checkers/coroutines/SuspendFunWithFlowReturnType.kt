package dev.jasonpearson.krit.fir.checkers.coroutines

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.classIdOf
import dev.jasonpearson.krit.fir.support.firstLineAnchor
import dev.jasonpearson.krit.fir.support.suspendsInPlace
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.descriptors.ClassKind
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.DeclarationCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.FirDeclarationChecker
import org.jetbrains.kotlin.fir.declarations.FirNamedFunction
import org.jetbrains.kotlin.fir.declarations.utils.isAbstract
import org.jetbrains.kotlin.fir.declarations.utils.isActual
import org.jetbrains.kotlin.fir.declarations.utils.isExpect
import org.jetbrains.kotlin.fir.declarations.utils.isOpen
import org.jetbrains.kotlin.fir.declarations.utils.isOverride
import org.jetbrains.kotlin.fir.declarations.utils.isSuspend
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.symbols.impl.FirRegularClassSymbol
import org.jetbrains.kotlin.fir.types.classId
import org.jetbrains.kotlin.fir.types.coneType
import org.jetbrains.kotlin.lexer.KtTokens

// Flags a `suspend` function that returns a Flow: a flow builder is cold and needs no coroutine, so the
// modifier forces every caller into one for nothing.
//
// Mirrors the Go SuspendFunWithFlowReturnType rule, which matches the written return type name (or, for an
// expression body, an oracle-typed Flow) and decides whether the body suspends by callee name: a fixed list
// of coroutine functions, any project function declared `suspend` and member calls named like a suspend
// overload (`first`, `toList`, `await*`). The checker reads the compiler's answer instead: the return type
// is the type the compiler resolved (an import alias, a typealias, an inferred type and a nullable Flow
// included) and the body suspends when it holds a call that resolves to a suspend function or reads
// `coroutineContext` (see `suspendsInPlace`). Like Go it skips open, abstract, override and expect functions,
// interface members and functions without a body, whose signature a caller, a supertype or a later override
// fixes; an `actual` function is skipped too, since its `expect` declaration fixes the signature.
//
// Only the five types Go lists are reported, not every subtype of Flow.
internal object SuspendFunWithFlowReturnType : FirDeclarationChecker<FirNamedFunction>(MppCheckerKind.Common), FirRule {
    override val ruleId = "SuspendFunWithFlowReturnType"
    override val declarationCheckers = object : DeclarationCheckers() {
        override val namedFunctionCheckers = setOf(SuspendFunWithFlowReturnType)
    }

    private const val FLOW_PACKAGE = "kotlinx.coroutines.flow"
    private val flowTypes = listOf("Flow", "StateFlow", "SharedFlow", "MutableStateFlow", "MutableSharedFlow")
        .map { classIdOf(FLOW_PACKAGE, it) }
        .toSet()

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(declaration: FirNamedFunction) {
        val source = declaration.source ?: return
        if (source.kind !is KtRealSourceElementKind || !declaration.symbol.isSuspend) return
        if (declaration.isSignatureFixed() || insideInterface()) return
        val body = declaration.body ?: return
        val returned = declaration.returnTypeRef.coneType.fullyExpandedType().classId
        if (returned !in flowTypes) return
        if (suspendsInPlace(body)) return
        report(
            firstLineAnchor(source, KtTokens.FUN_KEYWORD) ?: source,
            "Suspend function returns a Flow type. A function that returns Flow should not be suspend. " +
                "The flow builder is cold and does not require a coroutine.",
        )
    }

    private fun FirNamedFunction.isSignatureFixed(): Boolean = isOpen || isAbstract || isOverride || isActual || isExpect

    // Go looks at the nearest enclosing class declaration; an object literal is not one.
    context(context: CheckerContext)
    private fun insideInterface(): Boolean {
        val owner = context.containingDeclarations.lastOrNull { it is FirRegularClassSymbol } as? FirRegularClassSymbol
        return owner?.classKind == ClassKind.INTERFACE
    }
}
