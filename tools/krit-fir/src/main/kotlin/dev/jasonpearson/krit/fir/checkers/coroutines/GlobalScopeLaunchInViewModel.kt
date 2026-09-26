package dev.jasonpearson.krit.fir.checkers.coroutines

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import org.jetbrains.kotlin.descriptors.ClassKind
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.symbols.impl.FirRegularClassSymbol
import org.jetbrains.kotlin.fir.types.classId
import org.jetbrains.kotlin.fir.types.lowerBoundIfFlexible
import org.jetbrains.kotlin.fir.types.resolvedType
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName

/**
 * Port of the Go GlobalScopeLaunchInViewModel rule: a `launch` or `async` call
 * on `GlobalScope` inside a class whose name ends in `ViewModel` or
 * `Presenter`, reported on the call.
 *
 * Mirrored from Go:
 * - the call must be named `launch` or `async` and have an explicit receiver;
 *   an implicit `GlobalScope` receiver (`with(GlobalScope) { launch {} }`) is
 *   not reported;
 * - the owner is the nearest enclosing class, interface, enum class, or
 *   annotation class, local ones included. Objects, companion objects,
 *   anonymous objects, and enum-entry bodies are looked through, as Go's
 *   `class_declaration` walk does. Only the owner's simple name counts, not
 *   its supertypes;
 * - test files are not skipped.
 *
 * Deliberate differences from Go, pinned by goldens and listed in the PR:
 * - Precision: the receiver's type must be kotlinx.coroutines.GlobalScope. Go
 *   takes any receiver spelled `GlobalScope`, so it also reports a local
 *   `object GlobalScope` or a property named `GlobalScope` that holds some
 *   other scope.
 * - Recall: FIR also reports a receiver that is `GlobalScope` under another
 *   spelling (an import or type alias, a property or local holding
 *   `GlobalScope`), and a builder imported under an alias. Go reads names only.
 */
internal object GlobalScopeLaunchInViewModel : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "GlobalScopeLaunchInViewModel"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(GlobalScopeLaunchInViewModel)
    }

    private val globalScope = ClassId.topLevel(FqName("kotlinx.coroutines.GlobalScope"))
    private val builders = setOf("launch", "async")
    private val ownerSuffixes = listOf("ViewModel", "Presenter")

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val source = expression.source ?: return
        val callee = expression.calleeReference.toResolvedCallableSymbol() ?: return
        val method = callee.name.asString()
        if (method !in builders) return

        // A safe call's explicit receiver is the checked subject, typed as
        // the non-null receiver, so `GlobalScope?.launch {}` counts too.
        val receiver = expression.explicitReceiver ?: return
        val receiverType = receiver.resolvedType.fullyExpandedType().lowerBoundIfFlexible()
        if (receiverType.classId != globalScope) return

        // Go walks up to the nearest tree-sitter `class_declaration`: a class,
        // interface, enum class, or annotation class. `object`, companion
        // objects, object literals, and enum entries are other node types.
        val owner = context.containingDeclarations.asReversed()
            .firstOrNull { it is FirRegularClassSymbol && it.classKind != ClassKind.OBJECT && it.classKind != ClassKind.ENUM_ENTRY }
            as? FirRegularClassSymbol ?: return
        val ownerName = owner.name.asString()
        if (ownerSuffixes.none { ownerName.endsWith(it) }) return

        report(source, "GlobalScope.$method in $ownerName. Use viewModelScope instead for lifecycle-aware cancellation.")
    }
}
