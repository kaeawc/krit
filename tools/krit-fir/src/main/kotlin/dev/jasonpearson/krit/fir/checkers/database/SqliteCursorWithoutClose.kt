package dev.jasonpearson.krit.fir.checkers.database

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.assignedAccess
import dev.jasonpearson.krit.fir.support.hasCleanupCallInScope
import dev.jasonpearson.krit.fir.support.identifierAnchor
import dev.jasonpearson.krit.fir.support.identifierText
import dev.jasonpearson.krit.fir.support.isOrInheritsFrom
import dev.jasonpearson.krit.fir.support.unwrapSmartCasts
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.DeclarationCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.FirPropertyChecker
import org.jetbrains.kotlin.fir.declarations.FirAnonymousFunction
import org.jetbrains.kotlin.fir.declarations.FirClass
import org.jetbrains.kotlin.fir.declarations.FirNamedFunction
import org.jetbrains.kotlin.fir.declarations.FirProperty
import org.jetbrains.kotlin.fir.expressions.FirEqualityOperatorCall
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirOperation
import org.jetbrains.kotlin.fir.expressions.FirPropertyAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirQualifiedAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirSafeCallExpression
import org.jetbrains.kotlin.fir.expressions.FirTypeOperatorCall
import org.jetbrains.kotlin.fir.expressions.FirVariableAssignment
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.resolve.getContainingClassSymbol
import org.jetbrains.kotlin.fir.symbols.SymbolInternals
import org.jetbrains.kotlin.fir.symbols.impl.FirClassSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirNamedFunctionSymbol
import org.jetbrains.kotlin.fir.unwrapFakeOverrides
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name

/**
 * Port of the Go SqliteCursorWithoutClose rule: a named property (local, member, or top-level)
 * whose initializer, delegate, or getter body calls `rawQuery(...)` or `query(...)` on a
 * `SQLiteDatabase` or `SupportSQLiteDatabase`, with no `<name>.close(...)` or `<name>.use ...` in
 * the rest of its scope and no hand-over of the cursor (returned, stored, or passed on). Reported
 * on the property's name, like Go.
 *
 * Go matches the call by name and proves the receiver only when it is a name whose declared type
 * its resolver knows. This checker resolves the callee, so a receiver written as a call chain
 * (`helper.readableDatabase.rawQuery(...)`), an implicit receiver, or a subtype of the database
 * types counts too, and a same-named method of another class never does. The call must sit in
 * the property's own scope: calls inside a lambda, local function, or nested object belong to
 * another scope, as in Go.
 *
 * A cursor is not leaked when it is closed on the spot (`rawQuery(...).use { }`, `?.close()`) or
 * when the initializer calls `use` anywhere, as Go skips any initializer that contains `.use`.
 * The cleanup search is Go's, read from the syntax tree (see `hasCleanupCallInScope`). A cursor
 * is handed over when a resolved reference to the property after its declaration is not the
 * receiver of a member access, an assignment target, or a comparison; Go matches any identifier
 * of the same name there, so a shadowing local or a named argument hides a leak from it.
 */
internal object SqliteCursorWithoutClose :
    FirPropertyChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "SqliteCursorWithoutClose"
    override val declarationCheckers = object : DeclarationCheckers() {
        override val propertyCheckers = setOf(SqliteCursorWithoutClose)
    }

    private val queryNames = setOf(Name.identifier("rawQuery"), Name.identifier("query"))
    private val closeNames = setOf("close", "use")
    private val use = "use"
    private val databaseTypes = setOf(
        ClassId(FqName("android.database.sqlite"), Name.identifier("SQLiteDatabase")),
        ClassId(FqName("androidx.sqlite.db"), Name.identifier("SupportSQLiteDatabase")),
    )

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(declaration: FirProperty) {
        val source = declaration.source ?: return
        if (source.kind !is KtRealSourceElementKind) return
        if (source.elementType != KtNodeTypes.PROPERTY) return
        val name = identifierText(source) ?: return
        if (!opensCursor(declaration)) return
        if (hasCleanupCallInScope(source, name)) return
        if (handsOver(declaration, source)) return
        report(
            anchor(source),
            "Cursor '$name' from rawQuery/query should be wrapped in use { } or explicitly closed with .close().",
        )
    }

    // The initializer, delegate, or written getter body makes a database query in the property's
    // own scope that nothing closes on the spot.
    context(context: CheckerContext)
    private fun opensCursor(declaration: FirProperty): Boolean {
        val roots = buildList<FirElement> {
            declaration.initializer?.let(::add)
            declaration.delegate?.let(::add)
            val getter = declaration.getter
            if (getter != null && getter.source?.kind is KtRealSourceElementKind) {
                getter.body?.let(::add)
            }
        }
        var found = false
        var usesAnywhere = false
        val closedOnTheSpot = HashSet<FirExpression>()
        var nested = 0
        val visitor = object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                val boundary = element is FirAnonymousFunction || element is FirNamedFunction || element is FirClass
                when (element) {
                    is FirSafeCallExpression -> {
                        val selector = element.selector as? FirFunctionCall
                        if (selector?.calleeReference?.name?.asString() in closeNames) {
                            closedOnTheSpot += unwrapSmartCasts(element.receiver, throughNotNull = true)
                        }
                    }
                    is FirFunctionCall -> {
                        val callee = element.calleeReference.name.asString()
                        if (callee in closeNames) {
                            element.explicitReceiver?.let { closedOnTheSpot += unwrapSmartCasts(it, throughNotNull = true) }
                        }
                        if (callee == use) usesAnywhere = true
                        if (nested == 0 && element !in closedOnTheSpot && isDatabaseQuery(element)) found = true
                    }
                    else -> Unit
                }
                if (boundary) nested++
                element.acceptChildren(this)
                if (boundary) nested--
            }
        }
        roots.forEach { it.accept(visitor) }
        return found && !usesAnywhere
    }

    // `rawQuery`/`query` declared in, or inherited from, SQLiteDatabase or SupportSQLiteDatabase.
    // The owner comes from the symbol's lookup tag, which is bound for local classes too.
    context(context: CheckerContext)
    private fun isDatabaseQuery(call: FirFunctionCall): Boolean {
        val callee = call.calleeReference.toResolvedCallableSymbol() as? FirNamedFunctionSymbol ?: return false
        if (callee.name !in queryNames || callee.receiverParameterSymbol != null) return false
        val owner = callee.getContainingClassSymbol() as? FirClassSymbol<*> ?: return false
        return databaseTypes.any { isOrInheritsFrom(owner, it, context.session, lookupInterfaces = true) }
    }

    // A read of the property after its declaration that is more than a member access on it, an
    // assignment to it, or a comparison: the cursor is returned, stored, or passed to other code,
    // which then owns closing it.
    @OptIn(SymbolInternals::class)
    context(context: CheckerContext)
    private fun handsOver(declaration: FirProperty, source: KtSourceElement): Boolean {
        val scope = context.containingDeclarations.lastOrNull { it != declaration.symbol }
            ?: context.containingFileSymbol
            ?: return false
        val root = scope.fir
        val symbol = declaration.symbol
        val declarationEnd = source.endOffset
        val notHandedOver = HashSet<FirExpression>()
        var handedOver = false
        fun exempt(expression: FirExpression?) {
            if (expression == null) return
            notHandedOver += expression
            notHandedOver += unwrapSmartCasts(expression, throughNotNull = true)
        }
        root.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                if (handedOver) return
                when (element) {
                    is FirPropertyAccessExpression -> {
                        if (element !in notHandedOver &&
                            (element.source?.startOffset ?: -1) >= declarationEnd &&
                            element.calleeReference.toResolvedCallableSymbol()?.unwrapFakeOverrides() == symbol
                        ) {
                            handedOver = true
                            return
                        }
                    }
                    is FirSafeCallExpression -> exempt(element.receiver)
                    is FirVariableAssignment -> assignedAccess(element)?.let { exempt(it) }
                    is FirEqualityOperatorCall -> element.argumentList.arguments.forEach(::exempt)
                    is FirTypeOperatorCall ->
                        if (element.operation == FirOperation.IS || element.operation == FirOperation.NOT_IS) {
                            element.argumentList.arguments.forEach(::exempt)
                        }
                    else -> Unit
                }
                if (element is FirQualifiedAccessExpression) {
                    exempt(element.explicitReceiver)
                    exempt(element.extensionReceiver)
                    exempt(element.dispatchReceiver)
                }
                element.acceptChildren(this)
            }
        })
        return handedOver
    }

    // Go reports the property's name identifier.
    private fun anchor(source: KtSourceElement): KtSourceElement = identifierAnchor(source) ?: source
}
