package dev.jasonpearson.krit.fir.checkers.style

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.unwrapArgument
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.expressions.FirCheckedSafeCallSubject
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirSmartCastExpression
import org.jetbrains.kotlin.fir.expressions.FirSpreadArgumentExpression
import org.jetbrains.kotlin.fir.expressions.FirVarargArgumentsExpression
import org.jetbrains.kotlin.fir.expressions.arguments
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.types.canBeNull
import org.jetbrains.kotlin.fir.types.resolvedType
import org.jetbrains.kotlin.name.CallableId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name

// Flags `.orEmpty()`, `.isNullOrEmpty()` and `.isNullOrBlank()` on a receiver
// that cannot be null, and `listOfNotNull(...)` / `setOfNotNull(...)` whose
// arguments are all non-null.
//
// Mirrors the Go UselessCallOnNotNull rule:
// - the call is written with a receiver (`x.orEmpty()`), so an implicit
//   receiver does not count;
// - the receiver type is the one the declaration gives it: a smart cast does
//   not make `.orEmpty()` useless, as in detekt, which reads the declared
//   type;
// - the `*OfNotNull` call has at least one argument and none is spread;
// - the finding is reported on the callee name, with Go's messages:
//   "Remove redundant call to orEmpty", "Replace isNullOrEmpty with isEmpty",
//   "Replace listOfNotNull with listOf".
//
// Deliberate differences from Go, pinned by goldens:
// - Precision: the call must resolve to the stdlib function. Go matches the
//   method name and a receiver type name, so a member or extension of the same
//   name on another type is a lookalike only FIR separates.
// - Precision: a platform-typed receiver or argument (`String!`) can be null,
//   so it is not useless.
// - Recall: Go accepts only receiver types it can name (String, List, Set, Map,
//   Collection, CharSequence, Sequence and the Mutable* forms); FIR also
//   reports the other types the stdlib declares these for, such as an
//   ArrayList, an Array or an Iterable.
// - Recall: Go skips a receiver whose text contains `?.` anywhere, and any
//   `*OfNotNull` argument that is a navigation expression or contains a
//   stdlib `...OrNull` call by name; the resolved type decides here.
// - Recall: Go reads only a bare `listOfNotNull`; a qualified call or an
//   import alias is the same function.
internal object UselessCallOnNotNull : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "UselessCallOnNotNull"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(UselessCallOnNotNull)
    }

    private val text = FqName("kotlin.text")
    private val collections = FqName("kotlin.collections")
    private val sequences = FqName("kotlin.sequences")

    private fun id(pkg: FqName, name: String) = CallableId(pkg, Name.identifier(name))

    // Replacement for a call on a non-null receiver; null means remove the call.
    private val nullTolerantCalls: Map<CallableId, String?> = mapOf(
        id(text, "orEmpty") to null,
        id(collections, "orEmpty") to null,
        id(sequences, "orEmpty") to null,
        id(text, "isNullOrEmpty") to "isEmpty",
        id(collections, "isNullOrEmpty") to "isEmpty",
        id(text, "isNullOrBlank") to "isBlank",
    )

    private val ofNotNullCalls: Map<CallableId, String> = mapOf(
        id(collections, "listOfNotNull") to "listOf",
        id(collections, "setOfNotNull") to "setOf",
    )

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val source = expression.source ?: return
        if (source.kind !is KtRealSourceElementKind && expression.explicitReceiver !is FirCheckedSafeCallSubject) return
        val callableId = expression.calleeReference.toResolvedCallableSymbol()?.callableId ?: return
        val message = when {
            callableId in nullTolerantCalls -> receiverMessage(expression, callableId)
            callableId in ofNotNullCalls -> ofNotNullMessage(expression, callableId)
            else -> null
        } ?: return
        report(expression.calleeReference.source ?: source, message)
    }

    context(context: CheckerContext)
    private fun receiverMessage(call: FirFunctionCall, callableId: CallableId): String? {
        val written = call.explicitReceiver ?: return null
        val receiver = if (written is FirCheckedSafeCallSubject) written.originalReceiverRef.value else written
        if (canBeNullByDeclaration(receiver)) return null
        val name = callableId.callableName.asString()
        val replacement = nullTolerantCalls.getValue(callableId)
        return if (replacement == null) "Remove redundant call to $name" else "Replace $name with $replacement"
    }

    context(context: CheckerContext)
    private fun ofNotNullMessage(call: FirFunctionCall, callableId: CallableId): String? {
        val arguments = call.arguments.flatMap { if (it is FirVarargArgumentsExpression) it.arguments else listOf(it) }
        if (arguments.isEmpty() || arguments.any { it is FirSpreadArgumentExpression }) return null
        if (arguments.any { canBeNullByDeclaration(it) }) return null
        val name = callableId.callableName.asString()
        return "Replace $name with ${ofNotNullCalls.getValue(callableId)}"
    }

    // True when the expression can be null by its declared type: a smart cast
    // is looked through, and a platform type counts as nullable.
    context(context: CheckerContext)
    private fun canBeNullByDeclaration(expression: FirExpression): Boolean {
        var current = unwrapArgument(expression)
        while (current is FirSmartCastExpression) current = current.originalExpression
        return current.resolvedType.canBeNull(context.session)
    }
}
