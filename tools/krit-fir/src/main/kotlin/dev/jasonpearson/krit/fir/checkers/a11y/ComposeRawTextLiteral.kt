package dev.jasonpearson.krit.fir.checkers.a11y

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.checkers.compose.isPreviewScaffolding
import dev.jasonpearson.krit.fir.checkers.compose.isPrivateHelperOfPreviews
import dev.jasonpearson.krit.fir.checkers.compose.previewConfig
import dev.jasonpearson.krit.fir.containingScanPath
import dev.jasonpearson.krit.fir.isInTestFile
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.pathTo
import dev.jasonpearson.krit.fir.support.unwrapArgument
import dev.jasonpearson.krit.fir.support.unwrapSmartCasts
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.declarations.FirNamedFunction
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirLiteralExpression
import org.jetbrains.kotlin.fir.expressions.FirPropertyAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirStringConcatenationCall
import org.jetbrains.kotlin.fir.expressions.FirWhenBranch
import org.jetbrains.kotlin.fir.expressions.FirWhenExpression
import org.jetbrains.kotlin.fir.expressions.resolvedArgumentMapping
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.symbols.SymbolInternals
import org.jetbrains.kotlin.fir.symbols.impl.FirPropertySymbol
import org.jetbrains.kotlin.name.CallableId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.types.ConstantValueKind

// Flags `Text("Welcome")`: a Compose Text whose text is a string literal with letters in it, so it cannot be
// translated. Use `stringResource()`.
//
// Mirrors the Go rule. It stays quiet for test, debug, playground, sample and benchmark sources, preview and
// sample scaffolding, private helpers only previews call, and a branch that runs only under
// `LocalInspectionMode.current`. Where Go matches the written name `Text`, this resolves the call to an AndroidX
// Text, so a design-system `Text` reached through a wildcard import or the same package is not reported, and it
// reads the letters from the string's value, so an escape such as `\n` is not text.
internal object ComposeRawTextLiteral : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "ComposeRawTextLiteral"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(ComposeRawTextLiteral)
    }

    private const val MESSAGE = "Text() uses a hardcoded string literal. Use `stringResource()` for internationalization and accessibility."

    private val inspectionMode = CallableId(FqName("androidx.compose.ui.platform"), Name.identifier("LocalInspectionMode"))
    private val nonProductionMarkers = listOf(
        "playground", "sandbox", "/debug", "/samples/", "/sample/", "/demos/", "/demo/", "benchmark",
        "/ui-tooling/", "/test-utils/", "-test-utils/",
    )

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val callee = expression.calleeReference.toResolvedCallableSymbol() ?: return
        val id = callee.callableId ?: return
        if (id.classId != null || id.callableName.asString() != "Text" || !id.packageName.asString().startsWith("androidx.")) return
        val literal = expression.resolvedArgumentMapping?.entries?.firstOrNull { it.value.name.asString() == "text" }?.key
        val text = literal?.let { literalText(unwrapArgument(it)) } ?: return
        // Strings without a letter (spaces, bullets, separators) have nothing to translate.
        if (text.none { Character.isLetter(it) }) return
        if (isNonProductionSource()) return

        val path = pathTo(expression) ?: return
        val ancestors = path.dropLast(1).asReversed()
        val functionIndex = ancestors.indexOfFirst { it is FirNamedFunction }
        if (functionIndex < 0) return
        val function = ancestors[functionIndex] as FirNamedFunction
        if (isPrivateHelperOfPreviews(function) || function.isPreviewScaffolding(previewConfig())) return
        if (insideInspectionBranch(expression, ancestors.subList(0, functionIndex))) return
        report(expression.source, MESSAGE)
    }

    // The literal parts of a string or template, or null when the argument is not a string literal.
    private fun literalText(argument: FirExpression): String? = when {
        argument is FirLiteralExpression && argument.kind == ConstantValueKind.String -> argument.value as? String
        argument is FirStringConcatenationCall -> argument.argumentList.arguments.joinToString("") { part ->
            (part as? FirLiteralExpression)?.takeIf { it.kind == ConstantValueKind.String }?.value as? String ?: ""
        }
        else -> null
    }

    context(context: CheckerContext)
    private fun isNonProductionSource(): Boolean {
        if (isInTestFile()) return true
        val path = containingScanPath()?.replace('\\', '/')?.lowercase() ?: return false
        val extra = (config()["nonProductionPathMarkers"] as? List<*>).orEmpty().filterIsInstance<String>()
        return nonProductionMarkers.any { it in path } || extra.any { it.isNotEmpty() && it.lowercase() in path }
    }

    // True when the call sits in the branch of an `if` that runs only under `LocalInspectionMode.current`: the
    // then-branch of the read, or the else-branch of its negation. [ancestors] run nearest first up to the function.
    private fun insideInspectionBranch(call: FirFunctionCall, ancestors: List<FirElement>): Boolean {
        for (index in ancestors.indices) {
            val whenExpression = ancestors[index] as? FirWhenExpression ?: continue
            if (whenExpression.source?.lighterASTNode?.tokenType != KtNodeTypes.IF) continue
            val branch = ancestors.getOrNull(index - 1) as? FirWhenBranch ?: continue
            val inside = ancestors.getOrNull(index - 2) ?: call
            if (inside === branch.condition) continue
            val first = whenExpression.branches.firstOrNull() ?: continue
            if (branch === first && isInspectionRead(first.condition)) return true
            if (branch !== first && isNegatedInspectionRead(first.condition)) return true
        }
        return false
    }

    private fun isNegatedInspectionRead(condition: FirExpression): Boolean {
        val not = unwrapSmartCasts(condition) as? FirFunctionCall ?: return false
        return not.calleeReference.name.asString() == "not" && isInspectionRead(not.explicitReceiver)
    }

    // `LocalInspectionMode.current`, or a local val initialized with it.
    @OptIn(SymbolInternals::class)
    private fun isInspectionRead(expression: FirExpression?): Boolean {
        val access = unwrapSmartCasts(expression) as? FirPropertyAccessExpression ?: return false
        val symbol = access.calleeReference.toResolvedCallableSymbol() ?: return false
        if (symbol is FirPropertySymbol && symbol.isLocal) return isInspectionRead(symbol.fir.initializer)
        if (symbol.callableId?.callableName?.asString() != "current") return false
        val owner = unwrapSmartCasts(access.explicitReceiver) as? FirPropertyAccessExpression ?: return false
        return owner.calleeReference.toResolvedCallableSymbol()?.callableId == inspectionMode
    }
}
