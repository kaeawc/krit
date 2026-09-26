package dev.jasonpearson.krit.fir.checkers.coroutines

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import org.jetbrains.kotlin.KtFakeSourceElementKind
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.DeclarationCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.FirPropertyChecker
import org.jetbrains.kotlin.fir.declarations.FirProperty
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid
import org.jetbrains.kotlin.name.CallableId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.name.SpecialNames

/**
 * Port of the Go SharedFlowWithoutReplay rule: a `val`/`var` declaration
 * (top-level, member, or local) whose initializer, delegate, or accessors
 * create a kotlinx.coroutines.flow.MutableSharedFlow with no arguments at all
 * (`MutableSharedFlow<T>()`, or `MutableSharedFlow()` with an inferred type
 * argument), reported once per declaration on its first line (its modifier
 * list, or `val`/`var` when it has none), like Go.
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
 * Deliberate precision differences from Go's substring match on the
 * declaration text (see the golden data): no finding for a declaration that
 * only mentions `MutableSharedFlow<` next to an unrelated `>()` call, a
 * same-named local function, or the word in a comment or string; and a
 * finding where Go skips the whole declaration because some other call in it
 * is spelled `MutableSharedFlow(replay`, `MutableSharedFlow(extraBufferCapacity`,
 * or `MutableSharedFlow(` followed by a line break, where the no-argument
 * call is spelled through an import alias or with whitespace inside `()`, or
 * where it sits in a getter or setter on the line after the property
 * (tree-sitter parses that accessor as a sibling of the declaration).
 */
internal object SharedFlowWithoutReplay : FirPropertyChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "SharedFlowWithoutReplay"
    override val declarationCheckers = object : DeclarationCheckers() {
        override val propertyCheckers = setOf(SharedFlowWithoutReplay)
    }

    private val mutableSharedFlow = CallableId(FqName("kotlinx.coroutines.flow"), Name.identifier("MutableSharedFlow"))

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(declaration: FirProperty) {
        val source = declaration.source ?: return
        val destructuring = isDestructuring(declaration, source)
        if (!destructuring && !isPropertyStatement(source)) return
        if (!createsDefaultSharedFlow(declaration)) return
        // Go names the declaration by its first direct identifier child; a
        // destructuring declaration has none, so Go's message names ''.
        val name = if (destructuring) "" else declaration.name.asString()
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

    private fun createsDefaultSharedFlow(declaration: FirProperty): Boolean {
        val finder = DefaultSharedFlowFinder()
        declaration.initializer?.accept(finder)
        declaration.delegate?.accept(finder)
        declaration.getter?.takeIf { it.source?.kind !is KtFakeSourceElementKind }?.accept(finder)
        declaration.setter?.takeIf { it.source?.kind !is KtFakeSourceElementKind }?.accept(finder)
        return finder.found
    }

    // Finds a written `MutableSharedFlow(...)` call with no arguments anywhere in
    // the subtree, lambdas, anonymous objects, and local declarations included.
    private class DefaultSharedFlowFinder : FirVisitorVoid() {
        var found = false

        override fun visitElement(element: FirElement) {
            if (!found) element.acceptChildren(this)
        }

        override fun visitFunctionCall(functionCall: FirFunctionCall) {
            if (found) return
            if (functionCall.source?.kind is KtRealSourceElementKind &&
                functionCall.argumentList.arguments.isEmpty() &&
                functionCall.calleeReference.toResolvedCallableSymbol()?.callableId == mutableSharedFlow
            ) {
                found = true
                return
            }
            functionCall.acceptChildren(this)
        }
    }
}
