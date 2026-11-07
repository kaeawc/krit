package dev.jasonpearson.krit.fir.checkers.androidlint

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.declarations.FirClass
import org.jetbrains.kotlin.fir.declarations.FirFile
import org.jetbrains.kotlin.fir.declarations.FirFunction
import org.jetbrains.kotlin.fir.declarations.FirProperty
import org.jetbrains.kotlin.fir.expressions.FirArgumentList
import org.jetbrains.kotlin.fir.expressions.FirBlock
import org.jetbrains.kotlin.fir.expressions.FirCheckNotNullCall
import org.jetbrains.kotlin.fir.expressions.FirElvisExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirPropertyAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirQualifiedAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirReturnExpression
import org.jetbrains.kotlin.fir.expressions.FirSmartCastExpression
import org.jetbrains.kotlin.fir.expressions.FirTypeOperatorCall
import org.jetbrains.kotlin.fir.expressions.FirVariableAssignment
import org.jetbrains.kotlin.fir.expressions.FirWhenBranch
import org.jetbrains.kotlin.fir.expressions.FirWhenExpression
import org.jetbrains.kotlin.fir.expressions.FirWrappedArgumentExpression
import org.jetbrains.kotlin.fir.expressions.arguments
import org.jetbrains.kotlin.fir.references.toResolvedBaseSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirPropertySymbol
import org.jetbrains.kotlin.fir.types.coneType
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.symbols.impl.FirConstructorSymbol
import org.jetbrains.kotlin.fir.types.ConeClassLikeType
import org.jetbrains.kotlin.fir.types.ConeKotlinType
import org.jetbrains.kotlin.fir.types.ConeKotlinTypeProjection
import org.jetbrains.kotlin.fir.types.ConeTypeProjection
import org.jetbrains.kotlin.fir.types.isMarkedNullable
import org.jetbrains.kotlin.fir.types.lowerBoundIfFlexible
import org.jetbrains.kotlin.fir.types.resolvedType
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name

// Flags a `java.util.HashMap` constructor call whose key type is `Int` or
// `Long`, where an Android SparseArray variant avoids boxing the keys:
// `HashMap<Int, V>()` suggests SparseArray (SparseBooleanArray,
// SparseIntArray, or SparseLongArray for a Boolean, Int, or Long value), and
// `HashMap<Long, V>()` suggests LongSparseArray.
//
// Like the Go rule:
// - only a constructor call of HashMap counts, however it is spelled
//   (`HashMap`, `kotlin.collections.HashMap`, `java.util.HashMap`), with any
//   constructor arguments. A subclass constructor, a LinkedHashMap, a superclass
//   delegation (`class M : HashMap<Int, String>()`), a constructor reference,
//   and `hashMapOf` are not reported;
// - `Int` and `java.lang.Integer` keys suggest the Int variants, and the
//   message names the key as written (`Int`, `Integer`, `Long`);
// - a nullable key (`HashMap<Int?, V>`) is not reported, and a nullable value
//   suggests plain SparseArray;
// - the finding sits on the first line of the call expression, including the
//   package qualifier of a fully qualified call.
//
// Deliberate differences from Go, each pinned in the golden data:
// - Recall: the key and value types are resolved, so a key or value named
//   through a typealias, a parenthesized key type, an import alias of HashMap,
//   a user typealias to HashMap (`typealias IntMap<V> = HashMap<Int, V>`), and
//   type arguments inferred from the expected type
//   (`val m: HashMap<Int, String> = HashMap()`) are reported. Go reads only the
//   written type arguments by their last identifier and needs the call name
//   HashMap. A call on the right of `?:` or `+` is reported too; tree-sitter
//   does not parse it as a call there, so Go misses it.
// - Precision: a same-package class or function named HashMap is not
//   java.util.HashMap and is not reported. Go matches the call name alone.
//   Neither is a key that is only named Int or Long (an imported or nested
//   class, a type parameter), and a value only named Boolean, Int, or Long
//   suggests plain SparseArray.
// - Escape: a SparseArray is not a Map, so a HashMap is not reported when it is
//   returned or assigned where a `Map`/`MutableMap` is declared, passed as a
//   call argument, or stored in a variable that is itself returned, passed on,
//   assigned on, or used through a Map-only member (getOrPut, entries, keys,
//   values, iteration, ...).
internal object UseSparseArrays : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "UseSparseArrays"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(UseSparseArrays)
    }

    private val hashMapClassId = ClassId(FqName("java.util"), Name.identifier("HashMap"))

    private val kotlinInt = ClassId(FqName("kotlin"), Name.identifier("Int"))
    private val kotlinLong = ClassId(FqName("kotlin"), Name.identifier("Long"))
    private val kotlinBoolean = ClassId(FqName("kotlin"), Name.identifier("Boolean"))
    private val javaInteger = ClassId(FqName("java.lang"), Name.identifier("Integer"))
    private val javaLong = ClassId(FqName("java.lang"), Name.identifier("Long"))
    private val javaBoolean = ClassId(FqName("java.lang"), Name.identifier("Boolean"))

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        if (expression.calleeReference.toResolvedCallableSymbol() !is FirConstructorSymbol) return
        // The call's type is the constructed type with its (explicit or
        // inferred) type arguments; a typealias such as
        // kotlin.collections.HashMap is expanded to java.util.HashMap.
        val constructed = expression.resolvedType.fullyExpandedType().lowerBoundIfFlexible() as? ConeClassLikeType
            ?: return
        if (constructed.lookupTag.classId != hashMapClassId) return
        val arguments = constructed.typeArguments
        if (arguments.size != 2) return
        val key = nonNullClassId(arguments[0]) ?: return
        val value = nonNullClassId(arguments[1])
        val keyName: String
        val suggestion: String
        when (key) {
            kotlinInt, javaInteger -> {
                keyName = key.shortClassName.asString()
                suggestion = when (value) {
                    kotlinBoolean, javaBoolean -> "SparseBooleanArray"
                    kotlinInt, javaInteger -> "SparseIntArray"
                    kotlinLong, javaLong -> "SparseLongArray"
                    else -> "SparseArray"
                }
            }
            kotlinLong, javaLong -> {
                keyName = "Long"
                suggestion = "LongSparseArray"
            }
            else -> return
        }
        if (escapesAsMap(context.containingElements, expression)) return
        report(expression.source, "Use $suggestion instead of HashMap<$keyName, ...> for better performance on Android.")
    }

    private val mapClassIds = setOf(
        ClassId(FqName("kotlin.collections"), Name.identifier("Map")),
        ClassId(FqName("kotlin.collections"), Name.identifier("MutableMap")),
    )

    // Members a Map has and a SparseArray does not, so a map used through them
    // cannot be swapped.
    private val mapOnlyMembers = setOf(
        "getOrPut", "getOrElse", "getOrDefault", "mapValues", "mapKeys", "filterKeys", "filterValues", "toMap",
        "toList", "toMutableMap", "withDefault", "plus", "minus", "asSequence", "forEach", "iterator", "entries",
        "keys", "values", "putIfAbsent", "computeIfAbsent", "computeIfPresent", "compute", "merge", "putAll",
        "containsValue", "filter", "map", "flatMap", "any", "all", "none", "count", "firstNotNullOfOrNull",
        "maxByOrNull", "minByOrNull", "toSortedMap", "getValue", "plusAssign", "minusAssign",
    )

    // Receiver-preserving scope functions the walk looks through.
    private val transparentScopeFunctions = setOf("apply", "also")

    context(context: CheckerContext)
    private fun isMapType(type: ConeKotlinType?): Boolean {
        val expanded = type?.fullyExpandedType()?.lowerBoundIfFlexible() as? ConeClassLikeType ?: return false
        return expanded.lookupTag.classId in mapClassIds
    }

    // Whether the constructed map is typed, passed, or returned as a Map.
    context(context: CheckerContext)
    private fun escapesAsMap(path: List<FirElement>, expression: FirFunctionCall): Boolean {
        var current: FirElement = expression
        var i = path.lastIndex
        if (path.lastOrNull() === expression) i--
        while (i >= 0) {
            val parent = path[i]
            when (parent) {
                is FirSmartCastExpression, is FirArgumentList, is FirWrappedArgumentExpression,
                is FirWhenBranch, is FirWhenExpression, is FirElvisExpression, is FirCheckNotNullCall -> {}
                is FirTypeOperatorCall -> {}
                is FirBlock -> if (parent.statements.lastOrNull() !== current) return false
                is FirReturnExpression -> return isMapType(parent.target.labeledElement.returnTypeRef.coneType)
                is FirFunctionCall -> {
                    if (parent.explicitReceiver === current) {
                        if (parent.calleeReference.name.asString() !in transparentScopeFunctions) return false
                    } else {
                        return true
                    }
                }
                is FirProperty -> {
                    if (parent.initializer !== current) return false
                    if (isMapType(parent.returnTypeRef.coneType)) return true
                    return variableEscapes(path, i, parent.symbol)
                }
                is FirVariableAssignment -> {
                    if (parent.rValue !== current) return false
                    val symbol = (parent.lValue as? FirQualifiedAccessExpression)
                        ?.calleeReference?.toResolvedBaseSymbol() as? FirPropertySymbol ?: return false
                    return isMapType(symbol.resolvedReturnType)
                }
                is FirFunction -> return current is FirBlock && isMapType(parent.returnTypeRef.coneType)
                else -> return false
            }
            current = parent
            i--
        }
        return false
    }

    // Whether a variable holding the map is returned, passed on, stored in
    // another variable, or used through a Map-only member within its scope.
    private fun variableEscapes(path: List<FirElement>, propertyIndex: Int, variable: FirPropertySymbol): Boolean {
        val scope = (propertyIndex - 1 downTo 0).map { path[it] }.firstOrNull {
            it is FirFunction || it is FirClass || it is FirFile
        } ?: return false
        var found = false
        fun refersToVariable(expression: FirElement?): Boolean {
            var e = expression
            while (true) {
                e = when (e) {
                    is FirSmartCastExpression -> e.originalExpression
                    is FirWrappedArgumentExpression -> e.expression
                    else -> break
                }
            }
            return e is FirQualifiedAccessExpression && e.calleeReference.toResolvedBaseSymbol() == variable
        }
        scope.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                if (found) return
                found = when (element) {
                    is FirFunctionCall ->
                        element.arguments.any(::refersToVariable) ||
                            (refersToVariable(element.explicitReceiver) &&
                                element.calleeReference.name.asString() in mapOnlyMembers)
                    is FirPropertyAccessExpression ->
                        refersToVariable(element.explicitReceiver) &&
                            element.calleeReference.name.asString() in mapOnlyMembers
                    is FirReturnExpression -> refersToVariable(element.result)
                    is FirVariableAssignment -> refersToVariable(element.rValue)
                    is FirProperty -> refersToVariable(element.initializer)
                    else -> false
                }
                if (!found) element.acceptChildren(this)
            }
        })
        return found
    }

    // The class a type argument names, after typealias expansion, or null when
    // it is nullable or not a class type. A flexible Java type counts by its
    // non-null lower bound.
    context(context: CheckerContext)
    private fun nonNullClassId(projection: ConeTypeProjection): ClassId? {
        val type: ConeKotlinType = (projection as? ConeKotlinTypeProjection)?.type ?: return null
        val expanded = type.fullyExpandedType().lowerBoundIfFlexible()
        if (expanded.isMarkedNullable) return null
        return (expanded as? ConeClassLikeType)?.lookupTag?.classId
    }
}
