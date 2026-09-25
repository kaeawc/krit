package dev.jasonpearson.krit.fir.checkers.coroutines

import com.intellij.lang.LighterASTNode
import com.intellij.openapi.util.Ref
import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.declarations.FirProperty
import org.jetbrains.kotlin.fir.declarations.utils.isCompanion
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirLiteralExpression
import org.jetbrains.kotlin.fir.expressions.FirPropertyAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirSmartCastExpression
import org.jetbrains.kotlin.fir.expressions.FirWrappedArgumentExpression
import org.jetbrains.kotlin.fir.expressions.resolvedArgumentMapping
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.symbols.SymbolInternals
import org.jetbrains.kotlin.fir.symbols.impl.FirNamedFunctionSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirPropertySymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirRegularClassSymbol
import org.jetbrains.kotlin.fir.types.isAnyOrNullableAny
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid
import org.jetbrains.kotlin.lexer.KtTokens
import org.jetbrains.kotlin.text

// Flags `synchronized(lock) { }` whose lock is a boxed primitive: a
// primitive literal (`synchronized(1)`, `synchronized(true)`, `synchronized('c')`)
// or a property declared with a primitive type (`val count: Int`). Boxed
// primitives are cached and shared (Integer.valueOf), so unrelated code can
// end up contending on, or deadlocking over, the same monitor.
//
// Mirrors the Go SynchronizedOnBoxedPrimitive rule:
// - The call is written `synchronized(...)` (optionally qualified). The lock
//   is its first positional argument; a named `lock = ...` argument, or a
//   parenthesized, negated, or otherwise compound expression, is not
//   inspected.
// - Literal lock: decimal integer, long (`1L`, `0x1L`), floating-point,
//   boolean, and character literals. Hex/binary integers without an `L`
//   suffix and unsigned literals are not tree-sitter integer/long literals,
//   so Go does not flag them either. Literals are flagged anywhere.
// - Identifier lock: a bare name that resolves to a property declared (as a
//   member or local, at any depth) inside the nearest enclosing class or
//   object declaration (a companion object counts as its outer class, as in
//   Go). Its declared type is read from the declaration text exactly as Go
//   does: the text after the first `:`, cut at `=`, with `?` and type
//   arguments dropped, must be one of the eight primitive names, which is
//   also the name the message reports. Parameters, primary-constructor
//   properties, top-level properties, and inferred types are not flagged.
//
// Deliberate precision fixes over Go, which matches by name only:
// - The callee must be a monitor-lock function: named `synchronized` with a
//   first parameter of type Any (after typealias expansion). That covers
//   kotlin.synchronized, atomicfu's JVM `synchronized(SynchronizedObject)`
//   (a typealias of Any), and multiplatform `expect`/`actual` wrappers; a
//   lookalike whose first parameter is a specific type is ignored.
// - The identifier must resolve to that property, so a parameter or local
//   that shadows a boxed-primitive property is ignored.
//
// From language version 2.1, K2 itself rejects kotlin.synchronized on a
// primitive (SYNCHRONIZED_BLOCK_ON_VALUE_CLASS_OR_PRIMITIVE), so a file with
// such a call does not compile cleanly and Go stays authoritative for it.
// This checker still reports it wherever the file does compile: older
// language versions, where that diagnostic is a warning, and wrappers the
// compiler check does not know about.
internal object SynchronizedOnBoxedPrimitive : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "SynchronizedOnBoxedPrimitive"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(SynchronizedOnBoxedPrimitive)
    }

    private const val SYNCHRONIZED = "synchronized"

    private val boxedPrimitiveTypes = setOf("Int", "Long", "Short", "Byte", "Float", "Double", "Boolean", "Char")

    private const val LITERAL_MESSAGE =
        "synchronized() on a boxed primitive literal. Boxed primitives have identity-equality surprises. Use a dedicated Any() object."

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val callee = expression.calleeReference.toResolvedCallableSymbol() as? FirNamedFunctionSymbol ?: return
        if (callee.name.asString() != SYNCHRONIZED) return
        // Go matches the call by its written name, so an import alias is not a
        // synchronized() call there.
        if (expression.calleeReference.source?.text?.toString() != SYNCHRONIZED) return

        val lockParameter = callee.valueParameterSymbols.firstOrNull() ?: return
        if (!lockParameter.resolvedReturnType.fullyExpandedType().isAnyOrNullableAny) return
        val lockArgument = expression.resolvedArgumentMapping
            ?.entries
            ?.firstOrNull { it.value.name == lockParameter.name }
            ?.key ?: return
        if (lockArgument is FirWrappedArgumentExpression) return
        val lock = if (lockArgument is FirSmartCastExpression) lockArgument.originalExpression else lockArgument
        val lockSource = lock.source ?: return
        if (lockSource.kind !is KtRealSourceElementKind) return
        if (!isUnnamedValueArgument(lockSource)) return

        if (isBoxedPrimitiveLiteral(lock, lockSource)) {
            report(expression.source, LITERAL_MESSAGE)
            return
        }

        val typeName = boxedPropertyTypeName(lock, lockSource) ?: return
        report(
            expression.source,
            "synchronized() on a boxed primitive ($typeName). Boxed primitives have identity-equality surprises. Use a dedicated Any() object.",
        )
    }

    // The lock expression is the whole argument (not wrapped in parentheses or
    // an operator) and the argument carries no `name =` label.
    private fun isUnnamedValueArgument(source: KtSourceElement): Boolean {
        val tree = source.treeStructure
        val parent = tree.getParent(source.lighterASTNode) ?: return false
        if (parent.tokenType != KtNodeTypes.VALUE_ARGUMENT) return false
        return children(source, parent).none { it.tokenType == KtNodeTypes.VALUE_ARGUMENT_NAME }
    }

    private fun isBoxedPrimitiveLiteral(lock: FirExpression, source: KtSourceElement): Boolean {
        if (lock !is FirLiteralExpression) return false
        return when (source.elementType) {
            KtNodeTypes.BOOLEAN_CONSTANT, KtNodeTypes.CHARACTER_CONSTANT, KtNodeTypes.FLOAT_CONSTANT -> true
            KtNodeTypes.INTEGER_CONSTANT -> isDecimalOrLongLiteral(source.text?.toString() ?: return false)
            else -> false
        }
    }

    // tree-sitter's integer_literal is decimal only and its long_literal is
    // any integer (decimal, hex, binary) with an `L` suffix; unsigned
    // literals are a separate kind.
    private fun isDecimalOrLongLiteral(text: String): Boolean {
        if (text.endsWith("u") || text.endsWith("U") || text.endsWith("uL") || text.endsWith("UL")) return false
        if (text.endsWith("L")) return true
        val lower = text.lowercase()
        return !lower.startsWith("0x") && !lower.startsWith("0b")
    }

    context(context: CheckerContext)
    private fun boxedPropertyTypeName(lock: FirExpression, source: KtSourceElement): String? {
        if (lock !is FirPropertyAccessExpression) return null
        if (lock.explicitReceiver != null) return null
        if (source.elementType != KtNodeTypes.REFERENCE_EXPRESSION) return null
        val property = lock.calleeReference.toResolvedCallableSymbol() as? FirPropertySymbol ?: return null
        val propertySource = property.source ?: return null
        if (propertySource.kind !is KtRealSourceElementKind) return null
        if (propertySource.elementType != KtNodeTypes.PROPERTY) return null

        val owner = context.containingDeclarations
            .filterIsInstance<FirRegularClassSymbol>()
            .lastOrNull { !it.isCompanion } ?: return null
        if (!declaresProperty(owner, property)) return null

        val typeName = declaredTypeText(propertySource) ?: return null
        return typeName.takeIf { it in boxedPrimitiveTypes }
    }

    // True when [property] is declared anywhere inside [owner]'s body: a
    // member, a member of a nested class or companion, or a local in one of
    // its functions, initializers, lambdas, or object literals.
    @OptIn(SymbolInternals::class)
    private fun declaresProperty(owner: FirRegularClassSymbol, property: FirPropertySymbol): Boolean {
        var found = false
        owner.fir.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                if (found) return
                if (element is FirProperty && element.symbol == property) {
                    found = true
                    return
                }
                element.acceptChildren(this)
            }
        })
        return found
    }

    // Go's resolvePropertyTypeInScope: the declaration text (modifiers
    // through the last token, without attached comments) after the first
    // `:`, cut at the first `=`, without a trailing `?` or type arguments.
    private fun declaredTypeText(source: KtSourceElement): String? {
        val node = source.lighterASTNode
        val tokens = children(source, node).filter {
            it.tokenType !in KtTokens.WHITESPACES && it.tokenType !in KtTokens.COMMENTS
        }
        val first = tokens.firstOrNull() ?: return null
        val last = tokens.last()
        val nodeText = source.treeStructure.toString(node)
        val start = first.startOffset - node.startOffset
        val end = last.endOffset - node.startOffset
        if (start < 0 || end > nodeText.length || start > end) return null
        val text = nodeText.substring(start, end)

        val colon = text.indexOf(':')
        if (colon < 0) return null
        var afterColon = text.substring(colon + 1).trim()
        val eq = afterColon.indexOf('=')
        if (eq >= 0) afterColon = afterColon.substring(0, eq).trim()
        afterColon = afterColon.removeSuffix("?")
        val lt = afterColon.indexOf('<')
        if (lt >= 0) afterColon = afterColon.substring(0, lt)
        return afterColon.trim()
    }

    private fun children(source: KtSourceElement, node: LighterASTNode): List<LighterASTNode> {
        val ref = Ref<Array<LighterASTNode?>>()
        source.treeStructure.getChildren(node, ref)
        return ref.get()?.filterNotNull().orEmpty()
    }
}
