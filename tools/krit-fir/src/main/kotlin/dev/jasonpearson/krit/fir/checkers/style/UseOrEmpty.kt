package dev.jasonpearson.krit.fir.checkers.style

import com.intellij.lang.LighterASTNode
import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.lightText
import dev.jasonpearson.krit.fir.support.significantChildren
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirExpressionChecker
import org.jetbrains.kotlin.fir.expressions.FirCheckNotNullCall
import org.jetbrains.kotlin.fir.expressions.FirElvisExpression
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirQualifiedAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirSafeCallExpression
import org.jetbrains.kotlin.fir.expressions.FirSmartCastExpression
import org.jetbrains.kotlin.fir.expressions.FirTypeOperatorCall
import org.jetbrains.kotlin.fir.expressions.FirVarargArgumentsExpression
import org.jetbrains.kotlin.fir.expressions.FirWhenExpression
import org.jetbrains.kotlin.fir.expressions.arguments
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.types.ConeKotlinType
import org.jetbrains.kotlin.fir.types.ConeStarProjection
import org.jetbrains.kotlin.fir.types.constructClassLikeType
import org.jetbrains.kotlin.fir.types.isSubtypeOf
import org.jetbrains.kotlin.fir.types.resolvedType
import org.jetbrains.kotlin.name.CallableId
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.name.StandardClassIds

// Flags `x ?: emptyList()` (and the other empty collection, sequence, array,
// and string fallbacks) that should be `x.orEmpty()`.
//
// Mirrors the Go UseOrEmpty rule:
// - the Elvis fallback is an `emptyList/emptySet/emptyMap/emptyArray/
//   emptySequence()` call, a `listOf/setOf/mapOf/arrayOf/sequenceOf()` call
//   with no arguments, or the empty string literal `""` / `""""""`, as
//   written (a parenthesized fallback does not count);
// - an unqualified `emptyArray()` fallback is skipped, the way Go skips a
//   fallback whose text starts with `emptyArray(` (and never sees
//   `emptyArray<T>()`, see below); `arrayOf()` and a qualified
//   `kotlin.emptyArray()` still count;
// - an Elvis anywhere inside a string template is skipped;
// - a left side that reads through a safe call (`a?.b ?: emptyList()`) is
//   skipped;
// - the finding is reported on the Elvis, so on the line where its left side
//   starts, with Go's message naming the fallback as written.
//
// Deliberate differences from Go, pinned by goldens:
// - Precision: the fallback must resolve to the stdlib function (or the
//   JDK's `java.util.Collections.emptyList/emptySet/emptyMap`); Go matches
//   any call with one of the names, such as a local `fun emptyList()`.
// - Precision: the left side must have a type `.orEmpty()` is declared for
//   that fallback: a Collection for the list and set fallbacks, a Map, an
//   Array, a Sequence, or a String. Go reports `value ?: ""` on an `Any?` or
//   `CharSequence?` and `values ?: emptyList()` on an `Iterable?`, where no
//   `.orEmpty()` replaces the Elvis.
// - Precision: `listOf { ... }` passes the lambda as an element, so the list
//   is not empty; Go counts a call without a value-argument list as empty.
// - Recall: Go skips any left side whose text contains `?.`, so it also
//   misses safe calls inside an argument, a lambda, or a string literal of
//   the left side (`names[items.indexOfFirst { it?.ok == true }] ?: ""`).
//   The checker only skips a left side whose value reads through a safe
//   call (see readsThroughSafeCall).
// - Recall: an import alias of a stdlib fallback still names the empty
//   value; Go compares the callee name and misses it.
// - Recall: tree-sitter parses `x ?: emptyList<String>()` as the call
//   `(x ?: emptyList)<String>()`, so Go never sees a fallback with explicit
//   type arguments.
internal object UseOrEmpty : FirExpressionChecker<FirElvisExpression>(MppCheckerKind.Common), FirRule {
    override val ruleId = "UseOrEmpty"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val elvisExpressionCheckers = setOf(UseOrEmpty)
    }

    private enum class Family(val classId: ClassId, val typeArguments: Int) {
        COLLECTION(StandardClassIds.Collection, 1),
        MAP(StandardClassIds.Map, 2),
        ARRAY(StandardClassIds.Array, 1),
        SEQUENCE(ClassId(FqName("kotlin.sequences"), Name.identifier("Sequence")), 1),
        STRING(StandardClassIds.String, 0),
        ;

        fun receiverType(): ConeKotlinType = classId.constructClassLikeType(
            Array(typeArguments) { ConeStarProjection },
            isMarkedNullable = true,
        )
    }

    private val collections = FqName("kotlin.collections")
    private val sequences = FqName("kotlin.sequences")
    private val kotlin = FqName("kotlin")
    private val javaUtilCollections = ClassId(FqName("java.util"), Name.identifier("Collections"))

    private fun id(pkg: FqName, name: String) = CallableId(pkg, Name.identifier(name))
    private fun member(owner: ClassId, name: String) = CallableId(owner, Name.identifier(name))

    // Calls that always return an empty value.
    private val emptyFunctions: Map<CallableId, Family> = mapOf(
        id(collections, "emptyList") to Family.COLLECTION,
        id(collections, "emptySet") to Family.COLLECTION,
        id(collections, "emptyMap") to Family.MAP,
        id(kotlin, "emptyArray") to Family.ARRAY,
        id(sequences, "emptySequence") to Family.SEQUENCE,
        member(javaUtilCollections, "emptyList") to Family.COLLECTION,
        member(javaUtilCollections, "emptySet") to Family.COLLECTION,
        member(javaUtilCollections, "emptyMap") to Family.MAP,
    )

    // Factories that return an empty value when called without arguments.
    private val factoryFunctions: Map<CallableId, Family> = mapOf(
        id(collections, "listOf") to Family.COLLECTION,
        id(collections, "setOf") to Family.COLLECTION,
        id(collections, "mapOf") to Family.MAP,
        id(kotlin, "arrayOf") to Family.ARRAY,
        id(sequences, "sequenceOf") to Family.SEQUENCE,
    )

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirElvisExpression) {
        val source = expression.source ?: return
        if (source.kind != KtRealSourceElementKind) return
        if (source.lighterASTNode.tokenType != KtNodeTypes.BINARY_EXPRESSION) return
        val rightNode = significantChildren(source, source.lighterASTNode).lastOrNull() ?: return
        val rightText = lightText(source, rightNode).trim()

        val family = fallbackFamily(expression.rhs, rightNode, rightText) ?: return
        if (rightText.startsWith("emptyArray")) return
        if (insideStringTemplate(source)) return
        if (readsThroughSafeCall(expression.lhs)) return
        if (!expression.lhs.resolvedType.isSubtypeOf(family.receiverType(), context.session)) return

        report(source, "Use '.orEmpty()' instead of '?: $rightText'.")
    }

    private fun fallbackFamily(rhs: FirExpression, rightNode: LighterASTNode, rightText: String): Family? {
        if (rightText == "\"\"" || rightText == "\"\"\"\"\"\"") return Family.STRING
        // Go requires a call_expression, which also covers a qualified call.
        if (rightNode.tokenType != KtNodeTypes.CALL_EXPRESSION &&
            rightNode.tokenType != KtNodeTypes.DOT_QUALIFIED_EXPRESSION
        ) {
            return null
        }
        val call = rhs as? FirFunctionCall ?: return null
        val callableId = call.calleeReference.toResolvedCallableSymbol()?.callableId ?: return null
        emptyFunctions[callableId]?.let { return it }
        val family = factoryFunctions[callableId] ?: return null
        val noArguments = call.arguments.all { it is FirVarargArgumentsExpression && it.arguments.isEmpty() }
        return family.takeIf { noArguments }
    }

    private fun insideStringTemplate(source: KtSourceElement): Boolean {
        val tree = source.treeStructure
        var node = source.lighterASTNode
        while (true) {
            node = tree.getParent(node) ?: return false
            if (node.tokenType == KtNodeTypes.STRING_TEMPLATE) return true
        }
    }

    // The left side's value flows through a safe call: the safe call itself,
    // or a receiver, `!!` operand, cast operand, nested Elvis operand, or
    // `if`/`when` branch result that reads through one.
    private fun readsThroughSafeCall(expression: FirExpression?): Boolean = when (expression) {
        null -> false
        is FirSafeCallExpression -> true
        is FirSmartCastExpression -> readsThroughSafeCall(expression.originalExpression)
        is FirCheckNotNullCall -> readsThroughSafeCall(expression.argumentList.arguments.firstOrNull())
        is FirTypeOperatorCall -> readsThroughSafeCall(expression.argumentList.arguments.firstOrNull())
        is FirElvisExpression -> readsThroughSafeCall(expression.lhs) || readsThroughSafeCall(expression.rhs)
        is FirWhenExpression -> expression.branches.any {
            readsThroughSafeCall(it.result.statements.lastOrNull() as? FirExpression)
        }
        is FirQualifiedAccessExpression -> readsThroughSafeCall(expression.explicitReceiver)
        else -> false
    }
}
