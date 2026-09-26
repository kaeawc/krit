package dev.jasonpearson.krit.fir.checkers.androidlint

import com.intellij.lang.LighterASTNode
import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.lightChildren
import dev.jasonpearson.krit.fir.support.lightSourceOf
import org.jetbrains.kotlin.KtFakeSourceElementKind
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.declarations.FirAnonymousFunction
import org.jetbrains.kotlin.fir.declarations.FirAnonymousInitializer
import org.jetbrains.kotlin.fir.declarations.FirFile
import org.jetbrains.kotlin.fir.declarations.FirFunction
import org.jetbrains.kotlin.fir.declarations.FirNamedFunction
import org.jetbrains.kotlin.fir.declarations.FirProperty
import org.jetbrains.kotlin.fir.declarations.FirRegularClass
import org.jetbrains.kotlin.fir.declarations.FirValueParameter
import org.jetbrains.kotlin.fir.declarations.utils.isCompanion
import org.jetbrains.kotlin.fir.expressions.FirAnonymousFunctionExpression
import org.jetbrains.kotlin.fir.expressions.FirArgumentList
import org.jetbrains.kotlin.fir.expressions.FirBlock
import org.jetbrains.kotlin.fir.expressions.FirCheckNotNullCall
import org.jetbrains.kotlin.fir.expressions.FirCheckedSafeCallSubject
import org.jetbrains.kotlin.fir.expressions.FirElvisExpression
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirImplicitInvokeCall
import org.jetbrains.kotlin.fir.expressions.FirLiteralExpression
import org.jetbrains.kotlin.fir.expressions.FirLoop
import org.jetbrains.kotlin.fir.expressions.FirLoopJump
import org.jetbrains.kotlin.fir.expressions.FirOperation
import org.jetbrains.kotlin.fir.expressions.FirQualifiedAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirReturnExpression
import org.jetbrains.kotlin.fir.expressions.FirSafeCallExpression
import org.jetbrains.kotlin.fir.expressions.FirSmartCastExpression
import org.jetbrains.kotlin.fir.expressions.FirThisReceiverExpression
import org.jetbrains.kotlin.fir.expressions.FirThrowExpression
import org.jetbrains.kotlin.fir.expressions.FirTypeOperatorCall
import org.jetbrains.kotlin.fir.expressions.FirVariableAssignment
import org.jetbrains.kotlin.fir.expressions.FirWhenBranch
import org.jetbrains.kotlin.fir.expressions.FirWhenExpression
import org.jetbrains.kotlin.fir.expressions.FirWrappedArgumentExpression
import org.jetbrains.kotlin.fir.references.FirNamedReference
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.symbols.FirBasedSymbol
import org.jetbrains.kotlin.fir.symbols.SymbolInternals
import org.jetbrains.kotlin.fir.symbols.impl.FirPropertySymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirReceiverParameterSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirValueParameterSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirVariableSymbol
import org.jetbrains.kotlin.fir.types.isNothing
import org.jetbrains.kotlin.fir.types.resolvedType
import org.jetbrains.kotlin.fir.unwrapFakeOverrides
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid
import org.jetbrains.kotlin.lexer.KtToken
import org.jetbrains.kotlin.name.CallableId
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name

// Flags an android.widget.Toast.makeText(...) call whose toast is never shown.
// The toast counts as shown, exactly as in Go, when:
// - an enclosing call named `show` (on any receiver, the toast as its
//   receiver or anywhere in its arguments or lambda) sits between the call
//   and the nearest enclosing named function (`Toast.makeText(..).show()`,
//   `Toast.makeText(..)?.apply { .. }.show()`), except that a toast in the
//   lambda of `show(args) { .. }` does not count (Go's tree hangs that lambda
//   off an outer call with no name);
// - an enclosing call named `apply`, in the same range, has a trailing lambda
//   holding a `show()` call whose receiver is not a name (`show()`,
//   `this.show()`), and no parentheses before it (to Go, `apply(args) { .. }`
//   has no lambda);
// - the call is in the initializer of a property (at any depth, but not
//   across a named function, class, or object), and a call named `show` on a
//   receiver spelled with that property's name (its last identifier:
//   `toast`, `this.toast`) follows the call in the nearest enclosing named
//   function; with no enclosing function, anywhere in the nearest enclosing
//   class or named object (a companion object counts as its outer class).
// Anything else is reported: a toast returned, passed to another function,
// stored in a collection, or created as a bare statement. The finding is on
// the first line of the call, with Go's message.
//
// Deliberate differences from Go, each pinned in the golden data:
// - Recall: the call is identified by resolution, so a fully qualified
//   `android.widget.Toast.makeText(..)`, an import alias or typealias of
//   Toast, and a statically imported `makeText(..)` are reported
//   (ShowToastRecall). Go needs the receiver spelled `Toast`.
// - Precision: a project or local class named Toast is not android.widget.Toast
//   (ShowToastLookalike); Go only drops it when the Kotlin oracle resolves it
//   elsewhere. A toast Go reports although the code shows it is not reported
//   (ShowToastDivergence): shown through a scope function's lambda
//   (`.also { it.show() }`, `.let { it.show() }`, `.run { show() }`,
//   `with(toast) { show() }`, `t.apply { show() }`); through a variable
//   whose `show()` receiver is not spelled with the bare name (`t!!.show()`,
//   `(t as Toast).show()`); through a copy into another variable; through a
//   variable assigned rather than initialized (`t = Toast.makeText(..)`, or
//   an `if` or elvis holding the call, then `t.show()`); through a
//   parameter's default value or a `when` subject variable; through a local
//   shown in a property getter or a top-level lambda; or through a
//   top-level or member property shown from elsewhere in the file. A
//   variable is followed by the declaration its reads resolve to, and only
//   to reads the stored value reaches (Flow.variableShown): a toast
//   overwritten, skipped by a jump or branch, or stored on another instance
//   before the only `show` is still reported, as in Go (ShowToastOverwrite).
internal object ShowToast : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "ShowToast"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(ShowToast)
    }

    private const val MESSAGE = "Toast.makeText() called without .show(). The toast will not be displayed."
    private const val FLOW_BUDGET = 4

    private val toastClassId = ClassId(FqName("android.widget"), Name.identifier("Toast"))
    private val makeText = Name.identifier("makeText")
    private val show = Name.identifier("show")
    private val apply = Name.identifier("apply")

    private val kotlin = FqName("kotlin")
    private val scopeAlso = CallableId(kotlin, Name.identifier("also"))
    private val scopeApply = CallableId(kotlin, Name.identifier("apply"))
    private val scopeLet = CallableId(kotlin, Name.identifier("let"))
    private val scopeRun = CallableId(kotlin, Name.identifier("run"))
    private val scopeWith = CallableId(kotlin, Name.identifier("with"))
    private val scopeTakeIf = CallableId(kotlin, Name.identifier("takeIf"))
    private val scopeTakeUnless = CallableId(kotlin, Name.identifier("takeUnless"))

    private val goCallTypes = setOf(
        KtNodeTypes.CALL_EXPRESSION,
        KtNodeTypes.DOT_QUALIFIED_EXPRESSION,
        KtNodeTypes.SAFE_ACCESS_EXPRESSION,
    )
    private val qualifiedTypes = setOf(KtNodeTypes.DOT_QUALIFIED_EXPRESSION, KtNodeTypes.SAFE_ACCESS_EXPRESSION)

    @OptIn(SymbolInternals::class)
    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val callee = expression.calleeReference.toResolvedCallableSymbol()?.unwrapFakeOverrides() ?: return
        val callableId = callee.callableId ?: return
        if (callableId.callableName != makeText || callableId.classId != toastClassId) return
        val file = context.containingFileSymbol?.fir ?: return
        val path = pathTo(file, expression) ?: return
        if (goShown(expression, path)) return
        if (Flow(file).shown(expression, path, FLOW_BUDGET)) return
        report(expression.source, MESSAGE)
    }

    // The elements enclosing [target] in [root], outermost first; null when
    // [target] is not in [root].
    private fun pathTo(root: FirElement, target: FirElement): List<FirElement>? {
        var found: List<FirElement>? = null
        root.accept(object : FirVisitorVoid() {
            private val ancestors = ArrayList<FirElement>()
            override fun visitElement(element: FirElement) {
                if (found != null) return
                if (element === target) {
                    found = ancestors.toList()
                    return
                }
                ancestors += element
                element.acceptChildren(this)
                ancestors.removeAt(ancestors.lastIndex)
            }
        })
        return found
    }

    // ---- Go's evidence, read from the source as Go reads it ----

    private fun goShown(call: FirFunctionCall, path: List<FirElement>): Boolean {
        // Go walks the ancestors up to the nearest named function.
        val callStart = call.source?.startOffset ?: return false
        for (element in path.asReversed()) {
            if (element is FirNamedFunction) break
            for (ancestor in ancestorCalls(element)) {
                val name = goName(ancestor) ?: continue
                if (name != show && name != apply) continue
                // Go's tree nests `name(args) { .. }`: the lambda hangs off an
                // outer call with no name, and the inner `name(args)` call has
                // no lambda. So a toast in that lambda is under no `show` or
                // `apply` call, and such an `apply` has no lambda to read.
                val lambda = lambdaAfterArguments(ancestor)
                if (name == show) {
                    if (lambda != null && callStart >= lambda.startOffset && callStart < lambda.endOffset) continue
                    return true
                }
                if (lambda == null && trailingLambda(ancestor)?.let(::hasUnnamedShow) == true) return true
            }
        }
        val variable = goAssignedName(call, path) ?: return false
        val start = call.source?.startOffset ?: return false
        val function = path.lastOrNull { it is FirNamedFunction }
        if (function != null) {
            return anyCall(function) { candidate ->
                candidate !== call &&
                    (candidate.source?.startOffset ?: -1) >= start &&
                    goName(candidate) == show &&
                    goReceiverName(candidate) == variable
            }
        }
        val owner = path.lastOrNull { it is FirRegularClass && !it.isCompanion } ?: return false
        return anyCall(owner) { candidate -> goName(candidate) == show && goReceiverName(candidate) == variable }
    }

    // The calls an ancestor stands for: itself, or the selector of a safe
    // call whose receiver holds the toast (Go's tree nests the receiver
    // under the `?.` call).
    private fun ancestorCalls(element: FirElement): List<FirFunctionCall> = when (element) {
        is FirFunctionCall -> listOf(element)
        is FirSafeCallExpression -> listOfNotNull(element.selector as? FirFunctionCall)
        else -> emptyList()
    }

    // The name of the property whose initializer holds [call], stopping at a
    // named function, a class or named object, or the file.
    private fun goAssignedName(call: FirFunctionCall, path: List<FirElement>): String? {
        for (index in path.indices.reversed()) {
            when (val element = path[index]) {
                is FirNamedFunction, is FirRegularClass, is FirFile -> return null
                is FirProperty -> {
                    val child = path.getOrNull(index + 1) ?: call
                    return if (child === element.initializer) element.name.asString() else null
                }
                else -> {}
            }
        }
        return null
    }

    // The callee name as written: for a call written as a Go call expression
    // (not an infix or operator call), the called name; for an invoked
    // property of function type, the property's name.
    private fun goName(call: FirFunctionCall): Name? {
        if (call.source?.elementType !in goCallTypes) return null
        return if (call is FirImplicitInvokeCall) {
            ((call.explicitReceiver as? FirQualifiedAccessExpression)?.calleeReference as? FirNamedReference)?.name
        } else {
            call.calleeReference.name
        }
    }

    // The receiver's name as Go reads it: the identifier, or the last
    // identifier of a qualified receiver (`this.toast`, `a.b`); "" when the
    // receiver is absent or is anything else (`this`, a call, `x!!`, `(x)`).
    private fun goReceiverName(call: FirFunctionCall): String {
        val source = call.source ?: return ""
        val receiverNode = when (source.elementType) {
            in qualifiedTypes -> expressionChildren(source, source.lighterASTNode).firstOrNull()
            else -> {
                // A safe call's selector may carry only its own call's source.
                val subject = call.explicitReceiver as? FirCheckedSafeCallSubject ?: return ""
                val receiverSource = subject.originalReceiverRef.value.source ?: return ""
                return nameOf(receiverSource, receiverSource.lighterASTNode)
            }
        } ?: return ""
        return nameOf(source, receiverNode)
    }

    private fun nameOf(anchor: KtSourceElement, node: LighterASTNode): String = when (node.tokenType) {
        KtNodeTypes.REFERENCE_EXPRESSION -> textOf(anchor, node)
        in qualifiedTypes -> {
            val selector = expressionChildren(anchor, node).lastOrNull()
            if (selector?.tokenType == KtNodeTypes.REFERENCE_EXPRESSION) textOf(anchor, selector) else ""
        }
        else -> ""
    }

    // The element (non-token) children of [node]: expressions, without the
    // dots, whitespace, and comments between them.
    private fun expressionChildren(anchor: KtSourceElement, node: LighterASTNode): List<LighterASTNode> =
        lightChildren(anchor, node).filter { it.tokenType !is KtToken }

    private fun textOf(anchor: KtSourceElement, node: LighterASTNode): String =
        anchor.treeStructure.toString(node).toString().removeSurrounding("`")

    // The call's own CALL_EXPRESSION node: the source itself, or the selector
    // of a qualified call.
    private fun callNode(source: KtSourceElement): LighterASTNode? = when (source.elementType) {
        KtNodeTypes.CALL_EXPRESSION -> source.lighterASTNode
        in qualifiedTypes -> lightChildren(source, source.lighterASTNode)
            .lastOrNull { it.tokenType == KtNodeTypes.CALL_EXPRESSION }
        else -> null
    }

    // The lambda written after the call's parentheses, when the call has both
    // (`show(1) { .. }`, `show() { .. }`).
    private fun lambdaAfterArguments(call: FirFunctionCall): KtSourceElement? {
        val source = call.source ?: return null
        val node = callNode(source) ?: return null
        val children = lightChildren(source, node)
        if (children.none { it.tokenType == KtNodeTypes.VALUE_ARGUMENT_LIST }) return null
        val lambdaNode = children.firstOrNull { it.tokenType == KtNodeTypes.LAMBDA_ARGUMENT } ?: return null
        return lightSourceOf(lambdaNode, source)
    }

    // The lambda written after the call's parentheses (Go's trailing lambda).
    private fun trailingLambda(call: FirFunctionCall): FirAnonymousFunction? {
        val source = call.source ?: return null
        val callNode = callNode(source) ?: return null
        val lambdaNode = lightChildren(source, callNode).firstOrNull { it.tokenType == KtNodeTypes.LAMBDA_ARGUMENT }
            ?: return null
        val range = lightSourceOf(lambdaNode, source)
        return call.argumentList.arguments.firstNotNullOfOrNull { argument ->
            val lambda = unwrapArgument(argument) as? FirAnonymousFunctionExpression ?: return@firstNotNullOfOrNull null
            val start = lambda.source?.startOffset ?: return@firstNotNullOfOrNull null
            lambda.anonymousFunction.takeIf { start >= range.startOffset && start < range.endOffset }
        }
    }

    private fun hasUnnamedShow(lambda: FirAnonymousFunction): Boolean =
        anyCall(lambda) { goName(it) == show && goReceiverName(it) == "" }

    private fun anyCall(root: FirElement, predicate: (FirFunctionCall) -> Boolean): Boolean {
        var found = false
        root.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                if (found) return
                if (element is FirFunctionCall && predicate(element)) {
                    found = true
                    return
                }
                element.acceptChildren(this)
            }
        })
        return found
    }

    private fun unwrapArgument(argument: FirExpression): FirExpression {
        var current = argument
        while (current is FirWrappedArgumentExpression) current = current.expression
        return current
    }

    // ---- Where the toast's value goes ----

    // Follows the toast value from an expression that holds it to a `show`
    // call: through `!!`, casts, smart casts, the receiver of a stdlib
    // scope function (into its lambda's receiver or parameter, and on
    // through `also`/`apply`/`takeIf`/`takeUnless`), and into a variable
    // (initializer or assignment), whose later reads are followed in turn.
    private class Flow(private val file: FirFile) {

        fun shown(occurrence: FirExpression, path: List<FirElement>, budget: Int): Boolean {
            if (budget <= 0) return false
            var current: FirElement = occurrence
            for (parent in path.asReversed()) {
                when {
                    // The argument list sits between a call and its arguments.
                    parent is FirArgumentList -> {}
                    parent is FirSmartCastExpression && parent.originalExpression === current -> current = parent
                    parent is FirWrappedArgumentExpression && parent.expression === current -> current = parent
                    parent is FirCheckNotNullCall && parent.argumentList.arguments.firstOrNull() === current ->
                        current = parent
                    parent is FirTypeOperatorCall &&
                        (parent.operation == FirOperation.AS || parent.operation == FirOperation.SAFE_AS) &&
                        parent.argumentList.arguments.firstOrNull() === current -> current = parent
                    // A branch's value: the last statement of its block, the
                    // `if`/`when` it belongs to, either side of an elvis.
                    parent is FirBlock && parent.statements.lastOrNull() === current -> current = parent
                    parent is FirWhenBranch && parent.result === current -> current = parent
                    parent is FirWhenExpression && current is FirWhenBranch -> current = parent
                    parent is FirElvisExpression && (parent.lhs === current || parent.rhs === current) ->
                        current = parent
                    parent is FirSafeCallExpression && parent.receiver === current -> {
                        val selector = parent.selector as? FirFunctionCall ?: return false
                        when (receiverUse(selector, budget)) {
                            Use.SHOWN -> return true
                            Use.PASSED -> current = parent
                            Use.DROPPED -> return false
                        }
                    }
                    parent is FirFunctionCall && parent !is FirImplicitInvokeCall &&
                        parent.explicitReceiver === current -> when (receiverUse(parent, budget)) {
                        Use.SHOWN -> return true
                        Use.PASSED -> current = parent
                        Use.DROPPED -> return false
                    }
                    parent is FirFunctionCall && scopeId(parent) == scopeWith &&
                        parent.argumentList.arguments.firstOrNull() === current ->
                        return lambdaOf(parent)?.let { receiverShown(it, budget) } == true
                    parent is FirProperty && parent.initializer === current ->
                        return variableShown(parent.symbol, parent, budget)
                    parent is FirValueParameter && parent.defaultValue === current ->
                        return variableShown(parent.symbol, parent, budget)
                    parent is FirVariableAssignment && parent.rValue === current -> {
                        val target = storedSymbol(parent) ?: return false
                        return variableShown(target, parent, budget)
                    }
                    else -> return false
                }
            }
            return false
        }

        private enum class Use { SHOWN, PASSED, DROPPED }

        // What a call on the toast does with it.
        private fun receiverUse(call: FirFunctionCall, budget: Int): Use {
            if (call.calleeReference.name == show) return Use.SHOWN
            val lambda = lambdaOf(call)
            return when (scopeId(call)) {
                scopeAlso, scopeTakeIf, scopeTakeUnless ->
                    if (lambda != null && parameterShown(lambda, budget)) Use.SHOWN else Use.PASSED
                scopeApply -> if (lambda != null && receiverShown(lambda, budget)) Use.SHOWN else Use.PASSED
                scopeLet -> if (lambda != null && parameterShown(lambda, budget)) Use.SHOWN else Use.DROPPED
                scopeRun -> if (lambda != null && receiverShown(lambda, budget)) Use.SHOWN else Use.DROPPED
                else -> Use.DROPPED
            }
        }

        private fun scopeId(call: FirFunctionCall): CallableId? {
            val id = call.calleeReference.toResolvedCallableSymbol()?.callableId ?: return null
            return id.takeIf { it.packageName == kotlin && it.className == null }
        }

        private fun lambdaOf(call: FirFunctionCall): FirAnonymousFunction? =
            call.argumentList.arguments.firstNotNullOfOrNull {
                (unwrapArgument(it) as? FirAnonymousFunctionExpression)?.anonymousFunction
            }

        // The lambda's value parameter (`it` or named) holds the toast.
        private fun parameterShown(lambda: FirAnonymousFunction, budget: Int): Boolean {
            val parameter = lambda.valueParameters.firstOrNull()?.symbol ?: return false
            return anyRead(lambda, { read -> readSymbol(read) == parameter }) { read, path ->
                shown(read, path, budget - 1)
            }
        }

        // The lambda's receiver holds the toast: an implicit or explicit
        // `this.show()` on it, or an explicit `this` whose value is shown.
        private fun receiverShown(lambda: FirAnonymousFunction, budget: Int): Boolean {
            val symbol = lambda.symbol
            fun bound(expression: FirExpression?): Boolean {
                var receiver = expression
                while (receiver is FirSmartCastExpression) receiver = receiver.originalExpression
                val bound = (receiver as? FirThisReceiverExpression)?.calleeReference?.boundSymbol ?: return false
                return bound == symbol || (bound as? FirReceiverParameterSymbol)?.containingDeclarationSymbol == symbol
            }
            if (anyCall(lambda) { it.calleeReference.name == show && (bound(it.dispatchReceiver) || bound(it.extensionReceiver)) }) {
                return true
            }
            return anyRead(lambda, { it is FirThisReceiverExpression && !it.isImplicit && bound(it) }) { read, path ->
                shown(read, path, budget - 1)
            }
        }

        // A read of [variable] holding the value [store] puts there, whose
        // value is shown.
        // - A parameter is never reassigned, and its default value is
        //   computed before the body runs: any read in the file.
        // - A member or top-level property: any read in the file, but only
        //   when [store] is on `this` (or no receiver) and is the property's
        //   only write in the file (a `null` initializer aside). Otherwise
        //   the property is not followed: another write may replace the
        //   toast before any read, or the read may be on another instance.
        // - A local variable: a read the stored value reaches (readReached).
        private fun variableShown(variable: FirVariableSymbol<*>, store: FirElement, budget: Int): Boolean {
            val follow = { read: FirExpression, path: List<FirElement> -> shown(read, path, budget - 1) }
            return when {
                variable is FirValueParameterSymbol -> anyRead(file, { readSymbol(it) == variable }, follow)
                variable is FirPropertySymbol && !variable.isLocal ->
                    onlyStore(variable, store) && anyRead(file, { readSymbol(it) == variable }, follow)
                else -> readReachedShown(variable, store, follow)
            }
        }

        private fun onlyStore(variable: FirPropertySymbol, store: FirElement): Boolean {
            if (store is FirVariableAssignment) {
                val receiver = (store.lValue as? FirQualifiedAccessExpression)?.explicitReceiver
                if (receiver != null && receiver !is FirThisReceiverExpression) return false
            }
            if (store !is FirProperty) {
                val initializer = variable.resolvedInitializer
                if (initializer != null && !(initializer is FirLiteralExpression && initializer.value == null)) return false
            }
            return !visit(file) { element, _ ->
                element is FirVariableAssignment && element !== store && storedSymbol(element) == variable
            }
        }

        // A read of the local [variable] that the value [store] puts there
        // reaches, whose value is shown. The read must come after the store,
        // inside the function, lambda, accessor, or initializer holding the
        // store (a lambda may run later or repeatedly), inside every loop
        // holding the store (the next iteration stores again), and not in
        // another branch of an `if`/`when` whose branch holds the store. No
        // other write of the variable may lie between them, nor a jump
        // (`return`, `throw`, a `break`/`continue` of a loop holding the
        // store, a call returning Nothing) that may skip the read. A write
        // in a lambda or local function may run at any point, so then the
        // variable is not followed at all.
        private fun readReachedShown(
            variable: FirVariableSymbol<*>,
            store: FirElement,
            follow: (FirExpression, List<FirElement>) -> Boolean,
        ): Boolean {
            val storePath = pathTo(file, store) ?: return false
            val scopeIndex = storePath.indexOfLast(::isBody)
            if (scopeIndex < 0) return false
            val scope = storePath[scopeIndex]
            val storeEnd = store.source?.endOffset ?: return false
            val storeChain = storePath + store
            val barriers = ArrayList<KtSourceElement>()
            val capturedWrite = visit(file) { element, path ->
                if (element !is FirVariableAssignment || element === store || storedSymbol(element) != variable) {
                    return@visit false
                }
                element.source?.let(barriers::add)
                path.lastOrNull(::isBody) !== scope
            }
            if (capturedWrite) return false
            visit(scope) { element, path ->
                if (isJump(element, path, storeChain)) element.source?.let(barriers::add)
                false
            }
            val loops = storePath.subList(scopeIndex + 1, storePath.size).filterIsInstance<FirLoop>()
            return anyRead(scope, { readSymbol(it) == variable && (it.source?.startOffset ?: -1) >= storeEnd }) { read, path ->
                val readStart = read.source?.startOffset ?: return@anyRead false
                val readChain = path + read
                loops.all { loop -> path.any { it === loop } } &&
                    !inOtherBranch(storeChain, readChain) &&
                    barriers.none { it.startOffset >= storeEnd && it.endOffset <= readStart } &&
                    follow(read, path)
            }
        }

        private fun isBody(element: FirElement): Boolean = element is FirFunction || element is FirAnonymousInitializer

        // A jump that may leave the code between a store and a read: an
        // explicit `return` from the store's function or an enclosing one (a
        // lambda's own `return@label` only ends the lambda), a `throw`, a call
        // returning Nothing, or a `break`/`continue` of a loop holding the
        // store (other loops' jumps continue after the loop).
        private fun isJump(element: FirElement, path: List<FirElement>, storeChain: List<FirElement>): Boolean =
            when (element) {
                is FirReturnExpression -> element.source?.kind !is KtFakeSourceElementKind &&
                    path.drop(1).none { it === element.target.labeledElement }
                is FirLoopJump -> storeChain.any { it === element.target.labeledElement }
                is FirThrowExpression -> true
                is FirFunctionCall -> element.resolvedType.isNothing
                else -> false
            }

        // The store and the read sit in different branches of one `if`/`when`,
        // the store in its branch's body: taking the store's branch skips the
        // read's.
        private fun inOtherBranch(storeChain: List<FirElement>, readChain: List<FirElement>): Boolean {
            for (index in storeChain.indices) {
                val expression = storeChain[index] as? FirWhenExpression ?: continue
                val storeBranch = storeChain.getOrNull(index + 1) as? FirWhenBranch ?: continue
                if (storeChain.getOrNull(index + 2) !== storeBranch.result) continue
                val readIndex = readChain.indexOfFirst { it === expression }
                if (readIndex < 0) continue
                val readBranch = readChain.getOrNull(readIndex + 1) as? FirWhenBranch ?: continue
                if (readBranch !== storeBranch) return true
            }
            return false
        }

        private fun storedSymbol(assignment: FirVariableAssignment): FirVariableSymbol<*>? =
            (assignment.lValue as? FirQualifiedAccessExpression)?.calleeReference?.toResolvedCallableSymbol()
                ?.unwrapFakeOverrides() as? FirVariableSymbol<*>

        private fun readSymbol(read: FirExpression): FirBasedSymbol<*>? =
            (read as? FirQualifiedAccessExpression)?.calleeReference?.toResolvedCallableSymbol()?.unwrapFakeOverrides()

        // Visits [root] and calls [onRead] with each expression matching
        // [matches] and its ancestors from [root] down; true when one returns
        // true.
        private fun anyRead(
            root: FirElement,
            matches: (FirExpression) -> Boolean,
            onRead: (FirExpression, List<FirElement>) -> Boolean,
        ): Boolean = visit(root) { element, path -> element is FirExpression && matches(element) && onRead(element, path) }

        // Visits [root] and calls [onElement] with each element and its
        // ancestors from [root] down, stopping when it returns true; true when
        // one did. The ancestor list is only valid during the call.
        private fun visit(root: FirElement, onElement: (FirElement, List<FirElement>) -> Boolean): Boolean {
            var found = false
            root.accept(object : FirVisitorVoid() {
                private val ancestors = ArrayList<FirElement>()
                override fun visitElement(element: FirElement) {
                    if (found) return
                    if (onElement(element, ancestors)) {
                        found = true
                        return
                    }
                    ancestors += element
                    element.acceptChildren(this)
                    ancestors.removeAt(ancestors.lastIndex)
                }
            })
            return found
        }
    }
}
