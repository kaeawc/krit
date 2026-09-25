package dev.jasonpearson.krit.fir.checkers.potentialbugs

import com.intellij.lang.LighterASTNode
import com.intellij.util.diff.FlyweightCapableTreeStructure
import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirExpressionChecker
import org.jetbrains.kotlin.fir.expressions.FirCatch
import org.jetbrains.kotlin.fir.expressions.FirTryExpression
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.types.ConeClassLikeType
import org.jetbrains.kotlin.fir.types.ConeKotlinType
import org.jetbrains.kotlin.fir.types.coneType
import org.jetbrains.kotlin.fir.types.isSubtypeOf
import org.jetbrains.kotlin.fir.types.lowerBoundIfFlexible
import org.jetbrains.kotlin.util.getChildren

// Flags a catch clause that can never run because an earlier clause of the
// same `try` already catches its exception type (the same class, or a
// supertype of it).
//
// Mirrors the Go UnreachableCatchBlock rule. Go compares every earlier catch
// type with every later one: equal type text is a duplicate catch, otherwise it
// asks its resolver (a fixed table of well-known exception simple names, or the
// oracle's hierarchy) whether the later type is a subtype of the earlier one.
// Each (earlier, later) pair reports once on the later clause's `catch` line, so
// a clause shadowed by two earlier clauses gets two findings (one per distinct
// message; see check). FIR makes the same
// pairwise comparison on the resolved, alias-expanded catch types: a duplicate
// when both clauses catch the same type, unreachable when the later type is a
// proper subtype. The message names each type as it is written, like Go. Go's
// name table misses qualified names, aliases and project exceptions, and it
// holds one wrong edge (SocketTimeoutException is not a SocketException); it
// also reads a project class named like a well-known exception as that
// exception. The golden data pins both directions.
internal object UnreachableCatchBlock : FirExpressionChecker<FirTryExpression>(MppCheckerKind.Common), FirRule {
    override val ruleId = "UnreachableCatchBlock"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val tryExpressionCheckers = setOf(UnreachableCatchBlock)
    }

    private class CatchEntry(val source: KtSourceElement, val type: ConeKotlinType, val name: String)

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirTryExpression) {
        val catches = expression.catches.mapNotNull { entry(it) }
        for (j in catches.indices) {
            val child = catches[j]
            // Go emits one finding per pair, even when two pairs produce the
            // same message (`catch (e: A)` twice above a third `A`). An exact
            // repeat (same line, column and message) carries nothing new: the
            // compiler drops an equal diagnostic, and krit's FIR merge drops
            // exact repeats, so each distinct message is reported once.
            val messages = LinkedHashSet<String>()
            for (i in 0 until j) {
                val parent = catches[i]
                messages += when {
                    sameType(child.type, parent.type) ->
                        "Duplicate catch block for '${child.name}'."
                    child.type.isSubtypeOf(parent.type, context.session) ->
                        "Catch block for '${child.name}' is unreachable because '${parent.name}' is caught above."
                    else -> continue
                }
            }
            for (message in messages) report(child.source, message)
        }
    }

    // Class types compare by class id, which is only compared, never resolved
    // (resolving a local class id throws). Any other catch type, such as a
    // reified type parameter (K2 accepts `catch (e: T)` from language version
    // 2.4), is the same type when each is a subtype of the other, so
    // `catch (e: T)` twice stays a duplicate, as it is in Go.
    context(context: CheckerContext)
    private fun sameType(a: ConeKotlinType, b: ConeKotlinType): Boolean =
        if (a is ConeClassLikeType && b is ConeClassLikeType) {
            a.lookupTag.classId == b.lookupTag.classId
        } else {
            a.isSubtypeOf(b, context.session) && b.isSubtypeOf(a, context.session)
        }

    context(context: CheckerContext)
    private fun entry(catch: FirCatch): CatchEntry? {
        val source = catch.source ?: return null
        val typeRef = catch.parameter.returnTypeRef
        val type = typeRef.coneType.fullyExpandedType().lowerBoundIfFlexible()
        val name = typeRef.source?.let { writtenTypeName(it) } ?: return null
        return CatchEntry(source, type, name)
    }

    // The catch type as written, without annotations or type arguments: the
    // text Go interpolates (its tree-sitter user_type, cut at `<`).
    private fun writtenTypeName(source: KtSourceElement): String? {
        val tree = source.treeStructure
        val userType = firstUserType(source.lighterASTNode, tree) ?: return null
        val text = tree.toString(userType).toString().substringBefore('<').trim()
        return text.ifEmpty { null }
    }

    private fun firstUserType(node: LighterASTNode, tree: FlyweightCapableTreeStructure<LighterASTNode>): LighterASTNode? {
        if (node.tokenType == KtNodeTypes.USER_TYPE) return node
        if (node.tokenType == KtNodeTypes.MODIFIER_LIST) return null
        for (child in node.getChildren(tree)) {
            firstUserType(child, tree)?.let { return it }
        }
        return null
    }
}
