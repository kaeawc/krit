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
import org.jetbrains.kotlin.fir.declarations.FirFunction
import org.jetbrains.kotlin.fir.expressions.FirBlock
import org.jetbrains.kotlin.fir.expressions.FirBooleanOperatorExpression
import org.jetbrains.kotlin.fir.expressions.FirDesugaredAssignmentValueReferenceExpression
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirOperation
import org.jetbrains.kotlin.fir.expressions.FirPropertyAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirQualifiedAccessExpression
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
// K2 drops a smart cast the call does not need, and Any.toString() applies to
// the original type, so `if (x is CharArray) x.toString()` reaches the checker
// with `x` typed as declared. The is-check that proves the value is a
// CharArray is recovered from the enclosing code, in the shapes Go's resolver
// narrows: the body of an `if`/`when` branch whose condition (or one of its
// `&&` operands) is `x is CharArray`, a `when (x) { is CharArray -> }`
// branch, and the statements after `if (x !is CharArray) return`, plus the
// right operand of `x is CharArray && ...`. `x` must be a value K2 can
// smart-cast: a parameter, a local `val`, a local `var` not assigned after
// the check or inside a lambda or local function, or a final member or
// top-level `val` without a custom getter or delegate.
//
// Deliberate differences from Go, pinned by goldens:
// - Precision: Go types the receiver by looking its text up by name in scope,
//   then falls back to any same-named parameter or property anywhere in the
//   file declared `CharArray` or initialized with a `charArrayOf` call, and it
//   matches the type by its simple name. A receiver that is not a
//   kotlin.CharArray (a lambda parameter that shares a name with such a
//   declaration, a project class named `CharArray`, the `else` branch of an
//   `is CharArray` check, a check under `||`, a statement before
//   `if (x !is CharArray) return`, a var reassigned after the check, another
//   object's property with the checked name) is not reported, nor is a call to a
//   project `CharArray?.toString()` extension, which does not render the
//   array's identity.
// - Recall: a CharArray receiver Go cannot type by name is reported: a call
//   result (`"a".toCharArray()`, a Java `char[]`), `x!!`, a parenthesized
//   receiver, `this`, an implicit receiver (`toString()` inside a `CharArray`
//   extension or `with(chars) { ... }`), a subject-less `when { x is
//   CharArray -> }` branch, and the right operand of `x is CharArray && ...`.
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
        if (!isCharArray(receiver.resolvedType, 0) && !provenByIsCheck(receiver)) return
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

    // Whether an enclosing is-check proves [receiver] is a CharArray.
    context(context: CheckerContext)
    private fun provenByIsCheck(receiver: FirExpression): Boolean {
        val key = valueKey(receiver) ?: return false
        if (!isSmartCastable(key.variable)) return false
        val path = context.containingElements
        for (i in path.indices.reversed()) {
            val child = path.getOrNull(i + 1) ?: continue
            val check = when (val element = path[i]) {
                is FirWhenBranch ->
                    if (child === element.result) {
                        provingCheck(element.condition, key, path.getOrNull(i - 1) as? FirWhenExpression)
                    } else {
                        null
                    }
                is FirBooleanOperatorExpression ->
                    if (element.kind == LogicOperationKind.AND && child === element.rightOperand) {
                        provingCheck(element.leftOperand, key, null)
                    } else {
                        null
                    }
                is FirBlock -> earlyExitCheck(element, child, key)
                else -> null
            } ?: continue
            return !isLocalVar(key.variable) || !assignedAfter(key.variable, check)
        }
        return false
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

    // An `if (x !is CharArray) <exit>` statement in [block] before [child].
    context(context: CheckerContext)
    private fun earlyExitCheck(block: FirBlock, child: FirElement, key: ValueKey): FirTypeOperatorCall? {
        for (statement in block.statements) {
            if (statement === child) return null
            val exit = statement as? FirWhenExpression ?: continue
            val first = exit.branches.firstOrNull() ?: continue
            val check = first.condition as? FirTypeOperatorCall ?: continue
            if (!typeCheck(check, FirOperation.NOT_IS, key, null)) continue
            if (first.result.resolvedType.isNothing) return check
        }
        return null
    }

    private fun isLocalVar(variable: FirVariableSymbol<*>): Boolean =
        variable is FirPropertySymbol && variable.isLocal && !variable.isVal

    // Values K2 smart-casts: parameters, local vals and vars (vars are
    // checked for assignments separately), and final vals read through their
    // own field.
    private fun isSmartCastable(variable: FirVariableSymbol<*>): Boolean = when (variable) {
        is FirValueParameterSymbol -> true
        is FirPropertySymbol -> when {
            variable.isLocal -> !variable.hasDelegate
            !variable.isVal || variable.hasDelegate -> false
            variable.resolvedStatus.modality != Modality.FINAL -> false
            else -> variable.getterSymbol.let { it == null || it.isDefault }
        }
        else -> false
    }

    // Whether the local var [variable] is assigned after [check] or inside a
    // lambda or local function, which can run between the check and the call.
    context(context: CheckerContext)
    private fun assignedAfter(variable: FirVariableSymbol<*>, check: FirTypeOperatorCall): Boolean {
        val checkOffset = check.source?.startOffset ?: return true
        val root = context.containingElements.firstOrNull { it is FirFunction } ?: return true
        var assigned = false
        root.accept(object : FirVisitorVoid() {
            private var nested = 0

            override fun visitElement(element: FirElement) {
                if (assigned) return
                if (element is FirVariableAssignment &&
                    assignedVariable(element) == variable &&
                    (nested > 0 || (element.source?.startOffset ?: Int.MAX_VALUE) > checkOffset)
                ) {
                    assigned = true
                    return
                }
                val boundary = element is FirFunction && element !== root
                if (boundary) nested++
                element.acceptChildren(this)
                if (boundary) nested--
            }
        })
        return assigned
    }

    private fun assignedVariable(assignment: FirVariableAssignment): FirVariableSymbol<*>? {
        val lValue = assignment.lValue
        val target = if (lValue is FirDesugaredAssignmentValueReferenceExpression) lValue.expressionRef.value else lValue
        return (target as? FirQualifiedAccessExpression)?.calleeReference?.toResolvedVariableSymbol()
    }
}
