package dev.jasonpearson.krit.fir.checkers.coroutines

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import org.jetbrains.kotlin.KtFakeSourceElementKind
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.FirSession
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.DeclarationCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.FirPropertyChecker
import org.jetbrains.kotlin.fir.declarations.FirProperty
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirPropertyAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirReturnExpression
import org.jetbrains.kotlin.fir.expressions.FirSmartCastExpression
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.symbols.SymbolInternals
import org.jetbrains.kotlin.fir.symbols.impl.FirNamedFunctionSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirPropertySymbol
import org.jetbrains.kotlin.fir.types.classId
import org.jetbrains.kotlin.fir.types.lowerBoundIfFlexible
import org.jetbrains.kotlin.fir.types.resolvedType
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid
import org.jetbrains.kotlin.lexer.KtTokens
import org.jetbrains.kotlin.name.CallableId
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.name.SpecialNames
import org.jetbrains.kotlin.util.getChildren

/**
 * Port of the Go SharedFlowWithoutReplay rule: a `val`/`var` declaration
 * (top-level, member, or local) whose initializer, delegate, or accessors
 * create a kotlinx.coroutines.flow.MutableSharedFlow with no arguments at all
 * (`MutableSharedFlow<T>()`, or `MutableSharedFlow()` with an inferred type
 * argument), reported once per declaration on its first line (its modifier
 * list, or `val`/`var` when it has none), like Go. The message names the
 * declaration as written, backticks included, like Go.
 *
 * Covered like Go: the call anywhere inside the declaration (wrapped in
 * another call or a chain, inside a lambda, an anonymous object, or a
 * getter), so an enclosing declaration and a local one it contains are both
 * reported; destructuring declarations. Skipped like Go: calls that pass any
 * argument (`replay`, `extraBufferCapacity`, `onBufferOverflow`, positional),
 * primary-constructor parameters and their default values, function bodies and
 * expression bodies outside a property, `when (val x = ...)` subjects, and
 * loop variables.
 *
 * A call to a source factory function that returns a MutableSharedFlow counts
 * when a value the factory returns creates one with no arguments (the returned
 * expression itself, a local `val` it returns, or another such factory). Go
 * reports `val bus: MutableSharedFlow<Int> = lossy<Int>()` and
 * `createMutableSharedFlow()` by their spelling; FIR follows the factory body
 * instead, so it keeps those findings when the factory is lossy and drops them
 * when it passes replay or its body is not visible (an abstract or library
 * function).
 *
 * Deliberate precision differences from Go's substring match on the
 * declaration text (see the golden data): no finding for a declaration that
 * only mentions `MutableSharedFlow<` next to an unrelated `>()` call, a
 * factory that passes replay, a same-named local function, or the word in a
 * comment or string; and a finding where Go skips the whole declaration
 * because some other call in it is spelled `MutableSharedFlow(replay`,
 * `MutableSharedFlow(extraBufferCapacity`, or `MutableSharedFlow(` followed by
 * a line break (including an empty argument list split over two lines), where
 * the no-argument call is spelled through an import alias, with whitespace
 * inside `()`, or through a lossy factory Go cannot see, or where it sits in a
 * getter or setter on the line after the property (tree-sitter parses that
 * accessor as a sibling of the declaration).
 */
internal object SharedFlowWithoutReplay : FirPropertyChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "SharedFlowWithoutReplay"
    override val declarationCheckers = object : DeclarationCheckers() {
        override val propertyCheckers = setOf(SharedFlowWithoutReplay)
    }

    private val flowPackage = FqName("kotlinx.coroutines.flow")
    private val mutableSharedFlow = CallableId(flowPackage, Name.identifier("MutableSharedFlow"))
    private val mutableSharedFlowClass = ClassId(flowPackage, Name.identifier("MutableSharedFlow"))

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(declaration: FirProperty) {
        val source = declaration.source ?: return
        val destructuring = isDestructuring(declaration, source)
        if (!destructuring && !isPropertyStatement(source)) return
        if (!createsDefaultSharedFlow(declaration, context.session)) return
        // Go names the declaration by its first direct identifier child, as
        // written; a destructuring declaration has none, so Go's message
        // names ''.
        val name = if (destructuring) "" else nameText(source) ?: declaration.name.asString()
        report(source, "MutableSharedFlow '$name' created without replay or extraBufferCapacity. Default config is lossy.")
    }

    // A real `val`/`var` declaration statement: Go's property_declaration.
    // Loop variables, destructuring entries, and catch parameters have other
    // source element types; a `when (val x = ...)` subject is a PROPERTY in
    // Kotlin's tree but not a property_declaration in Go's.
    private fun isPropertyStatement(source: KtSourceElement): Boolean {
        if (source.kind !is KtRealSourceElementKind) return false
        if (source.elementType != KtNodeTypes.PROPERTY) return false
        val parent = source.treeStructure.getParent(source.lighterASTNode) ?: return true
        return parent.tokenType != KtNodeTypes.WHEN
    }

    // The synthetic `<destruct>` holder of `val (a, b) = ...`: one Go
    // property_declaration. Its entries read `<destruct>.componentN()` and never
    // hold the call themselves. A destructuring lambda parameter has a fake
    // source and is not a declaration statement.
    private fun isDestructuring(declaration: FirProperty, source: KtSourceElement): Boolean =
        declaration.name == SpecialNames.DESTRUCT &&
            source.kind is KtRealSourceElementKind &&
            source.elementType == KtNodeTypes.DESTRUCTURING_DECLARATION

    // The property name as written, backticks included, which is what Go
    // interpolates.
    private fun nameText(source: KtSourceElement): String? {
        val tree = source.treeStructure
        val identifier = source.lighterASTNode.getChildren(tree)
            .firstOrNull { it.tokenType == KtTokens.IDENTIFIER } ?: return null
        return tree.toString(identifier).toString()
    }

    private fun createsDefaultSharedFlow(declaration: FirProperty, session: FirSession): Boolean {
        val finder = DefaultSharedFlowFinder(session, emptySet())
        declaration.initializer?.accept(finder)
        declaration.delegate?.accept(finder)
        declaration.getter?.takeIf { it.source?.kind !is KtFakeSourceElementKind }?.accept(finder)
        declaration.setter?.takeIf { it.source?.kind !is KtFakeSourceElementKind }?.accept(finder)
        return finder.found
    }

    private fun createsDefaultSharedFlow(
        expression: FirExpression,
        session: FirSession,
        visiting: Set<FirNamedFunctionSymbol>,
    ): Boolean {
        val finder = DefaultSharedFlowFinder(session, visiting)
        expression.accept(finder)
        return finder.found
    }

    // A call to a function other than the kotlinx factory that returns a
    // MutableSharedFlow, whose source body returns one created with no
    // arguments.
    private fun callsLossyFactory(
        call: FirFunctionCall,
        session: FirSession,
        visiting: Set<FirNamedFunctionSymbol>,
    ): Boolean {
        val function = call.calleeReference.toResolvedCallableSymbol() as? FirNamedFunctionSymbol ?: return false
        if (function.callableId == mutableSharedFlow || function in visiting) return false
        val returned = call.resolvedType.fullyExpandedType(session).lowerBoundIfFlexible()
        if (returned.classId != mutableSharedFlowClass) return false
        return returnsDefaultSharedFlow(function, session, visiting + function)
    }

    // True when a value [function] returns creates a MutableSharedFlow with no
    // arguments: the returned expression itself, or a local `val` it returns.
    // A function without a visible body (abstract, expect, a library binary)
    // shows nothing, so it does not count.
    @OptIn(SymbolInternals::class)
    private fun returnsDefaultSharedFlow(
        function: FirNamedFunctionSymbol,
        session: FirSession,
        visiting: Set<FirNamedFunctionSymbol>,
    ): Boolean {
        val body = function.fir.body ?: return false
        val results = ArrayList<FirExpression>()
        body.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                if (element is FirReturnExpression && element.target.labeledElement.symbol == function) {
                    results += element.result
                }
                element.acceptChildren(this)
            }
        })
        return results.any { result ->
            createsDefaultSharedFlow(result, session, visiting) ||
                localValInitializer(result)?.let { createsDefaultSharedFlow(it, session, visiting) } == true
        }
    }

    // The initializer of the local `val` an expression reads.
    private fun localValInitializer(expression: FirExpression): FirExpression? {
        val value = if (expression is FirSmartCastExpression) expression.originalExpression else expression
        val access = value as? FirPropertyAccessExpression ?: return null
        val symbol = access.calleeReference.toResolvedCallableSymbol() as? FirPropertySymbol ?: return null
        if (!symbol.isLocal || symbol.isVar) return null
        return symbol.resolvedInitializer
    }

    // Finds a written `MutableSharedFlow(...)` call with no arguments, or a
    // call to a lossy factory, anywhere in the subtree: lambdas, anonymous
    // objects, and local declarations included. [visiting] holds the
    // factories being followed, so a recursive factory terminates.
    private class DefaultSharedFlowFinder(
        private val session: FirSession,
        private val visiting: Set<FirNamedFunctionSymbol>,
    ) : FirVisitorVoid() {
        var found = false

        override fun visitElement(element: FirElement) {
            if (!found) element.acceptChildren(this)
        }

        override fun visitFunctionCall(functionCall: FirFunctionCall) {
            if (found) return
            if (functionCall.source?.kind is KtRealSourceElementKind && isDefaultSharedFlow(functionCall)) {
                found = true
                return
            }
            functionCall.acceptChildren(this)
        }

        private fun isDefaultSharedFlow(call: FirFunctionCall): Boolean {
            val callee = call.calleeReference.toResolvedCallableSymbol()
            if (callee?.callableId == mutableSharedFlow) return call.argumentList.arguments.isEmpty()
            return callsLossyFactory(call, session, visiting)
        }
    }
}
