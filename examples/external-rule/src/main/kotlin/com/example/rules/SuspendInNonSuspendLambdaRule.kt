package com.example.rules

import dev.jasonpearson.krit.api.Capability
import dev.jasonpearson.krit.api.Finding
import dev.jasonpearson.krit.api.KritFile
import dev.jasonpearson.krit.api.KritRuleInfo
import dev.jasonpearson.krit.api.Resolver
import dev.jasonpearson.krit.api.RuleContext
import dev.jasonpearson.krit.api.Severity
import dev.jasonpearson.krit.api.TypeAwareRule
import org.jetbrains.kotlin.lexer.KtTokens
import org.jetbrains.kotlin.psi.KtCallExpression
import org.jetbrains.kotlin.psi.KtFile
import org.jetbrains.kotlin.psi.KtFunction
import org.jetbrains.kotlin.psi.KtFunctionLiteral
import org.jetbrains.kotlin.psi.KtLambdaExpression
import org.jetbrains.kotlin.psi.KtModifierListOwner
import org.jetbrains.kotlin.psi.KtTreeVisitorVoid

/**
 * Type-aware example: flag suspend function invocations that sit inside
 * a non-suspend lambda. PSI alone cannot tell whether a call resolves
 * to a `suspend` function or whether an enclosing lambda was typed as
 * `suspend () -> R`, so the rule consults [Resolver] for both facts.
 * Demonstrates the [Capability.NEEDS_RESOLVER] contract end to end.
 *
 * Negative cases the rule must skip:
 *
 *  - calls inside a `suspend` named function — the compiler accepts
 *    them and reporting would just be noise.
 *  - calls inside a lambda typed `suspend () -> R` (e.g.
 *    `runBlocking { delay(10) }`) — the resolver reports the lambda's
 *    functional type, which is the only reliable signal.
 *  - non-suspend calls anywhere.
 */
@KritRuleInfo(
    id = "example.SuspendInNonSuspendLambda",
    category = "custom",
    severity = Severity.ERROR,
    needs = [Capability.NEEDS_RESOLVER],
)
class SuspendInNonSuspendLambdaRule : TypeAwareRule() {
    override fun check(
        file: KritFile,
        ctx: RuleContext,
        ktFile: KtFile,
        resolver: Resolver,
    ): List<Finding> {
        val findings = mutableListOf<Finding>()
        ktFile.accept(object : KtTreeVisitorVoid() {
            override fun visitCallExpression(expression: KtCallExpression) {
                super.visitCallExpression(expression)
                val resolved = resolver.resolveCall(expression) ?: return
                if (!resolved.isSuspend) return
                if (isInsideSuspendContext(expression, resolver)) return
                val offset = expression.textOffset
                val document = ktFile.viewProvider.document
                val line = document?.getLineNumber(offset)?.plus(1) ?: 1
                val column = if (document != null) {
                    offset - document.getLineStartOffset(line - 1) + 1
                } else {
                    1
                }
                findings.add(
                    Finding(
                        message = "Suspend function `${resolved.fqName}` called outside a suspend context",
                        line = line,
                        column = column,
                    ),
                )
            }
        })
        return findings
    }

    /**
     * Walks PSI ancestors looking for the closest enclosing function or
     * lambda. If that scope is a `suspend` named function, or the
     * resolver reports the lambda as `suspend`, the call is fine.
     */
    private fun isInsideSuspendContext(call: KtCallExpression, resolver: Resolver): Boolean {
        var element = call.parent
        while (element != null) {
            when {
                // A lambda's PSI shape is a KtLambdaExpression that
                // wraps a KtFunctionLiteral; the latter extends
                // KtFunction. We have to short-circuit on the literal
                // and forward the suspend-context check to the wrapping
                // lambda — otherwise the `is KtFunction` branch fires
                // first, sees no `suspend` modifier on the literal, and
                // treats the lambda as a shadowing non-suspend scope.
                element is KtFunctionLiteral -> {
                    val parent = element.parent
                    if (parent is KtLambdaExpression && resolver.isLambdaSuspend(parent)) {
                        return true
                    }
                    return false
                }
                element is KtFunction -> {
                    if ((element as KtModifierListOwner).hasModifier(KtTokens.SUSPEND_KEYWORD)) return true
                    // A non-suspend named function shadows any outer
                    // suspend context: a call here is in that
                    // function's body, not the enclosing lambda's.
                    return false
                }
            }
            element = element.parent
        }
        return false
    }
}
