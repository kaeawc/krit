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
import org.jetbrains.kotlin.fir.declarations.FirValueParameter
import org.jetbrains.kotlin.fir.expressions.FirAnonymousFunctionExpression
import org.jetbrains.kotlin.fir.expressions.FirArgumentList
import org.jetbrains.kotlin.fir.expressions.FirBlock
import org.jetbrains.kotlin.fir.expressions.FirCheckNotNullCall
import org.jetbrains.kotlin.fir.expressions.FirCheckedSafeCallSubject
import org.jetbrains.kotlin.fir.expressions.FirDelegatedConstructorCall
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirOperation
import org.jetbrains.kotlin.fir.expressions.FirQualifiedAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirSafeCallExpression
import org.jetbrains.kotlin.fir.expressions.FirSmartCastExpression
import org.jetbrains.kotlin.fir.expressions.FirThisReceiverExpression
import org.jetbrains.kotlin.fir.expressions.FirTypeOperatorCall
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
import org.jetbrains.kotlin.fir.symbols.impl.FirPropertySymbol
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
// does not finalize it (Go takes it), and neither does a call on a member with
// the same name (`this.editor.apply()`, Go matches the last name). A cast of
// the variable (`(editor as? Editor)?.apply()`) still finalizes it.
// The checker also reports an unfinalized edit in an init block or a secondary
// constructor body outside any function, which Go misses (it only searches
// function bodies), but only where the Editor stays local: a bare statement or
// chain, or a local variable. An Editor stored in a member property, passed to
// a call, or handed to a scope function there may be committed by any member,
// and an edit call in a secondary constructor's delegation call or parameter
// defaults is handed to another constructor; none of those is reported (nor by
// Go). In a class declared inside a function, Go reports all of them, and so
// does the checker.
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
        val scope = path[scopeIndex]
        // Go's recall region: an init block or a secondary constructor with
        // no function around it.
        val outsideFunctions = (scope is FirAnonymousInitializer || scope is FirConstructor) &&
            (0 until scopeIndex).none { isFunctionScope(path[it]) }
        if (outsideFunctions && scope is FirConstructor && inConstructorHeader(path, start, scopeIndex)) return
        if (ancestorFinalizes(path, start, expression)) return
        if (flowsToFinalizingScope(path, start, scopeIndex, expression, context.session)) return
        val variable = assignedVariable(path, start, scopeIndex, expression)
        if (variable != null &&
            finalizedLater(path[scopeIndex], variable, expression, context.session)
        ) {
            return
        }
        if (outsideFunctions && !staysLocal(path, start, scopeIndex, expression, context.session)) return
        report(expression.source, MESSAGE)
    }

    // A secondary constructor's delegation call (`: this(p.edit())`) or a
    // parameter default: the Editor is handed to a constructor, as in the
    // class header forms Go leaves alone.
    private fun inConstructorHeader(path: List<FirElement>, start: Int, scopeIndex: Int): Boolean =
        (scopeIndex + 1..start).any { path[it] is FirDelegatedConstructorCall || path[it] is FirValueParameter }

    // In an init block or a secondary constructor body, whether the Editor
    // stays in the block: a statement (through Editor-returning calls, `!!`,
    // and casts), a call on it that does not hand it on, or a local
    // variable. Anything else (a member property, an argument, a scope
    // function) may reach a member that commits it.
    private fun staysLocal(
        path: List<FirElement>,
        start: Int,
        scopeIndex: Int,
        expression: FirFunctionCall,
        session: FirSession,
    ): Boolean {
        var current: FirElement = expression
        for (i in start downTo scopeIndex + 1) {
            when (val parent = path[i]) {
                is FirBlock -> return true
                is FirProperty -> return parent.initializer === current && parent.symbol.isLocal
                is FirVariableAssignment -> {
                    if (parent.rValue !== current) return false
                    val symbol = (parent.lValue as? FirQualifiedAccessExpression)
                        ?.calleeReference?.toResolvedBaseSymbol()
                    return symbol is FirPropertySymbol && symbol.isLocal
                }
                is FirSafeCallExpression -> when {
                    parent.selector === current -> {}
                    parent.receiver === current -> {
                        val selector = parent.selector as? FirFunctionCall ?: return false
                        if (!returnsEditor(selector, session)) return !isScopeFunction(selector)
                    }
                    else -> return false
                }
                is FirFunctionCall -> {
                    if (parent.explicitReceiver !== current) return false
                    if (!returnsEditor(parent, session)) return !isScopeFunction(parent)
                }
                is FirCheckNotNullCall, is FirSmartCastExpression, is FirArgumentList -> {}
                is FirTypeOperatorCall -> if (!isCast(parent)) return false
                else -> return false
            }
            current = path[i]
        }
        return false
    }

    private fun isCast(call: FirTypeOperatorCall): Boolean =
        call.operation == FirOperation.AS || call.operation == FirOperation.SAFE_AS

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
            val element = path[i]
            if (isFunctionScope(element) || element is FirAnonymousInitializer) return i
            if (element is FirConstructor && !element.isPrimary) return i
        }
        return -1
    }

    private fun isFunctionScope(element: FirElement): Boolean =
        element is FirNamedFunction || element is FirPropertyAccessor ||
            (element is FirAnonymousFunction && !element.isLambda)

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
    // calls, `!!`, casts, smart casts, and Editor-returning calls
    // (`editor.putString(k, v).apply()`).
    private fun editorRoot(receiver: FirExpression?, session: FirSession): FirExpression? {
        var current = receiver ?: return null
        repeat(64) {
            current = when (val c = current) {
                is FirCheckedSafeCallSubject -> c.originalReceiverRef.value
                is FirSafeCallExpression -> c.selector as? FirExpression ?: return c
                is FirCheckNotNullCall -> c.argumentList.arguments.firstOrNull() ?: return c
                is FirSmartCastExpression -> c.originalExpression
                is FirTypeOperatorCall -> {
                    if (!isCast(c)) return c
                    c.argumentList.arguments.singleOrNull() ?: return c
                }
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
