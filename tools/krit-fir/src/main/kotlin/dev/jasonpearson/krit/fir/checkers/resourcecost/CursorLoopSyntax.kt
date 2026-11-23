package dev.jasonpearson.krit.fir.checkers.resourcecost

import com.intellij.lang.LighterASTNode
import dev.jasonpearson.krit.fir.support.lightChildren
import dev.jasonpearson.krit.fir.support.lightSourceOf
import dev.jasonpearson.krit.fir.support.lightText
import dev.jasonpearson.krit.fir.support.qualifiedCall
import dev.jasonpearson.krit.fir.support.significantChildren
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtSourceElement

internal object CursorLoopSyntax {
    val lookupNames = setOf("getColumnIndex", "getColumnIndexOrThrow")

    fun descendants(source: KtSourceElement, node: LighterASTNode): Sequence<LighterASTNode> = sequence {
        yield(node)
        for (child in lightChildren(source, node)) yieldAll(descendants(source, child))
    }

    fun callee(source: KtSourceElement, call: LighterASTNode): LighterASTNode? =
        significantChildren(source, call).firstOrNull()?.takeIf { it.tokenType == KtNodeTypes.REFERENCE_EXPRESSION }

    fun callName(source: KtSourceElement, call: LighterASTNode): String? =
        callee(source, call)?.let { lightText(source, it) }

    fun receiverText(source: KtSourceElement, call: LighterASTNode): String {
        val qualified = qualifiedCall(lightSourceOf(call, source))
        val receiver = qualified?.let { significantChildren(source, it).firstOrNull() }
        return receiver?.let { lightText(source, it).trim() } ?: "cursor"
    }

    fun enclosingLoops(source: KtSourceElement, call: LighterASTNode): Sequence<LighterASTNode> =
        generateSequence(source.treeStructure.getParent(call)) { source.treeStructure.getParent(it) }
            .filter { it.tokenType == KtNodeTypes.WHILE }

    fun firstLookup(source: KtSourceElement, loop: LighterASTNode, constantCalls: Set<Int>): LighterASTNode? {
        val parts = lightChildren(source, loop)
        val condition = parts.firstOrNull { it.tokenType == KtNodeTypes.CONDITION }
        val body = parts.firstOrNull { it.tokenType == KtNodeTypes.BODY }
        if (condition == null || body == null) return null
        val iterates = descendants(source, condition).any {
            it.tokenType == KtNodeTypes.CALL_EXPRESSION && callName(source, it) == "moveToNext"
        }
        if (!iterates) return null
        val locals = CursorLoopLocals.names(source, body)
        return CursorLoopCallOrder.descendants(source, body).firstOrNull {
            it.tokenType == KtNodeTypes.CALL_EXPRESSION && callName(source, it) in lookupNames &&
                CursorLoopArguments.invariant(source, it, body, locals, constantCalls) &&
                CursorLoopReceivers.invariant(source, it, body, locals)
        }
    }

}
