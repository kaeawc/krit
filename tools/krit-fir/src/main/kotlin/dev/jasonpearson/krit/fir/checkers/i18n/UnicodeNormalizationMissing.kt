package dev.jasonpearson.krit.fir.checkers.i18n

import com.intellij.lang.LighterASTNode
import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.lightChildren
import dev.jasonpearson.krit.fir.support.lightSourceOf
import dev.jasonpearson.krit.fir.support.lightText
import dev.jasonpearson.krit.fir.support.qualifiedCall
import dev.jasonpearson.krit.fir.support.significantChildren
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.descriptors.ClassKind
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirSession
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.declarations.DirectDeclarationsAccess
import org.jetbrains.kotlin.fir.declarations.utils.isData
import org.jetbrains.kotlin.fir.declarations.utils.isInlineOrValue
import org.jetbrains.kotlin.fir.expressions.FirCallableReferenceAccess
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirFunctionCallOrigin
import org.jetbrains.kotlin.fir.expressions.FirImplicitInvokeCall
import org.jetbrains.kotlin.fir.expressions.FirQualifiedAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirResolvedQualifier
import org.jetbrains.kotlin.fir.expressions.FirSpreadArgumentExpression
import org.jetbrains.kotlin.fir.expressions.FirVarargArgumentsExpression
import org.jetbrains.kotlin.fir.expressions.FirWrappedArgumentExpression
import org.jetbrains.kotlin.fir.references.FirNamedReference
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.resolve.substitution.substitutorByMap
import org.jetbrains.kotlin.fir.resolve.toRegularClassSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirConstructorSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirNamedFunctionSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirRegularClassSymbol
import org.jetbrains.kotlin.fir.types.ConeClassLikeType
import org.jetbrains.kotlin.fir.types.ConeDefinitelyNotNullType
import org.jetbrains.kotlin.fir.types.ConeIntersectionType
import org.jetbrains.kotlin.fir.types.ConeKotlinType
import org.jetbrains.kotlin.fir.types.ConeKotlinTypeProjection
import org.jetbrains.kotlin.fir.types.ConeStarProjection
import org.jetbrains.kotlin.fir.types.ConeTypeParameterType
import org.jetbrains.kotlin.fir.types.coneType
import org.jetbrains.kotlin.fir.types.constructClassLikeType
import org.jetbrains.kotlin.fir.types.isSubtypeOf
import org.jetbrains.kotlin.fir.types.lowerBoundIfFlexible
import org.jetbrains.kotlin.fir.types.resolvedType
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
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
 *   looks for the nearest `function_declaration`) has a name that, lowercased,
 *   starts with `search` or `find`;
 * - no call expression anywhere in that function (lambdas, local functions,
 *   local classes, and default values included, as Go walks the whole
 *   declaration) has a callee written `normalize`, whatever declares it: Go
 *   takes any such call as evidence the developer handled equivalence, and so
 *   does this checker. As in Go, the callee must be the plain identifier
 *   `normalize`: an infix call, a parenthesized value, and a backticked name do
 *   not count.
 *
 * Deliberate differences from Go, pinned in the golden data:
 * - Precision (`UnicodeNormalizationMissingPrecision.kt`): Go cannot see the
 *   types, so it also reports `contains` calls whose compared values carry no
 *   characters (`ids.contains(5)`, `range.contains(n)`,
 *   `statuses.contains(status)`), where the message is false. FIR drops a call
 *   only when the receiver is not text and every argument (the receiver, for a
 *   call without arguments) provably compares no characters: numbers,
 *   booleans, enums, `UUID`, `Locale`, numeric ranges and progressions, arrays
 *   (identity equality), and collections, data classes, and value classes
 *   made only of such types. Anything else, text or a type whose equality may
 *   compare text (`Any`, `File`, a `Pair<String, String>`, a data class with a
 *   `String` property, a type parameter with a `CharSequence` bound), is
 *   reported as Go reports it.
 * - Recall (`UnicodeNormalizationMissingRecall.kt`): Go reads names as
 *   written, so it misses `` title.`contains`(q) `` and `(contains)(q)`, the
 *   same calls it reports as `title.contains(q)` and `contains(q)`, and a
 *   search function whose name is backticked, such as `` `find by title` ``.
 */
internal object UnicodeNormalizationMissing : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "UnicodeNormalizationMissing"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(UnicodeNormalizationMissing)
    }

    private val contains = Name.identifier("contains")
    private const val NORMALIZE = "normalize"
    private val searchPrefixes = listOf("search", "find")

    private val charSequence = StandardClassIds.CharSequence.constructClassLikeType(emptyArray(), isMarkedNullable = true)
    private val nullableChar = StandardClassIds.Char.constructClassLikeType(emptyArray(), isMarkedNullable = true)
    private val number = StandardClassIds.Number.constructClassLikeType(emptyArray(), isMarkedNullable = true)
    private val collection =
        StandardClassIds.Collection.constructClassLikeType(arrayOf(ConeStarProjection), isMarkedNullable = true)
    private val map =
        StandardClassIds.Map.constructClassLikeType(arrayOf(ConeStarProjection, ConeStarProjection), isMarkedNullable = true)

    private val rangesPackage = FqName("kotlin.ranges")
    private val collectionPackages = setOf(FqName("kotlin.collections"), FqName("java.util"))

    // Types whose equality compares no characters.
    private val textlessClasses: Set<ClassId> = buildSet {
        add(StandardClassIds.Boolean)
        add(StandardClassIds.Unit)
        add(StandardClassIds.UByte)
        add(StandardClassIds.UShort)
        add(StandardClassIds.UInt)
        add(StandardClassIds.ULong)
        add(ClassId(FqName("java.util"), Name.identifier("UUID")))
        // Language tags are ASCII and canonicalised by the JDK.
        add(ClassId(FqName("java.util"), Name.identifier("Locale")))
        for (name in listOf("Int", "Long", "UInt", "ULong")) {
            add(ClassId(rangesPackage, Name.identifier("${name}Range")))
            add(ClassId(rangesPackage, Name.identifier("${name}Progression")))
        }
        // Arrays compare by identity.
        add(StandardClassIds.Array)
        addAll(StandardClassIds.primitiveArrayTypeByElementType.values)
        addAll(StandardClassIds.unsignedArrayTypeByElementType.values)
    }

    // Generic types whose equality compares only values of their type arguments.
    private val structuralClasses: Set<ClassId> = setOf(
        ClassId(FqName("kotlin.collections"), FqName("Map.Entry"), false),
        ClassId(rangesPackage, Name.identifier("ClosedRange")),
        ClassId(rangesPackage, Name.identifier("OpenEndRange")),
        ClassId(rangesPackage, Name.identifier("ClosedFloatingPointRange")),
        ClassId(FqName("java.util"), Name.identifier("Optional")),
    )

    private val qualifiedTypes = setOf(KtNodeTypes.DOT_QUALIFIED_EXPRESSION, KtNodeTypes.SAFE_ACCESS_EXPRESSION)

    private const val MAX_TYPE_DEPTH = 8

    private const val MESSAGE =
        "contains() inside a search/find function will miss unicode-equivalent characters. " +
            "Normalize both operands with Normalizer.normalize(..., Normalizer.Form.NFC) before comparing."

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        if (!isWrittenCall(expression)) return
        if (!isWrittenContains(expression)) return
        val function = context.containingDeclarations.lastOrNull { it is FirNamedFunctionSymbol } as? FirNamedFunctionSymbol
            ?: return
        if (!isSearchFunction(function)) return
        if (!mayCompareText(expression, context.session)) return
        if (callsNormalize(function)) return
        val source = expression.source ?: return
        // Go's call_expression starts on the receiver's first line.
        report(qualifiedCall(source)?.let { lightSourceOf(it, source) } ?: source, MESSAGE)
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

    // The function's name without backticks, lowercased. Go reads the name
    // with its backticks, so it misses `` `find by title` ``, which is the
    // same kind of search function.
    private fun isSearchFunction(function: FirNamedFunctionSymbol): Boolean {
        val name = function.name.asString().lowercase()
        return searchPrefixes.any { name.startsWith(it) }
    }

    // Whether the call is written contains: a function called by that name
    // (the callee reference keeps the written name, so an import alias counts
    // under its alias), or the invocation of a value or object of that name.
    private fun isWrittenContains(call: FirFunctionCall): Boolean {
        if (call is FirImplicitInvokeCall) {
            return when (val value = call.explicitReceiver) {
                // `(r::contains)(x)`: Go reads no name through the
                // parentheses, and the reference is not written as a call.
                is FirCallableReferenceAccess -> false
                is FirQualifiedAccessExpression -> (value.calleeReference as? FirNamedReference)?.name == contains
                is FirResolvedQualifier -> value.relativeClassFqName?.shortName() == contains
                else -> false
            }
        }
        return call.calleeReference.name == contains
    }

    // Whether the call may compare characters. A search inside text always
    // may. Otherwise, in `r.contains(x)` the argument is compared with the
    // receiver's elements, so the arguments decide: the call compares no
    // characters only when every argument's type provably carries none. A
    // call without arguments is judged by its receiver, and a call with
    // neither is reported, as in Go.
    private fun mayCompareText(call: FirFunctionCall, session: FirSession): Boolean {
        val receiver = if (call is FirImplicitInvokeCall) {
            null
        } else {
            call.explicitReceiver ?: call.extensionReceiver ?: call.dispatchReceiver
        }
        val receiverType = receiver?.let(::typeOf)
        if (receiverType != null && isText(receiverType, session)) return true
        val argumentTypes = buildList {
            for (argument in call.argumentList.arguments) {
                val value = if (argument is FirWrappedArgumentExpression) argument.expression else argument
                if (value !is FirVarargArgumentsExpression) {
                    add(typeOf(value))
                    continue
                }
                for (element in value.arguments) {
                    // A spread array passes its elements: judge them by the
                    // vararg's element type.
                    add(if (element is FirSpreadArgumentExpression) value.coneElementTypeOrNull else typeOf(element))
                }
            }
        }
        val operandTypes = argumentTypes.ifEmpty { if (receiver == null) emptyList() else listOf(receiverType) }
        if (operandTypes.isEmpty()) return true
        return operandTypes.any { type -> type == null || !isTextless(type, session, 0) }
    }

    private fun typeOf(expression: FirExpression): ConeKotlinType? = runCatching { expression.resolvedType }.getOrNull()

    private fun isText(type: ConeKotlinType, session: FirSession): Boolean =
        type.isSubtypeOf(charSequence, session) || type.isSubtypeOf(nullableChar, session)

    // Whether equality on a value of [type] provably compares no characters.
    // Unknown types, error types, and the depth cap all answer false, so the
    // call is reported as Go reports it.
    private fun isTextless(type: ConeKotlinType, session: FirSession, depth: Int): Boolean {
        if (depth > MAX_TYPE_DEPTH) return false
        return when (val t = type.fullyExpandedType(session).lowerBoundIfFlexible()) {
            is ConeDefinitelyNotNullType -> isTextless(t.original, session, depth + 1)
            // A value of an intersection or a bounded type parameter has every
            // component's type: one text component makes it text, and
            // otherwise one textless component makes it textless.
            is ConeIntersectionType -> isTextlessComponents(t.intersectedTypes, session, depth)
            is ConeTypeParameterType -> isTextlessComponents(
                t.lookupTag.typeParameterSymbol.resolvedBounds.map { it.coneType },
                session,
                depth,
            )
            is ConeClassLikeType -> isTextlessClass(t, session, depth)
            else -> false
        }
    }

    private fun isTextlessComponents(types: Collection<ConeKotlinType>, session: FirSession, depth: Int): Boolean {
        if (types.any { isText(it, session) }) return false
        return types.any { isTextless(it, session, depth + 1) }
    }

    private fun isTextlessClass(type: ConeClassLikeType, session: FirSession, depth: Int): Boolean {
        if (isText(type, session)) return false
        val classId = type.lookupTag.classId
        if (classId in textlessClasses || type.isSubtypeOf(number, session)) return true
        val symbol = type.toRegularClassSymbol(session) ?: return false
        if (symbol.classKind == ClassKind.ENUM_CLASS) return true
        if (classId in structuralClasses || isCollection(type, classId, session)) {
            val arguments = type.typeArguments
            return arguments.isNotEmpty() && arguments.all { argument ->
                val argumentType = (argument as? ConeKotlinTypeProjection)?.type
                argumentType != null && isTextless(argumentType, session, depth + 1)
            }
        }
        if (symbol.isData || symbol.isInlineOrValue) return isTextlessRecord(type, symbol, session, depth)
        return false
    }

    // A stdlib or JDK collection or map compares its elements, keys, and values.
    private fun isCollection(type: ConeClassLikeType, classId: ClassId, session: FirSession): Boolean =
        classId.packageFqName in collectionPackages && type.typeArguments.isNotEmpty() &&
            (type.isSubtypeOf(collection, session) || type.isSubtypeOf(map, session))

    // A data or value class compares its primary constructor's properties.
    @OptIn(DirectDeclarationsAccess::class)
    private fun isTextlessRecord(
        type: ConeClassLikeType,
        symbol: FirRegularClassSymbol,
        session: FirSession,
        depth: Int,
    ): Boolean {
        val constructor = symbol.declarationSymbols.filterIsInstance<FirConstructorSymbol>().firstOrNull { it.isPrimary }
            ?: return false
        val parameters = constructor.valueParameterSymbols
        if (parameters.isEmpty()) return false
        val arguments = type.typeArguments
        val substitution = symbol.typeParameterSymbols.withIndex().associate { (index, parameter) ->
            parameter to ((arguments.getOrNull(index) as? ConeKotlinTypeProjection)?.type ?: return false)
        }
        val substitutor = substitutorByMap(substitution, session)
        return parameters.all { isTextless(substitutor.substituteOrSelf(it.resolvedReturnType), session, depth + 1) }
    }

    // Whether any call expression in [function] has a callee written
    // `normalize`: the plain identifier, which is the name Go's
    // call_expression match reads. Walks the function's whole source, like
    // Go's subtree walk.
    private fun callsNormalize(function: FirNamedFunctionSymbol): Boolean {
        val source = function.source ?: return false
        val pending = ArrayDeque<LighterASTNode>()
        pending.add(source.lighterASTNode)
        while (pending.isNotEmpty()) {
            val node = pending.removeLast()
            if (node.tokenType == KtNodeTypes.CALL_EXPRESSION) {
                val callee = significantChildren(source, node).firstOrNull()
                if (callee?.tokenType == KtNodeTypes.REFERENCE_EXPRESSION && lightText(source, callee) == NORMALIZE) {
                    return true
                }
            }
            pending.addAll(lightChildren(source, node))
        }
        return false
    }
}
