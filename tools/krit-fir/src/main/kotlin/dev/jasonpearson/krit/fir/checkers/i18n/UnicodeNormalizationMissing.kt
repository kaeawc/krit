package dev.jasonpearson.krit.fir.checkers.i18n

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.identifierText
import dev.jasonpearson.krit.fir.support.lightSourceOf
import dev.jasonpearson.krit.fir.support.significantChildren
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.FirSession
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.expressions.FirCallableReferenceAccess
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirFunctionCallOrigin
import org.jetbrains.kotlin.fir.expressions.FirImplicitInvokeCall
import org.jetbrains.kotlin.fir.expressions.FirQualifiedAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirResolvedQualifier
import org.jetbrains.kotlin.fir.expressions.FirVarargArgumentsExpression
import org.jetbrains.kotlin.fir.expressions.FirWrappedArgumentExpression
import org.jetbrains.kotlin.fir.references.FirNamedReference
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.symbols.SymbolInternals
import org.jetbrains.kotlin.fir.symbols.impl.FirNamedFunctionSymbol
import org.jetbrains.kotlin.fir.types.ConeClassLikeType
import org.jetbrains.kotlin.fir.types.ConeDefinitelyNotNullType
import org.jetbrains.kotlin.fir.types.ConeIntersectionType
import org.jetbrains.kotlin.fir.types.ConeKotlinType
import org.jetbrains.kotlin.fir.types.ConeTypeParameterType
import org.jetbrains.kotlin.fir.types.coneType
import org.jetbrains.kotlin.fir.types.constructClassLikeType
import org.jetbrains.kotlin.fir.types.isNothingOrNullableNothing
import org.jetbrains.kotlin.fir.types.isSubtypeOf
import org.jetbrains.kotlin.fir.types.lowerBoundIfFlexible
import org.jetbrains.kotlin.fir.types.resolvedType
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid
import org.jetbrains.kotlin.lexer.KtTokens
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.name.StandardClassIds

/**
 * Port of the Go UnicodeNormalizationMissing rule: a `contains(...)` call
 * inside a search/find function that never normalizes, reported on the line
 * where the call expression starts (the receiver's first line), like Go.
 *
 * Mirrors the Go rule's evidence:
 * - the call is written `contains(...)`: a regular call (not the `in`
 *   operator, not an infix call, neither of which is a Go `call_expression`)
 *   whose callee is written `contains`, so a function imported under the alias
 *   `contains` counts and `contains` imported under another name does not, or
 *   the invocation of a function-typed value named `contains`;
 * - the nearest enclosing named function (lambdas, anonymous functions,
 *   accessors, constructors, and local classes are looked through, as Go
 *   looks for the nearest `function_declaration`) has a name that, as written
 *   and lowercased, starts with `search` or `find`;
 * - no call anywhere in that function (lambdas, local functions, local
 *   classes, and default values included, as Go walks the whole declaration)
 *   is written `normalize`, whatever declares it: Go takes any such call as
 *   evidence the developer handled equivalence, and so does this checker.
 *
 * Deliberate differences from Go, pinned in the golden data:
 * - Precision (`UnicodeNormalizationMissingPrecision.kt`): Go cannot see the
 *   receiver type, so it also reports `contains` over values that are not
 *   text (`ids.contains(5)`, `users.contains(user)`, `range.contains(n)`),
 *   where no characters are compared and the message is false. FIR reports
 *   the call only when the receiver or an argument could hold text: a
 *   `CharSequence` or `Char` (or a subtype), a supertype of `String` or `Char`
 *   (`Any`, `Comparable<String>`), or a type parameter whose bounds all allow
 *   one.
 * - Recall (`UnicodeNormalizationMissingRecall.kt`): Go reads the callee as
 *   written, so it misses `` title.`contains`(q) `` and `(contains)(q)`, the
 *   same calls it reports as `title.contains(q)` and `contains(q)`.
 */
internal object UnicodeNormalizationMissing : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "UnicodeNormalizationMissing"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(UnicodeNormalizationMissing)
    }

    private val contains = Name.identifier("contains")
    private val normalize = Name.identifier("normalize")
    private val searchPrefixes = listOf("search", "find")

    private val charSequence = StandardClassIds.CharSequence.constructClassLikeType(emptyArray(), isMarkedNullable = true)
    private val nullableChar = StandardClassIds.Char.constructClassLikeType(emptyArray(), isMarkedNullable = true)
    private val string = StandardClassIds.String.constructClassLikeType(emptyArray(), isMarkedNullable = false)
    private val char = StandardClassIds.Char.constructClassLikeType(emptyArray(), isMarkedNullable = false)

    private val qualifiedTypes = setOf(KtNodeTypes.DOT_QUALIFIED_EXPRESSION, KtNodeTypes.SAFE_ACCESS_EXPRESSION)

    private const val MAX_TYPE_DEPTH = 8

    private const val MESSAGE =
        "contains() inside a search/find function will miss unicode-equivalent characters. " +
            "Normalize both operands with Normalizer.normalize(..., Normalizer.Form.NFC) before comparing."

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        if (!isWrittenCall(expression)) return
        if (!isWritten(expression, contains)) return
        val function = context.containingDeclarations.lastOrNull { it is FirNamedFunctionSymbol } as? FirNamedFunctionSymbol
            ?: return
        if (!isSearchFunction(function)) return
        if (!comparesText(expression, context.session)) return
        if (callsNormalize(function)) return
        val source = expression.source ?: return
        report(callExpressionSource(source), MESSAGE)
    }

    // A call written as a call expression, `f(x)` or `r.f(x)`, which is what
    // Go visits: not the `in` operator or an infix call. K2 gives the implicit
    // invoke of a function-typed value the Operator origin, so that origin is
    // accepted only for an implicit invoke whose source is a call expression.
    private fun isWrittenCall(call: FirFunctionCall): Boolean {
        when (call.origin) {
            FirFunctionCallOrigin.Regular -> {}
            FirFunctionCallOrigin.Operator -> if (call !is FirImplicitInvokeCall) return false
            else -> return false
        }
        val node = call.source?.lighterASTNode ?: return false
        return node.tokenType == KtNodeTypes.CALL_EXPRESSION || node.tokenType in qualifiedTypes
    }

    // The function's name as written (backticks included), lowercased, like Go.
    private fun isSearchFunction(function: FirNamedFunctionSymbol): Boolean {
        val source = function.source ?: return false
        val name = identifierText(source)?.lowercase() ?: return false
        return searchPrefixes.any { name.startsWith(it) }
    }

    // Whether the call is written [name]: a function called by that name
    // (the callee reference keeps the written name, so an import alias counts
    // under its alias), or the invocation of a value or object of that name.
    private fun isWritten(call: FirFunctionCall, name: Name): Boolean {
        if (call is FirImplicitInvokeCall) {
            return when (val value = call.explicitReceiver) {
                // `(r::contains)(x)`: Go reads no name through the
                // parentheses, and the reference is not written as a call.
                is FirCallableReferenceAccess -> false
                is FirQualifiedAccessExpression -> (value.calleeReference as? FirNamedReference)?.name == name
                is FirResolvedQualifier -> value.relativeClassFqName?.shortName() == name
                else -> false
            }
        }
        return call.calleeReference.name == name
    }

    // The receiver or an argument could hold text. An implicit-invoke call's
    // receiver is the function value itself, so only its arguments count.
    private fun comparesText(call: FirFunctionCall, session: FirSession): Boolean {
        val receiver = if (call is FirImplicitInvokeCall) {
            null
        } else {
            call.explicitReceiver ?: call.extensionReceiver ?: call.dispatchReceiver
        }
        val operands = buildList {
            receiver?.let(::add)
            for (argument in call.argumentList.arguments) {
                val value = if (argument is FirWrappedArgumentExpression) argument.expression else argument
                if (value is FirVarargArgumentsExpression) addAll(value.arguments) else add(value)
            }
        }
        return operands.any { operand -> typeOf(operand)?.let { mayHoldText(it, session, 0) } == true }
    }

    private fun typeOf(expression: FirExpression): ConeKotlinType? = runCatching { expression.resolvedType }.getOrNull()

    private fun mayHoldText(type: ConeKotlinType, session: FirSession, depth: Int): Boolean {
        if (depth > MAX_TYPE_DEPTH) return true
        return when (val t = type.fullyExpandedType(session).lowerBoundIfFlexible()) {
            is ConeDefinitelyNotNullType -> mayHoldText(t.original, session, depth + 1)
            is ConeIntersectionType -> t.intersectedTypes.all { mayHoldText(it, session, depth + 1) }
            is ConeTypeParameterType -> t.lookupTag.typeParameterSymbol.resolvedBounds.all {
                mayHoldText(it.coneType, session, depth + 1)
            }
            is ConeClassLikeType -> !t.isNothingOrNullableNothing && (
                t.isSubtypeOf(charSequence, session) ||
                    t.isSubtypeOf(nullableChar, session) ||
                    string.isSubtypeOf(t, session) ||
                    char.isSubtypeOf(t, session)
                )
            else -> true
        }
    }

    // Whether any call in [function] is written `normalize`. Walks the whole
    // declaration, like Go's subtree walk over the function_declaration.
    @OptIn(SymbolInternals::class)
    private fun callsNormalize(function: FirNamedFunctionSymbol): Boolean {
        var found = false
        function.fir.accept(object : FirVisitorVoid() {
            // Checked here rather than in visitFunctionCall: the generated
            // visitors send an implicit invoke call to visitElement.
            override fun visitElement(element: FirElement) {
                if (found) return
                if (element is FirFunctionCall && isWritten(element, normalize)) {
                    found = true
                    return
                }
                element.acceptChildren(this)
            }
        })
        return found
    }

    // The qualified expression (`r.contains(x)` / `r?.contains(x)`) whose
    // selector is this call, so the finding lands on the receiver's first
    // line, where Go's call_expression starts. K2 gives a dot call the whole
    // qualified expression as its source; a safe call keeps the selector call
    // expression, so step up to its parent.
    private fun callExpressionSource(source: KtSourceElement): KtSourceElement {
        val node = source.lighterASTNode
        if (node.tokenType in qualifiedTypes || node.tokenType != KtNodeTypes.CALL_EXPRESSION) return source
        val parent = source.treeStructure.getParent(node) ?: return source
        if (parent.tokenType !in qualifiedTypes) return source
        val parts = significantChildren(source, parent, setOf(KtTokens.DOT, KtTokens.SAFE_ACCESS))
        return if (parts.size > 1 && parts.last() == node) lightSourceOf(parent, source) else source
    }
}
