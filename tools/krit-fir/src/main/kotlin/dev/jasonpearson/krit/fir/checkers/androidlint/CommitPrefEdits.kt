package dev.jasonpearson.krit.fir.checkers.androidlint

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.FirSession
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.declarations.FirAnonymousFunction
import org.jetbrains.kotlin.fir.declarations.FirAnonymousInitializer
import org.jetbrains.kotlin.fir.declarations.FirClass
import org.jetbrains.kotlin.fir.declarations.FirConstructor
import org.jetbrains.kotlin.fir.declarations.FirFunction
import org.jetbrains.kotlin.fir.declarations.FirNamedFunction
import org.jetbrains.kotlin.fir.declarations.FirProperty
import org.jetbrains.kotlin.fir.declarations.FirPropertyAccessor
import org.jetbrains.kotlin.fir.declarations.FirRegularClass
import org.jetbrains.kotlin.fir.expressions.FirAnonymousFunctionExpression
import org.jetbrains.kotlin.fir.expressions.FirArgumentList
import org.jetbrains.kotlin.fir.expressions.FirCheckNotNullCall
import org.jetbrains.kotlin.fir.expressions.FirCheckedSafeCallSubject
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirQualifiedAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirSafeCallExpression
import org.jetbrains.kotlin.fir.expressions.FirSmartCastExpression
import org.jetbrains.kotlin.fir.expressions.FirThisReceiverExpression
import org.jetbrains.kotlin.fir.expressions.FirVariableAssignment
import org.jetbrains.kotlin.fir.expressions.FirWrappedArgumentExpression
import org.jetbrains.kotlin.fir.expressions.arguments
import org.jetbrains.kotlin.fir.references.toResolvedBaseSymbol
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.resolve.getContainingClassSymbol
import org.jetbrains.kotlin.fir.resolve.lookupSuperTypes
import org.jetbrains.kotlin.fir.symbols.FirBasedSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirClassSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirNamedFunctionSymbol
import org.jetbrains.kotlin.fir.types.classId
import org.jetbrains.kotlin.fir.types.constructClassLikeType
import org.jetbrains.kotlin.fir.types.isSubtypeOf
import org.jetbrains.kotlin.fir.types.resolvedType
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name

// Flags `SharedPreferences.edit()` whose Editor is never committed or applied,
// so the edits are silently dropped.
//
// Mirrors the Go CommitPrefEdits rule:
// - The call is a zero-argument `edit()`, on an explicit or implicit receiver.
// - A no-argument, no-lambda call named commit or apply that encloses the
//   edit call, up to the enclosing function, finalizes it: the chain
//   `prefs.edit().putString(k, v).apply()`, and (like Go, by name) any other
//   enclosing `x.commit()` / `x.apply()`.
// - An edit call in the initializer of `val editor = ...` is finalized by a
//   later `editor.commit()` / `editor.apply()` anywhere in the enclosing
//   function (nested lambdas included).
// - The finding is on the line where the call expression starts (its
//   receiver, for a chain split across lines), and only inside a function,
//   accessor, or anonymous function: Go reports nothing in a class or
//   top-level property initializer.
//
// Where Go matches any zero-argument call named edit (it has no types without
// the oracle), the checker requires `android.content.SharedPreferences.edit`
// or an override of it. So:
// - An `edit()` of a type that is not a SharedPreferences (a local class, an
//   object) is not reported.
// - The androidx.core KTX `edit { }` is not reported: it applies (or commits)
//   the Editor itself, so the message is false of it.
// Where Go reads the finalizing call by name, the checker reads the resolved
// tree, which also sees:
// - a scope function whose lambda finalizes the Editor through its receiver or
//   parameter: `prefs.edit().apply { putString(k, v); apply() }`,
//   `with(prefs.edit()) { commit() }`, `.also { it.apply() }`;
// - a finalizing chain on the variable: `editor.putString(k, v).apply()`
//   (Go only reads a call written `editor.apply()`);
// - an assignment to an existing variable, `editor = prefs.edit()`, followed
//   by `editor.apply()` (Go only reads `val`/`var` initializers).
// Those are not reported (Go reports them). The variable is matched by symbol,
// not by name, so a finalizing call on another variable with the same name
// does not finalize it (Go takes it). The checker also reports an unfinalized
// edit in an init block or a secondary constructor body, which Go misses (it
// only searches function bodies).
internal object CommitPrefEdits : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "CommitPrefEdits"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(CommitPrefEdits)
    }

    private const val MESSAGE = "SharedPreferences.edit() without commit() or apply()."

    private val EDIT = Name.identifier("edit")
    private val FINALIZERS = setOf(Name.identifier("commit"), Name.identifier("apply"))
    private val KOTLIN = FqName("kotlin")

    // Scope functions whose lambda sees the receiver as `this` or as its
    // parameter; apply and also also return the receiver.
    private val SCOPE_FUNCTIONS = setOf("apply", "also", "run", "let", "with").map(Name::identifier).toSet()
    private val RECEIVER_RETURNING = setOf("apply", "also").map(Name::identifier).toSet()
    private val WITH = Name.identifier("with")

    private val sharedPreferencesId = ClassId(FqName("android.content"), Name.identifier("SharedPreferences"))
    private val editorId = sharedPreferencesId.createNestedClassId(Name.identifier("Editor"))
    private val nullableEditorType = editorId.constructClassLikeType(emptyArray(), isMarkedNullable = true)

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        if (!isSharedPreferencesEdit(expression, context.session)) return
        val path = context.containingElements
        var start = path.lastIndex
        if (path.lastOrNull() === expression) start--
        val scopeIndex = scopeIndex(path, start)
        if (scopeIndex < 0) return
        if (ancestorFinalizes(path, start, expression)) return
        if (flowsToFinalizingScope(path, start, scopeIndex, expression, context.session)) return
        val variable = assignedVariable(path, start, scopeIndex, expression)
        if (variable != null &&
            finalizedLater(path[scopeIndex], variable, expression, context.session)
        ) {
            return
        }
        report(expression.source, MESSAGE)
    }

    // `SharedPreferences.edit()` or an override of it in a SharedPreferences
    // implementation. The owner comes from the symbol's lookup tag, bound for
    // local and anonymous classes.
    private fun isSharedPreferencesEdit(call: FirFunctionCall, session: FirSession): Boolean {
        val callee = call.calleeReference.toResolvedCallableSymbol() as? FirNamedFunctionSymbol ?: return false
        if (callee.name != EDIT) return false
        if (callee.valueParameterSymbols.isNotEmpty() || callee.receiverParameterSymbol != null) return false
        val owner = callee.getContainingClassSymbol() as? FirClassSymbol<*> ?: return false
        return isSubclassOf(owner, sharedPreferencesId, session)
    }

    private fun isSubclassOf(symbol: FirClassSymbol<*>, classId: ClassId, session: FirSession): Boolean =
        symbol.classId == classId ||
            lookupSuperTypes(symbol, lookupInterfaces = true, deep = true, useSiteSession = session)
                .any { it.classId == classId }

    // The index in [path] of the body the edit call runs in: the nearest
    // named function, accessor, or anonymous function (Go's function_declaration
    // or function_body), init block, or secondary constructor. -1 when the call
    // is in a property initializer outside any of them.
    private fun scopeIndex(path: List<FirElement>, start: Int): Int {
        for (i in start downTo 0) {
            when (val element = path[i]) {
                is FirNamedFunction, is FirPropertyAccessor, is FirAnonymousInitializer -> return i
                is FirAnonymousFunction -> if (!element.isLambda) return i
                is FirConstructor -> if (!element.isPrimary) return i
                else -> {}
            }
        }
        return -1
    }

    private fun childOf(path: List<FirElement>, index: Int, expression: FirFunctionCall): FirElement =
        if (index + 1 <= path.lastIndex) path[index + 1] else expression

    // Go: an enclosing call named commit or apply with no arguments and no
    // trailing lambda, whatever it is called on, up to the nearest named
    // function (Go's function_declaration; lambdas, accessors, and anonymous
    // functions do not stop the walk).
    private fun ancestorFinalizes(path: List<FirElement>, start: Int, expression: FirFunctionCall): Boolean {
        for (i in start downTo 0) {
            val parent = path[i]
            val call = when (parent) {
                is FirNamedFunction -> return false
                is FirFunctionCall -> parent
                // `a?.edit()?.apply()`: the finalizing call is the selector of
                // the safe call whose receiver holds the edit call.
                is FirSafeCallExpression ->
                    if (parent.receiver === childOf(path, i, expression)) parent.selector as? FirFunctionCall else null
                else -> null
            } ?: continue
            if (isFinalizerShape(call)) return true
        }
        return false
    }

    private fun isFinalizerShape(call: FirFunctionCall): Boolean =
        call.calleeReference.name in FINALIZERS && call.argumentList.arguments.isEmpty()

    // Follows the Editor from the edit call through Editor-returning calls
    // (`putString`, `apply { }`, `!!`) to a scope function whose lambda
    // commits or applies it.
    private fun flowsToFinalizingScope(
        path: List<FirElement>,
        start: Int,
        scopeIndex: Int,
        expression: FirFunctionCall,
        session: FirSession,
    ): Boolean {
        var current: FirElement = expression
        for (i in start downTo scopeIndex + 1) {
            val parent = path[i]
            val next: FirElement = when (parent) {
                is FirSafeCallExpression -> when {
                    parent.selector === current -> parent
                    parent.receiver === current -> {
                        val selector = parent.selector as? FirFunctionCall ?: return false
                        when (step(selector, session)) {
                            Step.FINALIZED -> return true
                            Step.FORWARDS -> parent
                            Step.STOPS -> null
                        }
                    }
                    else -> null
                }
                is FirCheckNotNullCall, is FirSmartCastExpression, is FirWrappedArgumentExpression -> parent
                is FirFunctionCall -> when {
                    parent.explicitReceiver === current -> when (step(parent, session)) {
                        Step.FINALIZED -> return true
                        Step.FORWARDS -> parent
                        Step.STOPS -> null
                    }
                    isWith(parent) && withSubject(parent) === unwrap(current) ->
                        return lambdaFinalizes(parent, session)
                    else -> null
                }
                // `with(prefs.edit())`: the argument list sits between the
                // call and its argument.
                is FirArgumentList -> current
                else -> null
            } ?: return false
            current = next
        }
        return false
    }

    private enum class Step { FINALIZED, FORWARDS, STOPS }

    // What a call on the Editor does with it.
    private fun step(call: FirFunctionCall, session: FirSession): Step {
        if (isScopeFunction(call)) {
            if (lambdaFinalizes(call, session)) return Step.FINALIZED
            return if (call.calleeReference.name in RECEIVER_RETURNING) Step.FORWARDS else Step.STOPS
        }
        return if (returnsEditor(call, session)) Step.FORWARDS else Step.STOPS
    }

    private fun returnsEditor(call: FirFunctionCall, session: FirSession): Boolean {
        val callee = call.calleeReference.toResolvedCallableSymbol() ?: return false
        val owner = callee.getContainingClassSymbol() as? FirClassSymbol<*> ?: return false
        return isSubclassOf(owner, editorId, session) && call.resolvedType.isSubtypeOf(nullableEditorType, session)
    }

    private fun isScopeFunction(call: FirFunctionCall): Boolean {
        val callee = call.calleeReference.toResolvedCallableSymbol() ?: return false
        return callee.callableId?.packageName == KOTLIN &&
            callee.callableId?.className == null &&
            callee.name in SCOPE_FUNCTIONS &&
            (callee.name == WITH || call.explicitReceiver != null)
    }

    private fun isWith(call: FirFunctionCall): Boolean = isScopeFunction(call) && call.calleeReference.name == WITH

    private fun withSubject(call: FirFunctionCall): FirExpression? =
        call.argumentList.arguments.firstOrNull()?.let(::unwrap)

    private fun unwrap(element: FirElement): FirElement = when (element) {
        is FirWrappedArgumentExpression -> unwrap(element.expression)
        else -> element
    }

    private fun unwrap(expression: FirExpression): FirExpression = when (expression) {
        is FirWrappedArgumentExpression -> unwrap(expression.expression)
        else -> expression
    }

    // True when the scope function's lambda calls commit() or apply() on the
    // Editor it received: through its receiver (`apply`, `run`, `with`) or its
    // parameter (`also`, `let`). Nested lambdas, functions, and classes are not
    // searched: code there may never run.
    private fun lambdaFinalizes(call: FirFunctionCall, session: FirSession): Boolean {
        val lambda = call.argumentList.arguments
            .map(::unwrap)
            .filterIsInstance<FirAnonymousFunctionExpression>()
            .lastOrNull()?.anonymousFunction ?: return false
        val receiverSymbol = lambda.receiverParameter?.symbol
        val parameterSymbol = lambda.valueParameters.firstOrNull()?.symbol
        var found = false
        lambda.body?.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                if (found) return
                if (element is FirFunction || element is FirClass) return
                if (element is FirFunctionCall && isFinalizerShape(element)) {
                    val root = editorRoot(element.explicitReceiver ?: element.dispatchReceiver, session)
                    val bound = when (root) {
                        is FirThisReceiverExpression -> root.calleeReference.boundSymbol.let {
                            it != null && (it == receiverSymbol || it == lambda.symbol)
                        }
                        is FirQualifiedAccessExpression ->
                            parameterSymbol != null && root.calleeReference.toResolvedBaseSymbol() == parameterSymbol
                        else -> false
                    }
                    if (bound) {
                        found = true
                        return
                    }
                }
                element.acceptChildren(this)
            }
        })
        return found
    }

    // The expression a finalizing call's receiver starts from, through safe
    // calls, `!!`, smart casts, and Editor-returning calls
    // (`editor.putString(k, v).apply()`).
    private fun editorRoot(receiver: FirExpression?, session: FirSession): FirExpression? {
        var current = receiver ?: return null
        repeat(64) {
            current = when (val c = current) {
                is FirCheckedSafeCallSubject -> c.originalReceiverRef.value
                is FirSafeCallExpression -> c.selector as? FirExpression ?: return c
                is FirCheckNotNullCall -> c.argumentList.arguments.firstOrNull() ?: return c
                is FirSmartCastExpression -> c.originalExpression
                is FirWrappedArgumentExpression -> c.expression
                is FirFunctionCall -> {
                    val forwards = when {
                        isScopeFunction(c) -> c.calleeReference.name in RECEIVER_RETURNING
                        else -> returnsEditor(c, session)
                    }
                    if (!forwards) return c
                    c.explicitReceiver ?: c.dispatchReceiver ?: return c
                }
                else -> return c
            }
        }
        return current
    }

    // Go's initializerAssignedName: the variable whose initializer holds the
    // edit call (through lambdas and other calls), up to the enclosing
    // function or class. Also an assignment `editor = prefs.edit()`.
    private fun assignedVariable(
        path: List<FirElement>,
        start: Int,
        scopeIndex: Int,
        expression: FirFunctionCall,
    ): FirBasedSymbol<*>? {
        for (i in start downTo scopeIndex + 1) {
            when (val parent = path[i]) {
                is FirProperty ->
                    return if (parent.initializer === childOf(path, i, expression)) parent.symbol else null
                is FirVariableAssignment ->
                    return if (parent.rValue === childOf(path, i, expression)) {
                        (parent.lValue as? FirQualifiedAccessExpression)?.calleeReference?.toResolvedBaseSymbol()
                    } else {
                        null
                    }
                // Go stops at a class or object declaration, not at an object
                // expression.
                is FirNamedFunction, is FirRegularClass -> return null
                else -> {}
            }
        }
        return null
    }

    // Go's functionHasReceiverCallAfter: a call after the edit call anywhere in
    // the enclosing body that commits or applies [variable]'s Editor, directly
    // (`editor.apply()`, `editor?.putString(k, v)?.commit()`) or through a
    // scope function (`editor.apply { apply() }`, `with(editor) { commit() }`).
    private fun finalizedLater(
        scope: FirElement,
        variable: FirBasedSymbol<*>,
        edit: FirFunctionCall,
        session: FirSession,
    ): Boolean {
        val editStart = edit.source?.startOffset ?: return false
        var found = false
        scope.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                if (found) return
                if (element is FirFunctionCall && element !== edit &&
                    (element.source?.startOffset ?: -1) >= editStart &&
                    finalizes(element, variable, session)
                ) {
                    found = true
                    return
                }
                element.acceptChildren(this)
            }
        })
        return found
    }

    private fun finalizes(call: FirFunctionCall, variable: FirBasedSymbol<*>, session: FirSession): Boolean {
        val subject = when {
            isFinalizerShape(call) -> call.explicitReceiver
            isWith(call) -> withSubject(call)
            isScopeFunction(call) -> call.explicitReceiver
            else -> return false
        }
        val root = editorRoot(subject, session) as? FirQualifiedAccessExpression ?: return false
        if (root.calleeReference.toResolvedBaseSymbol() != variable) return false
        return isFinalizerShape(call) || lambdaFinalizes(call, session)
    }
}
