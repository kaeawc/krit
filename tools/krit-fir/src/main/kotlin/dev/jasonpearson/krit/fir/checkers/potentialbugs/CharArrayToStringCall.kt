package dev.jasonpearson.krit.fir.checkers.potentialbugs

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import org.jetbrains.kotlin.contracts.description.LogicOperationKind
import org.jetbrains.kotlin.descriptors.Modality
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.declarations.FirAnonymousInitializer
import org.jetbrains.kotlin.fir.declarations.FirClass
import org.jetbrains.kotlin.fir.declarations.FirFunction
import org.jetbrains.kotlin.fir.declarations.FirProperty
import org.jetbrains.kotlin.fir.expressions.FirBlock
import org.jetbrains.kotlin.fir.expressions.FirBooleanOperatorExpression
import org.jetbrains.kotlin.fir.expressions.FirDesugaredAssignmentValueReferenceExpression
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirLoop
import org.jetbrains.kotlin.fir.expressions.FirOperation
import org.jetbrains.kotlin.fir.expressions.FirPropertyAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirQualifiedAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirSmartCastExpression
import org.jetbrains.kotlin.fir.expressions.FirThisReceiverExpression
import org.jetbrains.kotlin.fir.expressions.FirTypeOperatorCall
import org.jetbrains.kotlin.fir.expressions.FirVariableAssignment
import org.jetbrains.kotlin.fir.expressions.FirWhenBranch
import org.jetbrains.kotlin.fir.expressions.FirWhenExpression
import org.jetbrains.kotlin.fir.expressions.argument
import org.jetbrains.kotlin.fir.expressions.unwrapSmartcastExpression
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.references.toResolvedVariableSymbol
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.symbols.FirBasedSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirPropertySymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirValueParameterSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirVariableSymbol
import org.jetbrains.kotlin.fir.types.ConeClassLikeType
import org.jetbrains.kotlin.fir.types.ConeDefinitelyNotNullType
import org.jetbrains.kotlin.fir.types.ConeIntersectionType
import org.jetbrains.kotlin.fir.types.ConeKotlinType
import org.jetbrains.kotlin.fir.types.coneType
import org.jetbrains.kotlin.fir.types.isNothing
import org.jetbrains.kotlin.fir.types.lowerBoundIfFlexible
import org.jetbrains.kotlin.fir.types.resolvedType
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid
import org.jetbrains.kotlin.name.CallableId
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.name.SpecialNames
import org.jetbrains.kotlin.name.StandardClassIds

// Flags `toString()` called with no arguments on a `kotlin.CharArray`, which
// renders the array's identity (`[C@1b6d3586`), not its characters.
//
// Mirrors the Go CharArrayToStringCall rule: a zero-argument call named
// `toString` whose receiver is a CharArray (a parameter or property typed
// `CharArray` / `CharArray?` or through a type alias, a `charArrayOf(...)`
// initializer, a direct `charArrayOf(...)` receiver, or a name smart-cast by
// `is CharArray`), reported on the line where the call expression, receiver
// included, starts.
//
// The callee must be the real `toString`: the member inherited from
// `kotlin.Any` (spelled on `Any` or `CharArray`) or the stdlib extension
// `Any?.toString()` a nullable receiver resolves to. The receiver is the
// value `toString` is called on (the written receiver, else the implicit
// one); its type counts as CharArray through type aliases, platform
// (`CharArray!`) types, `CharArray?`, and smart casts.
//
// K2 drops a stable smart cast the call does not need, and Any.toString()
// applies to the original type, so `if (x is CharArray) x.toString()` reaches
// the checker with `x` typed as declared. The is-check that proves the value
// is a CharArray is recovered from the enclosing code, in the shapes Go's
// resolver narrows: the body of an `if`/`when` branch whose condition (or one
// of its `&&` operands) is `x is CharArray`, a `when (x) { is CharArray -> }`
// branch, and the statements after `if (x !is CharArray [|| ...]) <exit>`,
// plus the right operand of `x is CharArray && ...`. `x` is a bare name or
// `this.x`, and the check must still hold at the call:
// - a parameter, a local `val`, or a final member or top-level `val` without
//   a custom getter or delegate cannot change;
// - a local `var` must not be assigned where the assignment can run between
//   the check and the call: in the function, lambda, or initializer that
//   runs the check, between them in the source or in a loop around the call
//   but not the check; elsewhere in the var's scope, in a lambda, local
//   function, or local class created before the call or in a loop around it;
// - any other value (a member `var`, an `open` val, a val with a custom
//   getter, a delegate) is one K2 cannot smart-cast, and K2 then keeps the
//   unstable smart cast on the receiver; it must still be to CharArray, so a
//   reassignment in between drops the finding. Go reports all of these.
//
// Deliberate differences from Go, pinned by goldens:
// - Precision: Go types the receiver by looking its text up by name in scope
//   (the text after the last `.`), then falls back to any same-named
//   parameter or property anywhere in the file declared `CharArray` or
//   initialized with a `charArrayOf` call, and it matches the type by its
//   simple name. A receiver that is not a kotlin.CharArray (a lambda parameter
//   that shares a name with such a declaration, a project class named
//   `CharArray`, another object's property that shares a name with a CharArray
//   parameter, the `else` branch of an `is CharArray` check, a check under
//   `||`, an `if (x !is CharArray && ...) return` guard, a `when` branch with
//   several is-checks, a statement before `if (x !is CharArray) return` or
//   after an `if` that does not always exit, a var reassigned after the check,
//   another object's property with the checked name) is not reported, nor is
//   a call to a project `CharArray?.toString()` extension, which does not
//   render the array's identity.
// - Recall: a CharArray receiver Go cannot type by name is reported: a call
//   result (`"a".toCharArray()`, a Java `char[]`), `x!!` (also on a map
//   value), an indexed element, a parenthesized receiver, `this`, an implicit
//   receiver (`toString()` inside a `CharArray` extension or
//   `with(chars) { ... }`), a val bound from `as? CharArray ?: return`. So is
//   a value proven a CharArray where Go does not narrow: a subject-less
//   `when { x is CharArray -> }` branch, the right operand of
//   `x is CharArray && ...`, `this.x`, the statements after
//   `if (x !is CharArray) continue`, the branches of `return if (...)`, and a
//   local var in a getter of an `object {}` member.
internal object CharArrayToStringCall : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "CharArrayToStringCall"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(CharArrayToStringCall)
    }

    private const val MESSAGE =
        "Calling toString() on a CharArray does not return the string representation. Use String(charArray) instead."

    private const val MAX_TYPE_DEPTH = 8

    private val charArray = ClassId(FqName("kotlin"), Name.identifier("CharArray"))
    private val toString = Name.identifier("toString")
    private val toStringCallees = setOf(
        CallableId(StandardClassIds.Any, toString),
        CallableId(charArray, toString),
        CallableId(FqName("kotlin"), toString),
    )

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        if (expression.argumentList.arguments.isNotEmpty()) return
        val callee = expression.calleeReference.toResolvedCallableSymbol() ?: return
        if (callee.callableId !in toStringCallees) return
        val receiver = expression.explicitReceiver
            ?: expression.extensionReceiver
            ?: expression.dispatchReceiver
            ?: return
        if (!isCharArray(receiver.resolvedType, 0) && !provenByIsCheck(expression, receiver)) return
        report(expression.source, MESSAGE)
    }

    context(context: CheckerContext)
    private fun isCharArray(type: ConeKotlinType, depth: Int): Boolean {
        if (depth > MAX_TYPE_DEPTH) return false
        return when (val bound = type.fullyExpandedType().lowerBoundIfFlexible()) {
            is ConeDefinitelyNotNullType -> isCharArray(bound.original, depth + 1)
            is ConeIntersectionType -> bound.intersectedTypes.any { isCharArray(it, depth + 1) }
            is ConeClassLikeType -> bound.lookupTag.classId == charArray
            else -> false
        }
    }

    // A value, identified by its variable and the `this` it is read from
    // (null for locals, parameters, and top-level properties).
    private data class ValueKey(val variable: FirVariableSymbol<*>, val owner: FirBasedSymbol<*>?)

    // The is-check that proves the value, found at [index] in the containing
    // elements; [exitBranch] is the result of an early-exit `if`, which never
    // reaches the call.
    private class Proof(val check: FirTypeOperatorCall, val index: Int, val exitBranch: FirElement?)

    // The value [expression] reads, when it is a plain read of a variable,
    // bare or through `this`; null for any other expression.
    private fun valueKey(expression: FirExpression): ValueKey? {
        val access = expression.unwrapSmartcastExpression() as? FirPropertyAccessExpression ?: return null
        val variable = access.calleeReference.toResolvedVariableSymbol() ?: return null
        val explicit = access.explicitReceiver
        if (explicit != null && explicit !is FirThisReceiverExpression) return null
        val owner = when (val dispatch = access.dispatchReceiver) {
            null -> null
            is FirThisReceiverExpression -> dispatch.calleeReference.boundSymbol ?: return null
            else -> return null
        }
        if (access.extensionReceiver != null) return null
        return ValueKey(variable, owner)
    }

    // Whether an enclosing is-check proves [receiver] of [call] is a
    // CharArray.
    context(context: CheckerContext)
    private fun provenByIsCheck(call: FirFunctionCall, receiver: FirExpression): Boolean {
        val key = valueKey(receiver) ?: return false
        val proof = findProof(key) ?: return false
        return when {
            isLocalVar(key.variable) -> !assignedBetween(key.variable, proof, call)
            isStable(key.variable) -> true
            // K2 keeps a smart cast it cannot rely on, so the receiver shows
            // whether the value is still known to be a CharArray here.
            else -> receiver is FirSmartCastExpression && isCharArray(receiver.smartcastType.coneType, 0)
        }
    }

    context(context: CheckerContext)
    private fun findProof(key: ValueKey): Proof? {
        val path = context.containingElements
        for (i in path.indices.reversed()) {
            val child = path.getOrNull(i + 1) ?: continue
            when (val element = path[i]) {
                is FirWhenBranch ->
                    if (child === element.result) {
                        provingCheck(element.condition, key, path.getOrNull(i - 1) as? FirWhenExpression)
                            ?.let { return Proof(it, i, null) }
                    }
                is FirBooleanOperatorExpression ->
                    if (element.kind == LogicOperationKind.AND && child === element.rightOperand) {
                        provingCheck(element.leftOperand, key, null)?.let { return Proof(it, i, null) }
                    }
                is FirBlock -> earlyExitCheck(element, child, key, i)?.let { return it }
                else -> {}
            }
        }
        return null
    }

    // The `x is CharArray` that [condition] asserts, directly or as an `&&`
    // operand; [owner] is the `when` whose branch it guards, whose subject
    // the check may test.
    context(context: CheckerContext)
    private fun provingCheck(condition: FirExpression, key: ValueKey, owner: FirWhenExpression?): FirTypeOperatorCall? =
        when (condition) {
            is FirBooleanOperatorExpression ->
                if (condition.kind == LogicOperationKind.AND) {
                    provingCheck(condition.leftOperand, key, owner) ?: provingCheck(condition.rightOperand, key, owner)
                } else {
                    null
                }
            is FirTypeOperatorCall -> condition.takeIf { typeCheck(it, FirOperation.IS, key, owner) }
            else -> null
        }

    // The `x !is CharArray` that [condition] refutes when it is false,
    // directly or as an `||` operand: an exit guarded by it leaves `x` a
    // CharArray.
    context(context: CheckerContext)
    private fun guardCheck(condition: FirExpression, key: ValueKey): FirTypeOperatorCall? =
        when (condition) {
            is FirBooleanOperatorExpression ->
                if (condition.kind == LogicOperationKind.OR) {
                    guardCheck(condition.leftOperand, key) ?: guardCheck(condition.rightOperand, key)
                } else {
                    null
                }
            is FirTypeOperatorCall -> condition.takeIf { typeCheck(it, FirOperation.NOT_IS, key, null) }
            else -> null
        }

    // A `x is CharArray` (or `!is`, per [operation]) check of the value
    // [key], read directly or as the subject of [owner].
    context(context: CheckerContext)
    private fun typeCheck(call: FirTypeOperatorCall, operation: FirOperation, key: ValueKey, owner: FirWhenExpression?): Boolean {
        if (call.operation != operation) return false
        if (!isCharArray(call.conversionTypeRef.coneType, 0)) return false
        val tested = valueKey(call.argument) ?: return false
        if (tested == key) return true
        // `when (x) { is CharArray -> }` tests a synthetic subject variable
        // initialized with `x`.
        val subject = owner?.subjectVariable ?: return false
        if (tested.variable != subject.symbol || subject.name != SpecialNames.WHEN_SUBJECT) return false
        return subject.initializer?.let(::valueKey) == key
    }

    // An `if (x !is CharArray [|| ...]) <exit>` statement in [block] before
    // [child], the element at [index + 1] in the containing elements.
    context(context: CheckerContext)
    private fun earlyExitCheck(block: FirBlock, child: FirElement, key: ValueKey, index: Int): Proof? {
        for (statement in block.statements) {
            if (statement === child) return null
            val exit = statement as? FirWhenExpression ?: continue
            val first = exit.branches.firstOrNull() ?: continue
            val check = guardCheck(first.condition, key) ?: continue
            if (first.result.resolvedType.isNothing) return Proof(check, index, first.result)
        }
        return null
    }

    private fun isLocalVar(variable: FirVariableSymbol<*>): Boolean =
        variable is FirPropertySymbol && variable.isLocal && !variable.isVal && !variable.hasDelegate

    // Values that cannot change after the check: parameters, local vals, and
    // final vals read through their own field.
    private fun isStable(variable: FirVariableSymbol<*>): Boolean = when (variable) {
        is FirValueParameterSymbol -> true
        is FirPropertySymbol -> when {
            !variable.isVal || variable.hasDelegate -> false
            variable.isLocal -> true
            variable.resolvedStatus.modality != Modality.FINAL -> false
            else -> variable.getterSymbol.let { it == null || it.isDefault }
        }
        else -> false
    }

    // Whether the local var [variable] is assigned where the assignment can
    // run between the check of [proof] and [call].
    //
    // The check and the call run in one flow, the innermost function, lambda,
    // or initializer that holds the check. An assignment in that flow runs
    // between them when it lies between them in the source, or in a loop
    // around the call but not the check. An assignment in any other function,
    // lambda, or local class of the declaring scope runs when that code is
    // invoked, which can happen between the check and the call if the code
    // was created before the call or in a loop around it. The code of the
    // declaring scope and of the functions between it and the checking flow
    // is suspended while the flow runs, so it cannot run in between.
    context(context: CheckerContext)
    private fun assignedBetween(variable: FirVariableSymbol<*>, proof: Proof, call: FirFunctionCall): Boolean {
        val checkStart = proof.check.source?.startOffset ?: return true
        val callStart = call.source?.startOffset ?: return true
        val path = context.containingElements
        val rootIndex = declaringScope(variable, path) ?: return true
        val root = path[rootIndex]
        val flowIndex = (proof.index downTo rootIndex).firstOrNull { isFlow(path[it]) } ?: return true
        val flow = path[flowIndex]
        val enclosing = path.subList(rootIndex, flowIndex + 1)
        // Loops around the call inside the declaring scope; those deeper
        // than the proof do not contain the check.
        val callLoops = (rootIndex + 1 until path.size).filter { path[it] is FirLoop }
        val loopsAroundCall = callLoops.map { path[it] }
        val loopsAroundCallOnly = callLoops.filter { it > proof.index }.map { path[it] }
        var assigned = false
        root.accept(object : FirVisitorVoid() {
            // Start of the outermost function, lambda, or local class being
            // visited that does not enclose the checking flow; null outside
            // such code.
            private var nestedStart: Int? = null
            private var inFlow = root === flow
            private var inLoopAroundCall = 0
            private var inLoopAroundCallOnly = 0
            private var inExitBranch = 0

            override fun visitElement(element: FirElement) {
                if (assigned) return
                if (element is FirVariableAssignment && assignedVariable(element) == variable && invalidates(element)) {
                    assigned = true
                    return
                }
                val enclosingStart = nestedStart
                val enclosingInFlow = inFlow
                if (element === flow) {
                    inFlow = true
                } else if (nestedStart == null && (isFlow(element) || element is FirClass) &&
                    enclosing.none { it === element }
                ) {
                    nestedStart = element.source?.startOffset ?: -1
                }
                val aroundCall = loopsAroundCall.any { it === element }
                val aroundCallOnly = loopsAroundCallOnly.any { it === element }
                val exitBranch = element === proof.exitBranch
                if (aroundCall) inLoopAroundCall++
                if (aroundCallOnly) inLoopAroundCallOnly++
                if (exitBranch) inExitBranch++
                element.acceptChildren(this)
                if (exitBranch) inExitBranch--
                if (aroundCallOnly) inLoopAroundCallOnly--
                if (aroundCall) inLoopAroundCall--
                nestedStart = enclosingStart
                inFlow = enclosingInFlow
            }

            private fun invalidates(assignment: FirVariableAssignment): Boolean {
                // Code created before the call, or in a loop around it, may be
                // invoked between the check and the call.
                nestedStart?.let { return it < callStart || inLoopAroundCall > 0 }
                if (!inFlow) return false
                if (inLoopAroundCallOnly > 0) return true
                val at = assignment.source?.startOffset ?: return true
                return inExitBranch == 0 && at > checkStart && at < callStart
            }
        })
        return assigned
    }

    private fun isFlow(element: FirElement): Boolean = element is FirFunction || element is FirAnonymousInitializer

    // The index in [path] of the innermost function, lambda, or initializer
    // that declares the local [variable].
    private fun declaringScope(variable: FirVariableSymbol<*>, path: List<FirElement>): Int? {
        for (i in path.indices.reversed()) {
            val scope = path[i]
            if (!isFlow(scope)) continue
            if (declares(scope, variable)) return i
        }
        return null
    }

    private fun declares(scope: FirElement, variable: FirVariableSymbol<*>): Boolean {
        var found = false
        scope.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                if (found) return
                if (element is FirProperty && element.symbol == variable) {
                    found = true
                    return
                }
                element.acceptChildren(this)
            }
        })
        return found
    }

    private fun assignedVariable(assignment: FirVariableAssignment): FirVariableSymbol<*>? {
        val lValue = assignment.lValue
        val target = if (lValue is FirDesugaredAssignmentValueReferenceExpression) lValue.expressionRef.value else lValue
        return (target as? FirQualifiedAccessExpression)?.calleeReference?.toResolvedVariableSymbol()
    }
}
