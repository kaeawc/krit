package dev.jasonpearson.krit.fir.checkers.androidlint

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.checkers.bodyInCheckedFile
import dev.jasonpearson.krit.fir.checkers.followableInitializer
import dev.jasonpearson.krit.fir.report
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.declarations.FirClass
import org.jetbrains.kotlin.fir.declarations.FirFunction
import org.jetbrains.kotlin.fir.declarations.FirProperty
import org.jetbrains.kotlin.fir.declarations.utils.isConst
import org.jetbrains.kotlin.fir.expressions.FirBlock
import org.jetbrains.kotlin.fir.expressions.FirDesugaredAssignmentValueReferenceExpression
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirLiteralExpression
import org.jetbrains.kotlin.fir.expressions.FirLoop
import org.jetbrains.kotlin.fir.expressions.FirPropertyAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirQualifiedAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirStatement
import org.jetbrains.kotlin.fir.expressions.FirVariableAssignment
import org.jetbrains.kotlin.fir.expressions.resolvedArgumentMapping
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.references.toResolvedVariableSymbol
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.resolve.getContainingClassSymbol
import org.jetbrains.kotlin.fir.resolve.lookupSuperTypes
import org.jetbrains.kotlin.fir.symbols.impl.FirClassLikeSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirNamedFunctionSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirPropertySymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirValueParameterSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirVariableSymbol
import org.jetbrains.kotlin.fir.types.ConeKotlinType
import org.jetbrains.kotlin.fir.types.classId
import org.jetbrains.kotlin.fir.types.constructClassLikeType
import org.jetbrains.kotlin.fir.types.isSubtypeOf
import org.jetbrains.kotlin.fir.types.lowerBoundIfFlexible
import org.jetbrains.kotlin.fir.unwrapFakeOverrides
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.name.StandardClassIds
import org.jetbrains.kotlin.types.ConstantValueKind

/**
 * Flags an `AlarmManager.setRepeating(type, triggerAtMillis, intervalMillis, operation)`
 * call as Android Lint's ShortAlarm does: a trigger time below 5 000 ms or an
 * interval below 60 000 ms. Only `setRepeating` is checked (not
 * `setInexactRepeating`), only its four-parameter overload, and any known
 * value below the minimum counts, zero and negative values included. The
 * finding is on the offending argument, with Lint's message
 * "Value will be forced up to N as of Android 5.1; don't rely on this to be exact".
 *
 * Like the Go rule, the argument must be a known number: an integer literal
 * (`5000L`, `30000`), not a parameter, a variable, a call, or an expression.
 *
 * Go matches the call by name alone, whatever its receiver. The message is
 * about the alarm the call sets, so this checker reports a call whose
 * resolved callee sets a platform repeating alarm from that argument: a
 * member of `android.app.AlarmManager` (or a subclass); an extension on
 * AlarmManager whose parameter is a Long; a function whose body it
 * cannot read (Java, a library, an interface member, or a function of another
 * file: a class file keeps no body) whose signature looks like an alarm API
 * (a `PendingIntent` or `AlarmManager.OnAlarmListener` parameter, or an
 * `AlarmManager` receiver or parameter), as Go does; or a Kotlin wrapper of the
 * same file whose body passes that parameter into the same argument of one of
 * these, directly or through a local or member `var` it may have stored the
 * parameter in (ShortAlarmForwarding).
 *
 * Deliberate differences from Go, each pinned in the golden data:
 * - Precision: a project scheduler (also one named `AlarmManager`), a local
 *   or anonymous class, a top-level function, or an extension whose body
 *   never passes the parameter to an AlarmManager argument (an empty
 *   lookalike, a retry count, a wrapper that sets a fixed hourly alarm, a
 *   wrapper that overwrites the stored parameter with a fixed interval before
 *   the call) sets no alarm value from the literal, so it is not reported
 *   (ShortAlarmLookalike, ShortAlarmThirdParty, ShortAlarmForwarding). That holds
 *   for a lookalike declared in the file under check; one of another file shows
 *   no body, so only its signature is read (a class file keeps it): it is reported when that looks like an
 *   alarm API (a wrapper with a PendingIntent parameter), not otherwise (an unrelated
 *   `Poller.setRepeating(name, delay, intervalMs, task: Runnable)`).
 * - Recall: FIR reads the literal's value, so a value Go cannot parse is
 *   reported: a literal with underscores (`5_000L`), a hex or binary literal,
 *   a parenthesized literal, and a reference to a Kotlin `const val` whose
 *   value is such a literal (ShortAlarmRecall).
 * - Tree-sitter joins a statement that starts with `(` to the previous line's
 *   call, so Go reports the earlier line and misses the later one; FIR reads
 *   the statements as Kotlin does (ShortAlarmDivergence).
 */
internal object ShortAlarm : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "ShortAlarm"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(ShortAlarm)
    }

    private const val PARAMETER_COUNT = 4

    // The trigger time (index 1) and the interval (index 2) with the minimum Lint enforces for each.
    private class Bound(val index: Int, val minimum: Long)
    private val bounds = listOf(Bound(1, 5_000L), Bound(2, 60_000L))
    private val alarmManager = ClassId(FqName("android.app"), Name.identifier("AlarmManager"))
    private val pendingIntent = ClassId(FqName("android.app"), Name.identifier("PendingIntent"))
    private val onAlarmListener = ClassId(FqName("android.app"), FqName("AlarmManager.OnAlarmListener"), false)
    private val alarmManagerType = alarmManager.constructClassLikeType(emptyArray(), isMarkedNullable = true)
    private val repeatingSetter = Name.identifier("setRepeating")
    private val integralKinds = setOf(
        ConstantValueKind.Int,
        ConstantValueKind.Long,
        ConstantValueKind.Short,
        ConstantValueKind.Byte,
        ConstantValueKind.IntegerLiteral,
    )

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val callee = expression.calleeReference.toResolvedCallableSymbol() as? FirNamedFunctionSymbol ?: return
        if (callee.name != repeatingSetter || callee.valueParameterSymbols.size != PARAMETER_COUNT) return
        for (bound in bounds) {
            val argument = argumentFor(expression, callee, bound.index) ?: continue
            val millis = integralValue(argument, HashSet()) ?: continue
            if (millis >= bound.minimum) continue
            if (!setsAlarmValue(callee, bound.index, HashSet())) continue
            val source = argument.source ?: continue
            report(source, "Value will be forced up to ${bound.minimum} as of Android 5.1; don't rely on this to be exact")
        }
    }

    // The argument bound to the callee's value parameter at [index].
    private fun argumentFor(call: FirFunctionCall, callee: FirNamedFunctionSymbol, index: Int): FirExpression? {
        val parameter = callee.valueParameterSymbols.getOrNull(index) ?: return null
        return call.resolvedArgumentMapping?.entries?.firstOrNull { it.value.symbol == parameter }?.key
    }

    // Whether a call to [function] (named like a repeating setter) sets a
    // platform repeating alarm whose interval comes from its third parameter:
    // - a member of AlarmManager or a subclass;
    // - an extension on AlarmManager (a subtype, or nullable) whose third
    //   parameter is a Long, as a project helper around the member is;
    // - a function whose body FIR cannot read (Java, a library, an abstract
    //   or interface member): Go reports it and nothing shows the message is
    //   false, so it is reported too;
    // - a Kotlin function whose body passes its third parameter into the
    //   interval of a call that is one of these (a wrapper, however deep).
    // A function whose body never passes the parameter on (an empty
    // lookalike, a retry count) sets no alarm interval from it.
    context(context: CheckerContext)
    private fun setsAlarmValue(function: FirNamedFunctionSymbol, index: Int, seen: MutableSet<FirNamedFunctionSymbol>): Boolean {
        val original = function.unwrapFakeOverrides()
        val parameter = original.valueParameterSymbols.getOrNull(index) ?: return false
        val receiverType = original.resolvedReceiverType
        if (receiverType == null) {
            val owner = original.getContainingClassSymbol()
            if (owner != null && isAlarmManager(owner)) return true
        } else if (isLong(parameter.resolvedReturnType) && receiverType.isSubtypeOf(alarmManagerType, context.session)) {
            return true
        }
        if (!seen.add(original)) return false
        val body = bodyOf(original) ?: return looksLikeAlarmApi(original)
        return passesToValue(ParameterFlow(body, parameter), index, seen)
    }

    // Whether a function whose body cannot be read (another file, a library, Java, an abstract member) has the
    // signature of an alarm API: a value parameter of type PendingIntent or AlarmManager.OnAlarmListener, or an
    // AlarmManager as its dispatch receiver, extension receiver or a parameter. Only signature facts are read, which
    // a class file keeps, so a slice compile and a full compile agree.
    context(context: CheckerContext)
    private fun looksLikeAlarmApi(function: FirNamedFunctionSymbol): Boolean {
        val owner = function.getContainingClassSymbol()
        if (owner != null && isAlarmManager(owner)) return true
        function.resolvedReceiverType?.let { if (it.isSubtypeOf(alarmManagerType, context.session)) return true }
        return function.valueParameterSymbols.any { parameter ->
            val type = parameter.resolvedReturnType.fullyExpandedType().lowerBoundIfFlexible()
            type.classId == pendingIntent || type.classId == onAlarmListener ||
                type.isSubtypeOf(alarmManagerType, context.session)
        }
    }

    // The body of a wrapper declared in the file under check. A function of another file has no readable body
    // (a class file keeps none), so it is treated like a library function, as Go treats any function of the name.
    context(context: CheckerContext)
    private fun bodyOf(function: FirNamedFunctionSymbol): FirBlock? = function.bodyInCheckedFile()

    // Whether the function body holds a repeating-setter call (see
    // [setsAlarmValue]) whose interval argument may carry the parameter's
    // value (see [ParameterFlow.carries]).
    context(context: CheckerContext)
    private fun passesToValue(flow: ParameterFlow, index: Int, seen: MutableSet<FirNamedFunctionSymbol>): Boolean {
        var found = false
        walkStatements(flow.body, emptyList()) { element, slots ->
            found = element is FirFunctionCall && forwardsValue(element, index, slots, flow, seen)
            found
        }
        return found
    }

    context(context: CheckerContext)
    private fun forwardsValue(
        call: FirFunctionCall,
        index: Int,
        slots: List<Slot>,
        flow: ParameterFlow,
        seen: MutableSet<FirNamedFunctionSymbol>,
    ): Boolean {
        val callee = call.calleeReference.toResolvedCallableSymbol() as? FirNamedFunctionSymbol ?: return false
        if (callee.name != repeatingSetter || callee.valueParameterSymbols.size != PARAMETER_COUNT) return false
        val argument = argumentFor(call, callee, index) ?: return false
        return flow.carries(argument, slots, HashSet()) && setsAlarmValue(callee, index, seen)
    }

    // One step on the way from a function body down to an element: the block
    // and the index of its statement that holds the element. [loop] is set
    // when a loop lies between the enclosing slot's statement and [block], so
    // a write later in that loop can reach the element on the next iteration;
    // [deferred] when a function or class does, so the element may run after
    // any write in the body.
    private class Slot(val block: FirBlock, val index: Int, val loop: Boolean, val deferred: Boolean)

    // Visits [root] and everything under it with the statement slots of each
    // element, [base] first. Stops once [visit] returns true.
    private fun walkStatements(root: FirElement, base: List<Slot>, visit: (FirElement, List<Slot>) -> Boolean) {
        val slots = base.toMutableList()
        var loop = false
        var deferred = false
        var stopped = false
        root.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                if (stopped) return
                if (visit(element, slots)) {
                    stopped = true
                    return
                }
                val outerLoop = loop
                val outerDeferred = deferred
                if (element is FirBlock) {
                    loop = false
                    deferred = false
                    element.statements.forEachIndexed { index, statement ->
                        slots.add(Slot(element, index, outerLoop, outerDeferred))
                        statement.accept(this)
                        slots.removeAt(slots.lastIndex)
                    }
                } else {
                    if (element is FirLoop) loop = true
                    if (element is FirFunction || element is FirClass) deferred = true
                    element.acceptChildren(this)
                }
                loop = outerLoop
                deferred = outerDeferred
            }
        })
    }

    // A write of a tracked variable: the value written and its position.
    private class Definition(val value: FirExpression, val slots: List<Slot>)

    // Tracks the interval parameter of a wrapper through the variables of
    // its body. Go reports the wrapper's caller whatever the body does, so
    // a value FIR cannot prove free of the parameter counts as carrying it.
    private class ParameterFlow(val body: FirBlock, val parameter: FirValueParameterSymbol) {
        // Whether [expression], evaluated at [slots], may carry the
        // parameter's value: it reads the parameter (directly or in an
        // expression such as `seconds * 1000L`), or a local variable or a
        // member `var` that a write reaching [slots] may have set from it
        // (`interval = intervalMillis` before `am.setRepeating(t, a,
        // interval, op)`). The parameter is matched by symbol, so a shadowing
        // name in a lambda or local function does not count.
        fun carries(expression: FirExpression, slots: List<Slot>, visited: MutableSet<FirExpression>): Boolean {
            var found = false
            expression.accept(object : FirVisitorVoid() {
                override fun visitElement(element: FirElement) {
                    if (found) return
                    if (element is FirPropertyAccessExpression) {
                        val symbol = element.calleeReference.toResolvedCallableSymbol()
                        if (symbol == parameter ||
                            symbol is FirPropertySymbol && tracked(symbol) &&
                            reaching(symbol, slots).any { visited.add(it.value) && carries(it.value, it.slots, visited) }
                        ) {
                            found = true
                            return
                        }
                    }
                    element.acceptChildren(this)
                }
            })
            return found
        }

        // A variable the body can assign: a local, or a member `var`. A
        // delegated property's value comes from its delegate.
        private fun tracked(symbol: FirPropertySymbol): Boolean =
            (symbol.isLocal || !symbol.isVal) && !symbol.hasDelegate

        // The writes of [variable] whose value may be the one read at
        // [slots]. Walking back through the statements before it, a plain
        // statement that declares or assigns the variable is the last write
        // and ends the walk; a write nested in an earlier statement (a branch,
        // a lambda) may or may not run, so it counts and the walk goes on. A
        // write in an enclosing loop may reach the next iteration, so every
        // write in that loop counts. Inside a lambda, a local function, or a
        // local class the order of the body says nothing about when the
        // element runs, so every write in the body counts. With no last write
        // in the body, a member keeps the value it had on entry, which the
        // body did not set.
        private fun reaching(variable: FirPropertySymbol, slots: List<Slot>): List<Definition> {
            if (slots.any { it.deferred }) return writes(body, emptyList(), variable)
            val definitions = mutableListOf<Definition>()
            for (depth in slots.indices.reversed()) {
                val slot = slots[depth]
                val prefix = slots.subList(0, depth)
                if (slot.loop && depth > 0) {
                    val holder = slots[depth - 1]
                    definitions += writes(holder.block.statements[holder.index], prefix, variable)
                }
                for (index in slot.index - 1 downTo 0) {
                    val statement = slot.block.statements[index]
                    val here = prefix + Slot(slot.block, index, slot.loop, slot.deferred)
                    val last = lastWrite(statement, variable)
                    if (last != null) {
                        // A declaration without an initializer writes nothing.
                        writtenValue(last)?.let { definitions += Definition(it, here) }
                        return definitions
                    }
                    definitions += writes(statement, here, variable)
                }
            }
            return definitions
        }

        // [statement] itself when it declares or assigns [variable].
        private fun lastWrite(statement: FirStatement, variable: FirPropertySymbol): FirStatement? = when {
            statement is FirProperty && statement.symbol == variable -> statement
            statement is FirVariableAssignment && assignedVariable(statement) == variable -> statement
            else -> null
        }

        private fun writtenValue(write: FirStatement): FirExpression? = when (write) {
            is FirProperty -> write.initializer
            is FirVariableAssignment -> write.rValue
            else -> null
        }

        // Every write of [variable] in [root], at [base] and below.
        private fun writes(root: FirElement, base: List<Slot>, variable: FirPropertySymbol): List<Definition> {
            val found = mutableListOf<Definition>()
            walkStatements(root, base) { element, slots ->
                if (element is FirStatement && lastWrite(element, variable) != null) {
                    writtenValue(element)?.let { found += Definition(it, slots.toList()) }
                }
                false
            }
            return found
        }

        private fun assignedVariable(assignment: FirVariableAssignment): FirVariableSymbol<*>? {
            val lValue = assignment.lValue
            val target = if (lValue is FirDesugaredAssignmentValueReferenceExpression) lValue.expressionRef.value else lValue
            return (target as? FirQualifiedAccessExpression)?.calleeReference?.toResolvedVariableSymbol()
        }
    }

    context(context: CheckerContext)
    private fun isLong(type: ConeKotlinType): Boolean =
        type.fullyExpandedType().lowerBoundIfFlexible().classId == StandardClassIds.Long

    // The containing class comes from the symbol's lookup tag, which is bound to
    // local and anonymous classes; only class ids already in hand are compared.
    context(context: CheckerContext)
    private fun isAlarmManager(symbol: FirClassLikeSymbol<*>): Boolean =
        symbol.classId == alarmManager ||
            lookupSuperTypes(symbol, lookupInterfaces = false, deep = true, useSiteSession = context.session)
                .any { it.lookupTag.classId == alarmManager }

    // The value of an integer literal (FIR drops parentheses around it and
    // folds underscores, hex, and binary spellings), or of a Kotlin const val
    // whose initializer is one; null for anything else.
    context(context: CheckerContext)
    private fun integralValue(expression: FirExpression, seen: MutableSet<FirPropertySymbol>): Long? = when (expression) {
        is FirLiteralExpression ->
            if (expression.kind in integralKinds) (expression.value as? Number)?.toLong() else null
        is FirPropertyAccessExpression -> {
            val property = expression.calleeReference.toResolvedCallableSymbol() as? FirPropertySymbol
            if (property == null || !property.isConst || !seen.add(property)) {
                null
            } else {
                property.followableInitializer()?.let { integralValue(it, seen) }
            }
        }
        else -> null
    }
}
