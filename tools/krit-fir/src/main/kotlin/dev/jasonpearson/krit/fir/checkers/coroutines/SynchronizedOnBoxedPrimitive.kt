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
import org.jetbrains.kotlin.fir.expressions.FirImplicitInvokeCall
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
import org.jetbrains.kotlin.fir.types.ConeClassLikeType
import org.jetbrains.kotlin.fir.types.ConeKotlinType
import org.jetbrains.kotlin.fir.types.ConeTypeParameterType
import org.jetbrains.kotlin.fir.types.isAnyOrNullableAny
import org.jetbrains.kotlin.fir.types.lowerBoundIfFlexible
import org.jetbrains.kotlin.fir.types.resolvedType
import org.jetbrains.kotlin.fir.unwrapFakeOverrides
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid
import org.jetbrains.kotlin.lexer.KtTokens
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.text

// Flags `synchronized(lock) { }` whose lock is a boxed primitive: a
// primitive literal (`synchronized(1)`, `synchronized(true)`, `synchronized('c')`)
// or a property declared with a primitive type (`val count: Int`). Boxed
// primitives are cached and shared (Integer.valueOf), so unrelated code can
// end up contending on, or deadlocking over, the same monitor.
//
// Mirrors the Go SynchronizedOnBoxedPrimitive rule:
// - The call is written `synchronized(...)` (optionally qualified), whatever
//   it resolves to: kotlin.synchronized, a wrapper, an import alias into that
//   name, or a value named synchronized called through `invoke`. The lock is
//   its first positional argument; a named `lock = ...` argument, or a
//   parenthesized, negated, or otherwise compound expression, is not
//   inspected.
// - Literal lock: decimal integer, long (`1L`, `0x1L`), floating-point,
//   boolean, and character literals. Hex/binary integers without an `L`
//   suffix and unsigned literals are not tree-sitter integer/long literals,
//   so Go does not flag them either. Literals are flagged anywhere.
// - Identifier lock: a bare name inside a class or object declaration (a
//   companion object counts as its outer class, as in Go). Go looks the name
//   up among the property declarations (members or locals, at any depth) of
//   the nearest enclosing class or object and reads the first typed one's
//   type from its text: after the first `:`, cut at `=`, with `?` and type
//   arguments dropped, it must be one of the eight primitive names.
//   Parameters, primary-constructor properties, top-level properties, and
//   inferred types are never that declaration.
//
// Where resolution and Go's name lookup disagree, the lock's resolved type
// decides. A finding needs the lock to really be a boxed primitive, and then
// either Go's lookup or the property the name resolves to (declared in that
// class, with a primitive type written in its text) to say so. The message
// names the lock's real type. So:
// - A parameter, local, or implicit-receiver property that shadows a
//   boxed-primitive property is dropped when it is not a primitive itself,
//   and kept (as Go reports it) when it is.
// - Findings Go misses are added only where the resolved property is
//   primitive-typed: a later same-named declaration than the one Go picks,
//   or a typed when-subject variable, which tree-sitter does not parse as a
//   property declaration.
//
// Deliberate precision fix over Go: the callee must be a monitor-lock
// function, whose first parameter is Any or Any? after typealias expansion,
// or a type parameter bounded only by those. That covers kotlin.synchronized,
// atomicfu's JVM `synchronized(SynchronizedObject)` (a typealias of Any), and
// multiplatform or generic wrappers; a lookalike whose first parameter is a
// specific type is ignored. This matches on the written name and the lock
// parameter's type instead of a callableId on purpose: Go matches by name,
// and those wrappers share no callableId.
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

    private val kotlinPackage = FqName("kotlin")

    private const val LITERAL_MESSAGE =
        "synchronized() on a boxed primitive literal. Boxed primitives have identity-equality surprises. Use a dedicated Any() object."

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val callee = expression.calleeReference.toResolvedCallableSymbol() as? FirNamedFunctionSymbol ?: return
        if (writtenCalleeName(expression) != SYNCHRONIZED) return

        val lockParameter = callee.valueParameterSymbols.firstOrNull() ?: return
        if (!isMonitorType(lockParameter.resolvedReturnType)) return
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

        val typeName = boxedIdentifierTypeName(lock, lockArgument.resolvedType, lockSource) ?: return
        report(
            expression.source,
            "synchronized() on a boxed primitive ($typeName). Boxed primitives have identity-equality surprises. Use a dedicated Any() object.",
        )
    }

    // The name the call is written with, as Go reads it: the callee name, or
    // for `synchronized(...)` resolved to `synchronized.invoke(...)`, the name
    // of the value being invoked.
    private fun writtenCalleeName(expression: FirFunctionCall): String? {
        val nameSource = if (expression is FirImplicitInvokeCall) {
            val receiver = expression.explicitReceiver as? FirPropertyAccessExpression ?: return null
            receiver.calleeReference.source
        } else {
            expression.calleeReference.source
        }
        return nameSource?.text?.toString()
    }

    // A monitor-lock parameter accepts any object: Any or Any? (after
    // typealias expansion), or a type parameter whose bounds are only those.
    context(context: CheckerContext)
    private fun isMonitorType(type: ConeKotlinType): Boolean {
        val expanded = type.fullyExpandedType()
        if (expanded.isAnyOrNullableAny) return true
        val typeParameter = expanded as? ConeTypeParameterType ?: return false
        return typeParameter.lookupTag.typeParameterSymbol.resolvedBounds
            .all { it.coneType.fullyExpandedType().isAnyOrNullableAny }
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

    // The primitive name for a bare identifier lock inside a class or object,
    // or null when it is not flagged (see the header comment).
    context(context: CheckerContext)
    private fun boxedIdentifierTypeName(lock: FirExpression, lockType: ConeKotlinType, source: KtSourceElement): String? {
        if (lock !is FirPropertyAccessExpression) return null
        if (lock.explicitReceiver != null) return null
        if (source.elementType != KtNodeTypes.REFERENCE_EXPRESSION) return null
        val primitive = primitiveName(lockType) ?: return null
        val name = source.text?.toString() ?: return null

        val owner = context.containingDeclarations
            .filterIsInstance<FirRegularClassSymbol>()
            .lastOrNull { !it.isCompanion } ?: return null
        val declarations = namedPropertyDeclarations(owner, name)

        val goType = declarations
            .filter { !isWhenSubject(it) }
            .firstNotNullOfOrNull { declaration -> declaredTypeText(declaration.source!!)?.takeIf { it.isNotEmpty() } }
        if (goType in boxedPrimitiveTypes) return primitive

        val resolved = (lock.calleeReference.toResolvedCallableSymbol() as? FirPropertySymbol)?.unwrapFakeOverrides()
            ?: return null
        val resolvedDeclaration = declarations.firstOrNull { it.symbol == resolved } ?: return null
        return primitive.takeIf { declaredTypeText(resolvedDeclaration.source!!) in boxedPrimitiveTypes }
    }

    // The short name of kotlin.Int, kotlin.Long, ... for [type], ignoring
    // nullability and platform flexibility, or null for any other type.
    context(context: CheckerContext)
    private fun primitiveName(type: ConeKotlinType): String? {
        val classType = type.fullyExpandedType().lowerBoundIfFlexible() as? ConeClassLikeType ?: return null
        val classId = classType.lookupTag.classId
        if (classId.packageFqName != kotlinPackage || classId.isNestedClass) return null
        return classId.shortClassName.asString().takeIf { it in boxedPrimitiveTypes }
    }

    // Every source `val`/`var` declaration named [name] anywhere inside
    // [owner]'s body (members, members of nested classes and companions, and
    // locals in its functions, initializers, lambdas, and object literals),
    // in source order, as Go walks the class's property_declaration nodes.
    @OptIn(SymbolInternals::class)
    private fun namedPropertyDeclarations(owner: FirRegularClassSymbol, name: String): List<FirProperty> {
        val found = mutableListOf<FirProperty>()
        owner.fir.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                if (element is FirProperty && element.name.asString() == name) {
                    val source = element.source
                    if (source != null &&
                        source.kind is KtRealSourceElementKind &&
                        source.elementType == KtNodeTypes.PROPERTY
                    ) {
                        found += element
                    }
                }
                element.acceptChildren(this)
            }
        })
        return found.sortedBy { it.source!!.startOffset }
    }

    // `when (val n: Int = x)`: PSI makes the subject variable a PROPERTY, but
    // tree-sitter has no property_declaration for it, so Go's lookup skips it.
    private fun isWhenSubject(property: FirProperty): Boolean {
        val source = property.source ?: return false
        return source.treeStructure.getParent(source.lighterASTNode)?.tokenType == KtNodeTypes.WHEN
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
