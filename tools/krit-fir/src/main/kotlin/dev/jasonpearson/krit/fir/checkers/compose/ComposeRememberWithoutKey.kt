package dev.jasonpearson.krit.fir.checkers.compose

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.unwrapArgument
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.FirSession
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.types.ConeKotlinType
import org.jetbrains.kotlin.fir.types.classId
import org.jetbrains.kotlin.fir.types.isSomeFunctionType
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.fir.declarations.FirProperty
import org.jetbrains.kotlin.fir.declarations.FirNamedFunction
import org.jetbrains.kotlin.fir.expressions.FirVariableAssignment
import org.jetbrains.kotlin.text
import dev.jasonpearson.krit.fir.support.hasAnnotationNamed
import dev.jasonpearson.krit.fir.support.simpleName
import org.jetbrains.kotlin.descriptors.Visibilities
import org.jetbrains.kotlin.fir.declarations.FirFile
import org.jetbrains.kotlin.fir.declarations.FirRegularClass
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirQualifiedAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirWhenExpression
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirValueParameterSymbol
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid
import org.jetbrains.kotlin.name.FqName

// Flags `remember { f(param) }`: a keyless remember whose calculation captures a
// parameter of the enclosing function. Because the memo has no keys, the cached
// value never recomputes when that parameter changes across recomposition, so
// the composable renders stale data.
//
// A keyless remember that captures nothing external (`remember { mutableStateOf(0) }`,
// `remember { Paint() }`) is the correct, idiomatic form and must not be
// flagged. This mirrors the Go ComposeRememberWithoutKey rule, which fires
// only when the calculation references an enclosing function parameter, and
// like Go it only looks at @Composable functions. Also like Go, it stays quiet
// when the calculation seeds local state (`mutableStateOf(param)`), when the
// parameter is named `initial*` / `default*`, or when the function re-assigns
// the remembered variable from the parameter outside the remember call (an
// assignment or a LaunchedEffect/SideEffect/DisposableEffect), and it reports
// every captured parameter that none of those excuse, plus any factory-like parameter
// the calculation also reads, since the key list must name every captured value. A function-typed or
// State/LazyListState/ScrollState/PagerState parameter read only inside
// derivedStateOf or snapshotFlow is tracked there and is not reported either. A function-typed parameter that
// the calculation only invokes (`remember { Holder(create()) }`) is a one-shot initializer and is skipped
// entirely, including from the key list. A deferred read such as `remember { Holder({ nav.toolbar }) }` still
// captures the first-composition `nav`, so it stays reported.
internal object ComposeRememberWithoutKey : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "ComposeRememberWithoutKey"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(ComposeRememberWithoutKey)
    }
    private val rememberFqName = FqName("androidx.compose.runtime.remember")
    private val composable = setOf("Composable")
    private val trackingCalls = setOf("derivedStateOf", "snapshotFlow", "movableContentOf", "rememberUpdatedState")
    private val keyFqName = FqName("androidx.compose.runtime.key")
    private val trackedStateTypes = setOf(
        "State", "LazyListState", "LazyGridState", "LazyStaggeredGridState", "ScrollState", "PagerState", "TextFieldState",
    )
    private val effectCalls = setOf("LaunchedEffect", "SideEffect", "DisposableEffect")
    private val stateSeedingFactories = setOf(
        "mutableStateOf", "mutableIntStateOf", "mutableFloatStateOf", "mutableLongStateOf",
        "mutableDoubleStateOf", "mutableStateListOf", "mutableStateMapOf", "toMutableStateList",
        "toMutableStateMap",
    )

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val source = expression.source ?: return

        val callee = expression.calleeReference.toResolvedCallableSymbol() ?: return
        if (callee.callableId?.asSingleFqName() != rememberFqName) return

        // remember(vararg keys, calculation) — a single argument is the
        // keyless overload; any keys mean recomposition is already handled.
        val argument = expression.argumentList.arguments.singleOrNull() ?: return

        val function = nearestEnclosingFunction() ?: return
        // Only composables recompose, so a keyless remember elsewhere cannot go stale.
        if (!function.hasAnnotationNamed(composable)) return
        // Preview and testing scaffolding keeps its first-composition values on purpose, as in Go.
        if (isScaffolding(function)) return
        // `if (k != null) remember(k) { } else remember { }`: the keyless arm is the fallback for no key.
        if (isUnkeyedFallbackArm(function, source)) return
        val keyed = keyBlockParams(function, source)
        // A Circuit Navigator lives as long as the presenter, so it never goes stale.
        val enclosingParams = function.symbol.valueParameterSymbols
            .filter { it !in keyed && !isNavigator(it.resolvedReturnType, context.session) }
            .toSet()
        if (enclosingParams.isEmpty()) return

        // Walk the whole calculation argument, not just a lambda body: this
        // also covers a bound callable reference (`remember(model::toString)`),
        // whose receiver captures the enclosing parameter just as a lambda
        // capture would.
        val calculation = unwrapArgument(argument)
        // A function-typed parameter that is only invoked builds a one-shot holder; keying on it would rebuild
        // the object because callers pass a fresh lambda on each recomposition.
        val captured = capturedEnclosingParams(calculation, enclosingParams, context.session)
            .filterNot { isInvokeOnlyFactory(calculation, it, context.session) }
        if (captured.isEmpty()) return
        // A state-seeding factory copies the parameter into local state on purpose.
        if (seedsMutableState(calculation)) return
        val variable = context.containingElements.filterIsInstance<FirProperty>().lastOrNull()?.name?.asString().orEmpty()
        // A comment saying the first-composition capture is intended excuses the call, as in Go.
        if (documentedFirstCapture(source, variable)) return
        // A factory parameter does not trigger a report alone, but it is a captured name the key list must keep.
        val excused = captured.filter { param ->
            val name = param.name.asString()
            name.startsWith("initial") || name.startsWith("default") || resyncsStateFromParam(function, source, variable, name)
        }.toSet()
        val factories = captured.filter { it !in excused && isInjectedFactory(calculation, it, context.session) }.toSet()
        if (captured.none { it !in excused && it !in factories }) return
        val names = captured.filter { it !in excused }.map { it.name.asString() }
        val list = names.joinToString(", ")
        val noun = if (names.size == 1) "parameter" else "parameters"
        val verb = if (names.size == 1) "${names[0]} changes" else "they change"

        report(source, "remember { } reads enclosing $noun $list but has no keys; the cached value won't update when $verb. Pass remember($list) { ... }.")
    }

    private fun isPreviewOrSample(function: FirNamedFunction): Boolean {
        val lower = function.name.asString().lowercase()
        return listOf("preview", "sample", "fixture").any { lower.startsWith(it) || lower.endsWith(it) } ||
            function.annotations.any { it.simpleName()?.endsWith("Preview") == true }
    }

    // True for a preview or sample composable, a @VisibleForTesting one, one in a Fake/Test class, or a private
    // one that only preview functions in this file call.
    context(context: CheckerContext)
    private fun isScaffolding(function: FirNamedFunction): Boolean {
        val enclosing = context.containingElements
        if (enclosing.filterIsInstance<FirNamedFunction>().any(::isPreviewOrSample)) return true
        if (function.hasAnnotationNamed(setOf("VisibleForTesting"))) return true
        val className = enclosing.filterIsInstance<FirRegularClass>().lastOrNull()?.name?.asString().orEmpty()
        if (className.startsWith("Fake") || className.contains("Test")) return true
        if (function.status.visibility != Visibilities.Private) return false
        val file = enclosing.filterIsInstance<FirFile>().firstOrNull() ?: return false
        var calls = 0
        var fromPreview = 0
        file.accept(object : FirVisitorVoid() {
            private val functions = ArrayDeque<FirNamedFunction>()
            override fun visitElement(element: FirElement) {
                if (element is FirNamedFunction) functions.addLast(element)
                if (element is FirFunctionCall && element.calleeReference.toResolvedCallableSymbol() == function.symbol) {
                    calls++
                    if (functions.any(::isPreviewOrSample)) fromPreview++
                }
                element.acceptChildren(this)
                if (element is FirNamedFunction) functions.removeLast()
            }
        })
        return calls > 0 && calls == fromPreview
    }

    private val firstCapturePhrases = listOf(
        "first composition", "initial composition", "captured once", "capture once", "only once", "on purpose",
        "intentional", "initial value", "initial position", "initial state",
    )

    // True when a comment inside the call, or directly above its statement, names the capture as intended.
    context(context: CheckerContext)
    private fun documentedFirstCapture(source: KtSourceElement, variable: String): Boolean {
        fun says(text: String) = firstCapturePhrases.any { text.lowercase().contains(it) }
        val inside = source.text.toString()
        if (Regex("//[^\\n]*|/\\*[\\s\\S]*?\\*/").findAll(inside).any { says(it.value) }) return true
        val fileText = context.containingFile?.getContentsAsStream()?.readBytes()?.decodeToString() ?: return false
        val statementStart = context.containingElements.filterIsInstance<FirProperty>().lastOrNull()
            ?.takeIf { variable.isNotEmpty() }?.source?.startOffset?.coerceAtMost(source.startOffset) ?: source.startOffset
        val lines = fileText.substring(0, statementStart.coerceIn(0, fileText.length)).lines().dropLast(1).asReversed()
        for (line in lines) {
            val trimmed = line.trim()
            val isComment = trimmed.startsWith("//") || trimmed.startsWith("/*") || trimmed.startsWith("*") || trimmed.endsWith("*/")
            if (!isComment) return false
            if (says(trimmed)) return true
        }
        return false
    }

    // True for a function-typed parameter whose every read in the calculation is an invocation, as in
    // `remember { Holder(create()) }`. A lambda that escapes or is read without a call does not count.
    private fun isInvokeOnlyFactory(calculation: FirElement, param: FirValueParameterSymbol, session: FirSession): Boolean {
        if (!param.resolvedReturnType.fullyExpandedType(session).isSomeFunctionType(session)) return false
        var reads = 0
        var invokes = 0
        calculation.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                if (element is FirQualifiedAccessExpression && element.calleeReference.toResolvedCallableSymbol() == param) {
                    reads++
                    return
                }
                if (element is FirFunctionCall) {
                    val calleeName = element.calleeReference.name.asString()
                    // `create()` keeps the written name `create` as its callee but receives the parameter as the invoke receiver.
                    if (calleeName == "invoke" || calleeName == param.name.asString()) {
                        val receiver = element.explicitReceiver
                        if (receiver is FirQualifiedAccessExpression && receiver.calleeReference.toResolvedCallableSymbol() == param) invokes++
                    }
                }
                element.acceptChildren(this)
            }
        })
        return reads > 0 && reads == invokes
    }

    // True for a factory-like parameter the calculation only calls methods on.
    private fun isInjectedFactory(calculation: FirElement, param: FirValueParameterSymbol, session: FirSession): Boolean {
        val label = (param.name.asString() + " " + param.resolvedReturnType.fullyExpandedType(session).classId?.shortClassName).lowercase()
        if (listOf("factory", "provider", "creator").none { it in label }) return false
        var reads = 0
        var receivers = 0
        calculation.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                if (element is FirQualifiedAccessExpression && element.calleeReference.toResolvedCallableSymbol() == param) {
                    reads++
                    return
                }
                if (element is FirFunctionCall) {
                    val receiver = element.explicitReceiver
                    if (receiver is FirQualifiedAccessExpression && receiver.calleeReference.toResolvedCallableSymbol() == param) receivers++
                }
                element.acceptChildren(this)
            }
        })
        return reads > 0 && reads == receivers
    }

    // True when an if/when branch holding [remember] has a sibling branch that calls remember with keys.
    private fun isUnkeyedFallbackArm(function: FirNamedFunction, remember: KtSourceElement): Boolean {
        var found = false
        function.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                if (found) return
                if (element is FirWhenExpression) {
                    val branches = element.branches
                    val own = branches.filter { b -> b.result.source?.let { it.startOffset <= remember.startOffset && remember.endOffset <= it.endOffset } == true }
                    if (own.size == 1 && branches.any { it !== own[0] && callsKeyedRemember(it.result) }) {
                        found = true
                        return
                    }
                }
                element.acceptChildren(this)
            }
        })
        return found
    }

    private fun callsKeyedRemember(root: FirElement): Boolean {
        var keyed = false
        root.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                if (keyed) return
                if (element is FirFunctionCall && element.calleeReference.toResolvedCallableSymbol()?.callableId?.asSingleFqName() == rememberFqName &&
                    element.argumentList.arguments.size > 1
                ) {
                    keyed = true
                    return
                }
                element.acceptChildren(this)
            }
        })
        return keyed
    }

    // The parameters named in the key arguments of every key(...) block that surrounds the remember call.
    private fun keyBlockParams(function: FirNamedFunction, remember: KtSourceElement): Set<FirValueParameterSymbol> {
        val out = HashSet<FirValueParameterSymbol>()
        function.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                if (element is FirFunctionCall && element.calleeReference.toResolvedCallableSymbol()?.callableId?.asSingleFqName() == keyFqName &&
                    element.source?.let { it.startOffset <= remember.startOffset && remember.endOffset <= it.endOffset } == true
                ) {
                    element.argumentList.arguments.dropLast(1).forEach { arg ->
                        arg.accept(object : FirVisitorVoid() {
                            override fun visitElement(inner: FirElement) {
                                if (inner is FirQualifiedAccessExpression) {
                                    (inner.calleeReference.toResolvedCallableSymbol() as? FirValueParameterSymbol)?.let(out::add)
                                }
                                inner.acceptChildren(this)
                            }
                        })
                    }
                }
                element.acceptChildren(this)
            }
        })
        return out
    }

    // The function that lexically contains the remember call: the nearest
    // enclosing NAMED function. Anonymous functions (Compose content lambdas
    // such as `Column { ... }`) also appear in containingDeclarations, so
    // filtering FirFunctionSymbol would pick the wrapping lambda (no value
    // parameters) and silently miss the capture. FirNamedFunctionSymbol mirrors
    // the Go rule's nearest "function_declaration" anchor. Null when there is
    // no enclosing named function.
    context(context: CheckerContext)
    private fun nearestEnclosingFunction(): FirNamedFunction? =
        context.containingElements.filterIsInstance<FirNamedFunction>().lastOrNull()

    // True when the calculation calls a factory that seeds local state.
    private fun seedsMutableState(calculation: FirElement): Boolean {
        var seeds = false
        calculation.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                if (seeds) return
                if (element is FirFunctionCall && element.calleeReference.name.asString() in stateSeedingFactories) {
                    seeds = true
                    return
                }
                element.acceptChildren(this)
            }
        })
        return seeds
    }

    // True when the function re-assigns state from [param] outside the remember
    // call, in an assignment or a LaunchedEffect/SideEffect/DisposableEffect
    // that mentions both the remembered variable and the parameter.
    private fun resyncsStateFromParam(function: FirNamedFunction, remember: KtSourceElement, variable: String, param: String): Boolean {
        val word = Regex("\\b" + Regex.escape(param) + "\\b")
        var found = false
        function.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                if (found) return
                val isResync = element is FirVariableAssignment ||
                    (element is FirFunctionCall && element.calleeReference.name.asString() in effectCalls)
                val text = if (isResync) element.source?.takeIf { it.kind is KtRealSourceElementKind }?.text?.toString() else null
                val inside = element.source?.let { it.startOffset >= remember.startOffset && it.endOffset <= remember.endOffset } == true
                if (text != null && !inside && word.containsMatchIn(text) && (variable.isEmpty() || variable in text)) {
                    found = true
                    return
                }
                element.acceptChildren(this)
            }
        })
        return found
    }

    // The distinct parameters the depth-first walk of the calculation argument
    // resolves, in visit order. For a callable reference the referenced callee
    // is not a capture, but its receiver expression is walked as a child and
    // matches. Symbol identity handles shadowing: a nested lambda parameter
    // with the same name is a different symbol and does not match.
    private fun capturedEnclosingParams(
        argument: FirElement,
        enclosing: Set<FirValueParameterSymbol>,
        session: FirSession,
    ): List<FirValueParameterSymbol> {
        val found = LinkedHashSet<FirValueParameterSymbol>()
        val reads = HashMap<FirValueParameterSymbol, Int>()
        val tracked = HashMap<FirValueParameterSymbol, Int>()
        var trackingDepth = 0
        argument.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                if (element is FirQualifiedAccessExpression) {
                    val symbol = element.calleeReference.toResolvedCallableSymbol()
                    if (symbol is FirValueParameterSymbol && symbol in enclosing) {
                        found += symbol
                        reads.merge(symbol, 1, Int::plus)
                        if (trackingDepth > 0) tracked.merge(symbol, 1, Int::plus)
                        return
                    }
                }
                val tracking = element is FirFunctionCall && element.calleeReference.name.asString() in trackingCalls
                if (tracking) trackingDepth++
                element.acceptChildren(this)
                if (tracking) trackingDepth--
            }
        })
        // A function-typed or state-holder parameter read only inside derivedStateOf or snapshotFlow is
        // tracked by that block and never goes stale, as in Go.
        return found.filterNot { param ->
            reads[param] == tracked[param] && isTrackedKind(param.resolvedReturnType, session)
        }
    }

    private fun isNavigator(type: ConeKotlinType, session: FirSession): Boolean =
        type.fullyExpandedType(session).classId?.shortClassName?.asString() == "Navigator"

    private fun isTrackedKind(type: ConeKotlinType, session: FirSession): Boolean {
        val expanded = type.fullyExpandedType(session)
        return expanded.isSomeFunctionType(session) || expanded.classId?.shortClassName?.asString() in trackedStateTypes
    }
}
