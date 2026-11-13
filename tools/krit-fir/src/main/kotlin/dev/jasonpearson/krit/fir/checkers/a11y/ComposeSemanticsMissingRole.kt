package dev.jasonpearson.krit.fir.checkers.a11y

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.checkers.compose.isPreviewScaffolding
import dev.jasonpearson.krit.fir.checkers.compose.isPrivateHelperOfPreviews
import dev.jasonpearson.krit.fir.checkers.compose.isTestSupportSource
import dev.jasonpearson.krit.fir.checkers.compose.previewConfig
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.pathTo
import dev.jasonpearson.krit.fir.support.unwrapArgument
import dev.jasonpearson.krit.fir.support.writtenArgumentName
import org.jetbrains.kotlin.descriptors.ClassKind
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.declarations.FirNamedFunction
import org.jetbrains.kotlin.fir.declarations.FirRegularClass
import org.jetbrains.kotlin.fir.expressions.FirArgumentList
import org.jetbrains.kotlin.fir.expressions.FirBlock
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirLiteralExpression
import org.jetbrains.kotlin.fir.expressions.FirNamedArgumentExpression
import org.jetbrains.kotlin.fir.expressions.FirQualifiedAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirResolvedQualifier
import org.jetbrains.kotlin.fir.expressions.FirVarargArgumentsExpression
import org.jetbrains.kotlin.fir.expressions.FirWhenBranch
import org.jetbrains.kotlin.fir.expressions.FirWhenExpression
import org.jetbrains.kotlin.fir.expressions.resolvedArgumentMapping
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid
import org.jetbrains.kotlin.name.CallableId
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.types.ConstantValueKind

// Flags `Modifier.clickable { }`, `.toggleable(...)` and `.selectable(...)` written on a chain that starts at
// `Modifier` and gives screen readers no role.
//
// Mirrors the Go rule. It stays quiet for test and fake sources, preview and sample scaffolding, private helpers
// only previews call, `enabled = false`, `indication = null` (an invisible scrim), a `role` argument, and a chain
// that sets a role in `semantics { }` or hides itself from accessibility. Where Go matches the written names, this
// resolves the modifier to Compose's foundation function and the chain root to `androidx.compose.ui.Modifier`, and
// it reads the chain exemptions from resolved `semantics`, `role` and hiding calls instead of searching the chain's
// text for the words.
internal object ComposeSemanticsMissingRole : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "ComposeSemanticsMissingRole"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(ComposeSemanticsMissingRole)
    }

    private val foundation = FqName("androidx.compose.foundation")
    private val selection = FqName("androidx.compose.foundation.selection")
    private val semanticsPackage = FqName("androidx.compose.ui.semantics")

    private val interactionModifiers = setOf(
        CallableId(foundation, Name.identifier("clickable")),
        CallableId(selection, Name.identifier("toggleable")),
        CallableId(selection, Name.identifier("selectable")),
    )
    private val semanticsCall = CallableId(semanticsPackage, Name.identifier("semantics"))
    private val roleProperty = CallableId(semanticsPackage, Name.identifier("role"))
    private val hidingNames = setOf("hideFromAccessibility", "invisibleToUser", "clearAndSetSemantics")

    private val modifierClass = ClassId.topLevel(FqName("androidx.compose.ui.Modifier"))
    private val modifierRoots = setOf(modifierClass, modifierClass.createNestedClassId(Name.identifier("Companion")))

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val callee = expression.calleeReference.toResolvedCallableSymbol() ?: return
        if (callee.callableId !in interactionModifiers) return
        if (!rootedAtModifier(expression.explicitReceiver)) return
        if (isTestSupportSource() || inFakeOrTestClass()) return
        if (hasExemptArgument(expression)) return

        val path = pathTo(expression) ?: return
        val ancestors = path.dropLast(1).asReversed()
        val function = ancestors.firstOrNull { it is FirNamedFunction } as? FirNamedFunction
        if (function != null && (function.isPreviewScaffolding(previewConfig()) || isPrivateHelperOfPreviews(function))) return
        if (chainIsExempt(outermostChain(expression, ancestors))) return

        report(
            expression.calleeReference.source ?: expression.source,
            "Modifier.${callee.name} without an explicit `role`. Add `role = Role.X` or a `Modifier.semantics { role = ... }` for screen readers.",
        )
    }

    // The receiver chain ends at `Modifier` or its companion: `Modifier.padding(8.dp).clickable { }`. A chain that starts at
    // a `modifier` parameter or any other value is not reported.
    private fun rootedAtModifier(receiver: FirExpression?): Boolean {
        var current = receiver
        while (current is FirQualifiedAccessExpression) current = current.explicitReceiver
        return current is FirResolvedQualifier && current.qualifierSymbol?.classId in modifierRoots
    }

    // Go skips only the nearest enclosing class, and not an object, named Fake* or containing Test.
    context(context: CheckerContext)
    private fun inFakeOrTestClass(): Boolean {
        val owner = context.containingElements.filterIsInstance<FirRegularClass>()
            .lastOrNull { it.classKind != ClassKind.OBJECT } ?: return false
        val name = owner.name.asString()
        return name.startsWith("Fake") || "Test" in name
    }

    // A written `enabled = false` or `indication = null` argument, or any argument bound to the `role` parameter.
    private fun hasExemptArgument(call: FirFunctionCall): Boolean {
        val mapping = call.resolvedArgumentMapping ?: return false
        for ((argument, parameter) in mapping) {
            val value = unwrapArgument(argument)
            val written = writtenArgumentName(argument)
            when (parameter.name.asString()) {
                "role" -> return true
                "enabled" -> if (written == "enabled" && value is FirLiteralExpression && value.value == false) return true
                "indication" -> if (written == "indication" && value is FirLiteralExpression && value.kind == ConstantValueKind.Null) return true
            }
        }
        return false
    }

    // The whole dotted chain the call sits in. A chain handed to `then(...)` belongs to the chain of that call.
    private fun outermostChain(call: FirFunctionCall, ancestors: List<FirElement>): FirElement {
        var current: FirElement = call
        var index = 0
        while (true) {
            while (index < ancestors.size) {
                val parent = ancestors[index]
                if (parent !is FirQualifiedAccessExpression || parent.explicitReceiver !== current) break
                current = parent
                index++
            }
            var next = index
            while (next < ancestors.size && isArgumentWrapper(ancestors[next])) next++
            val owner = ancestors.getOrNull(next) as? FirFunctionCall ?: return current
            if (owner.calleeReference.name.asString() != "then") return current
            current = owner
            index = next + 1
        }
    }

    // Elements between a chain and the `then` call it is passed to: argument lists and the branches of an if or when.
    private fun isArgumentWrapper(element: FirElement): Boolean =
        element is FirArgumentList || element is FirNamedArgumentExpression || element is FirVarargArgumentsExpression ||
            element is FirWhenExpression || element is FirWhenBranch || element is FirBlock

    // The chain sets a role in a `semantics { }` block, or hides or replaces its semantics (a scrim).
    private fun chainIsExempt(chain: FirElement): Boolean {
        var hasSemantics = false
        var setsRole = false
        var hides = false
        chain.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                if (element is FirQualifiedAccessExpression) {
                    val id = element.calleeReference.toResolvedCallableSymbol()?.callableId
                    when {
                        id == semanticsCall -> hasSemantics = true
                        id == roleProperty -> setsRole = true
                        id?.packageName == semanticsPackage && id.callableName.asString() in hidingNames -> hides = true
                    }
                }
                element.acceptChildren(this)
            }
        })
        return hides || (hasSemantics && setsRole)
    }
}
