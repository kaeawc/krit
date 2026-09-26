package dev.jasonpearson.krit.fir.checkers.androidlint

import com.intellij.lang.LighterASTNode
import com.intellij.openapi.util.Ref
import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirLiteralExpression
import org.jetbrains.kotlin.fir.expressions.FirStringConcatenationCall
import org.jetbrains.kotlin.fir.expressions.FirVarargArgumentsExpression
import org.jetbrains.kotlin.fir.expressions.resolvedArgumentMapping
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirFunctionSymbol
import org.jetbrains.kotlin.fir.types.constructClassLikeType
import org.jetbrains.kotlin.fir.types.isSubtypeOf
import org.jetbrains.kotlin.fir.types.resolvedType
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.types.ConstantValueKind

/**
 * Flags a `setText(...)` call on an `android.widget.TextView` (or a subtype:
 * Button, EditText, a project subclass) whose text argument is a string
 * literal: the text is hardcoded instead of coming from a string resource.
 *
 * Like the Go rule:
 * - the call must be named `setText` and its receiver must be a TextView. That
 *   covers TextView's own setters, a subclass's overload, and an extension
 *   `setText` on a TextView, whichever `setText` the call resolves to;
 * - the text argument must be a string literal or a string template
 *   (`"Hello"`, `""`, `"Count: $n"`). A resource id, a variable, a constant,
 *   a concatenation (`"a" + b`), or a call result is not reported;
 * - only calls count; assigning the synthetic property (`view.text = "x"`) is
 *   not a `setText` call and is not reported;
 * - a `setText` on something that is not a TextView (`Toast`, `RemoteViews`,
 *   notification builders, a project class) is not reported;
 * - the finding sits on the first line of the call expression.
 *
 * Deliberate differences from Go, each pinned in the golden data:
 * - Recall: the receiver is typed by resolution. Go types an explicit receiver
 *   with source inference and otherwise falls back to its name (`...TextView`,
 *   `...Button`, `...Text`, `tv`, `btn`); for a bare or `this.`/`super.` call
 *   it reads only the direct supertypes of the nearest enclosing class. So FIR
 *   also reports calls on the implicit or `this` receiver of a scope function
 *   (`tv.apply { setText("x") }`), in an extension on TextView, in an inner
 *   class (bare or `this@Outer.`) or a local of a TextView subclass, on a
 *   lambda parameter or a `findViewById` result, on an indirect subclass, on a
 *   TextView-bounded type parameter, and on a TextView chain rooted at a name
 *   Go treats as a non-View root (`AlertDialog`, `MenuItem`, ...).
 * - Recall: the text is the first argument not named in source, as in Go,
 *   and also the argument bound to the first parameter (a vararg's first
 *   element), so a parenthesized, annotated, labeled, or named literal still
 *   counts. Go needs a bare literal as the first unlabeled argument.
 * - Precision: a call Go accepts by name or by the enclosing class, where the
 *   receiver is not a TextView, is not reported: a variable whose name looks
 *   like a view (`titleText`, `button`) but whose type is not a TextView, a
 *   project class that is only named like a TextView, and a bare or `this.`
 *   call that resolves to another receiver's `setText` (inside
 *   `with(builder) { ... }` or an `object : Builder()` in a TextView subclass).
 */
internal object SetTextI18n : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "SetTextI18n"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(SetTextI18n)
    }

    private const val MESSAGE = "Do not pass hardcoded text to setText. Use resource strings with placeholders."
    private val SET_TEXT = Name.identifier("setText")

    // Nullable, so a nullable or platform-typed receiver (`tv?.setText`,
    // `findViewById<TextView>(id)`) is a subtype too.
    private val TEXT_VIEW = ClassId(FqName("android.widget"), Name.identifier("TextView"))
        .constructClassLikeType(emptyArray(), isMarkedNullable = true)

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val callee = expression.calleeReference.toResolvedCallableSymbol() as? FirFunctionSymbol<*> ?: return
        if (callee.name != SET_TEXT) return
        val mapping = expression.resolvedArgumentMapping ?: return
        val firstParameter = callee.valueParameterSymbols.firstOrNull() ?: return
        // The text is the argument bound to the first parameter (a vararg's
        // first element), or, as Go picks it, the first argument not named in
        // source: `setText(bold = true, "x")` passes "x" positionally.
        val bound = mapping.entries.firstOrNull { it.value.symbol == firstParameter }?.key
        val boundText = if (bound is FirVarargArgumentsExpression) bound.arguments.firstOrNull() else bound
        val hardcoded = (boundText != null && isStringLiteral(boundText)) ||
            mapping.keys
                .flatMap { if (it is FirVarargArgumentsExpression) it.arguments else listOf(it) }
                .any { isStringLiteral(it) && isFirstUnnamedArgument(it) }
        if (!hardcoded) return
        // The receiver whose text is set: the written receiver, else the
        // implicit one (an extension's receiver before a member's owner).
        val receiver = expression.explicitReceiver
            ?: expression.extensionReceiver
            ?: expression.dispatchReceiver
            ?: return
        if (!receiver.resolvedType.isSubtypeOf(TEXT_VIEW, context.session)) return
        report(expression.source, MESSAGE)
    }

    // A plain literal, a raw string, or a template. FIR drops the parentheses
    // around a literal, so a parenthesized one counts too. The raw FIR builder
    // also turns a `+` chain that starts with a string (`"a" + b`) into a
    // string concatenation, so only one whose source is a template counts.
    private fun isStringLiteral(expression: FirExpression): Boolean = when (expression) {
        is FirLiteralExpression -> expression.kind == ConstantValueKind.String
        is FirStringConcatenationCall -> expression.source?.elementType == KtNodeTypes.STRING_TEMPLATE
        else -> false
    }

    // The argument is the first value argument in the parentheses without a
    // `name =` label, as Go's flatPositionalValueArgument(args, 0) picks it.
    // Parentheses, annotations, and labels around the literal are skipped.
    private fun isFirstUnnamedArgument(argument: FirExpression): Boolean {
        val source = argument.source ?: return false
        val tree = source.treeStructure
        var node = source.lighterASTNode
        while (node.tokenType in argumentWrappers) node = tree.getParent(node) ?: return false
        if (node.tokenType != KtNodeTypes.VALUE_ARGUMENT) return false
        val list = tree.getParent(node) ?: return false
        if (list.tokenType != KtNodeTypes.VALUE_ARGUMENT_LIST) return false
        val first = children(source, list).firstOrNull {
            it.tokenType == KtNodeTypes.VALUE_ARGUMENT &&
                children(source, it).none { child -> child.tokenType == KtNodeTypes.VALUE_ARGUMENT_NAME }
        }
        return first != null && first.startOffset == node.startOffset && first.endOffset == node.endOffset
    }

    private val argumentWrappers = setOf(
        KtNodeTypes.STRING_TEMPLATE,
        KtNodeTypes.PARENTHESIZED,
        KtNodeTypes.ANNOTATED_EXPRESSION,
        KtNodeTypes.LABELED_EXPRESSION,
    )

    private fun children(source: KtSourceElement, node: LighterASTNode): List<LighterASTNode> {
        val ref = Ref<Array<LighterASTNode?>>()
        source.treeStructure.getChildren(node, ref)
        return ref.get()?.filterNotNull().orEmpty()
    }
}
