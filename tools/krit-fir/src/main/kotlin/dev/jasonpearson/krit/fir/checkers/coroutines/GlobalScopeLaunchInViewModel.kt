package dev.jasonpearson.krit.fir.checkers.coroutines

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import org.jetbrains.kotlin.descriptors.ClassKind
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.declarations.utils.isFinal
import org.jetbrains.kotlin.fir.declarations.utils.isLateInit
import org.jetbrains.kotlin.fir.expressions.FirCheckedSafeCallSubject
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirPropertyAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirSmartCastExpression
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.symbols.impl.FirConstructorSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirPropertySymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirRegularClassSymbol
import org.jetbrains.kotlin.fir.types.classId
import org.jetbrains.kotlin.fir.types.lowerBoundIfFlexible
import org.jetbrains.kotlin.fir.types.resolvedType
import org.jetbrains.kotlin.name.CallableId
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.text

/**
 * Port of the Go GlobalScopeLaunchInViewModel rule: a `launch` or `async` call
 * on `GlobalScope` inside a class whose name ends in `ViewModel` or
 * `Presenter`, reported on the call.
 *
 * Mirrored from Go:
 * - the call must be named `launch` or `async` and have an explicit receiver;
 *   an implicit `GlobalScope` receiver (`with(GlobalScope) { launch {} }`) is
 *   not reported, while an explicit `this.launch {}` there is;
 * - the owner is the nearest enclosing class, interface, enum class, or
 *   annotation class, local ones included. Objects, companion objects,
 *   anonymous objects, and enum-entry bodies are looked through, as Go's
 *   `class_declaration` walk does. Only the owner's simple name counts, not
 *   its supertypes;
 * - a receiver spelled `GlobalScope` (Go's test) whose value cannot be known,
 *   such as a parameter, a var, or a property with a getter, is reported;
 * - test files are not skipped.
 *
 * Deliberate differences from Go, pinned by goldens and listed in the PR:
 * - Precision: a receiver spelled `GlobalScope` that provably is not
 *   kotlinx.coroutines.GlobalScope is not reported: a local
 *   `object GlobalScope`, or a val named `GlobalScope` initialized with a new
 *   scope (`CoroutineScope(...)`, `MainScope()`, `scope + context`).
 * - Recall: FIR also reports a receiver that is `GlobalScope` under another
 *   spelling (an import or type alias, a val or local initialized with it, a
 *   cast, `!!`, parentheses, backticks, `this`), a backticked class name, and
 *   a builder imported under an alias. Go reads names only.
 * - The message names the resolved builder, so `async` imported as `launch`
 *   reads `GlobalScope.async`, where Go reads `GlobalScope.launch`.
 */
internal object GlobalScopeLaunchInViewModel : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "GlobalScopeLaunchInViewModel"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(GlobalScopeLaunchInViewModel)
    }

    private val coroutines = FqName("kotlinx.coroutines")
    private val globalScope = ClassId.topLevel(FqName("kotlinx.coroutines.GlobalScope"))

    // GlobalScope is an object, so only an expression of one of these types
    // can hold it.
    private val globalScopeHolders = setOf(
        globalScope,
        ClassId.topLevel(FqName("kotlinx.coroutines.CoroutineScope")),
        ClassId.topLevel(FqName("kotlin.Any")),
    )

    // Calls that return a new scope, never GlobalScope itself.
    private val newScopeFactories = listOf("CoroutineScope", "MainScope", "plus")
        .map { CallableId(coroutines, Name.identifier(it)) }
        .toSet()

    private val builders = setOf("launch", "async")
    private val ownerSuffixes = listOf("ViewModel", "Presenter")
    private const val MAX_DEPTH = 8

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val source = expression.source ?: return
        val callee = expression.calleeReference.toResolvedCallableSymbol() ?: return
        val method = callee.name.asString()
        if (method !in builders) return

        // A safe call's explicit receiver is the checked subject, typed as
        // the non-null receiver, so `GlobalScope?.launch {}` counts too.
        val receiver = expression.explicitReceiver ?: return
        when (holdsGlobalScope(receiver, 0)) {
            true -> Unit
            false -> return
            // The value cannot be known: Go's name test decides.
            null -> if (!spelledGlobalScope(expression)) return
        }

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

    /**
     * True when [expression] is kotlinx.coroutines.GlobalScope, false when it
     * provably is not, and null when its value cannot be known: a parameter, a
     * var, a property with a getter, a delegate, or an overridable
     * declaration, or a call that might return GlobalScope. A final val or
     * local val is followed to its initializer, so
     * `val scope: CoroutineScope = GlobalScope` holds GlobalScope.
     */
    context(context: CheckerContext)
    private fun holdsGlobalScope(expression: FirExpression, depth: Int): Boolean? {
        val classId = expression.resolvedType.fullyExpandedType().lowerBoundIfFlexible().classId
        if (classId == globalScope) return true
        if (classId != null && classId !in globalScopeHolders) return false
        if (depth >= MAX_DEPTH) return null
        // A safe call subject's original receiver may be a raw, unresolved
        // access (the resolved one replaced it), which then resolves to no
        // symbol here.
        val current = when (expression) {
            is FirSmartCastExpression -> expression.originalExpression
            is FirCheckedSafeCallSubject -> expression.originalReceiverRef.value
            else -> expression
        }
        return when (current) {
            is FirPropertyAccessExpression -> {
                val symbol = current.calleeReference.toResolvedCallableSymbol() as? FirPropertySymbol ?: return null
                if (!symbol.isVal || symbol.isLateInit || symbol.hasDelegate) return null
                if (!symbol.isLocal && !symbol.isFinal) return null
                if (symbol.getterSymbol?.isDefault == false) return null
                val initializer = symbol.resolvedInitializer ?: return null
                holdsGlobalScope(initializer, depth + 1)
            }
            is FirFunctionCall -> {
                val symbol = current.calleeReference.toResolvedCallableSymbol()
                if (symbol is FirConstructorSymbol || symbol?.callableId in newScopeFactories) false else null
            }
            else -> null
        }
    }

    /**
     * Go's receiver test: the receiver is the identifier `GlobalScope`, or a
     * navigation whose last identifier is `GlobalScope` (`this.GlobalScope`).
     * Backticks, parentheses, and `!!` fail it, as they do in Go. The
     * receiver's text is the call's text before the callee name.
     */
    private fun spelledGlobalScope(call: FirFunctionCall): Boolean {
        val source = call.source ?: return false
        val calleeStart = call.calleeReference.source?.startOffset ?: return false
        val text = source.text ?: return false
        val end = calleeStart - source.startOffset
        if (end <= 0 || end > text.length) return false
        val receiver = text.subSequence(0, end).trimEnd().removeSuffix(".").removeSuffix("?").trimEnd()
        if (!receiver.endsWith(GLOBAL_SCOPE)) return false
        val before = receiver.dropLast(GLOBAL_SCOPE.length)
        return before.isEmpty() || before.trimEnd().endsWith(".")
    }

    private const val GLOBAL_SCOPE = "GlobalScope"
}
