package dev.jasonpearson.krit.fir.checkers.database

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.canHoldResource
import dev.jasonpearson.krit.fir.support.classIdOf
import dev.jasonpearson.krit.fir.support.hasCleanupCallInScope
import dev.jasonpearson.krit.fir.support.identifierAnchor
import dev.jasonpearson.krit.fir.support.identifierText
import dev.jasonpearson.krit.fir.support.isHandedOverAfter
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
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirSafeCallExpression
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.resolve.getContainingClassSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirClassSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirNamedFunctionSymbol
import org.jetbrains.kotlin.fir.types.ConeKotlinType
import org.jetbrains.kotlin.fir.types.coneType
import org.jetbrains.kotlin.fir.types.resolvedType
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid
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
 * receiver of a member access, an assignment target, or a comparison (see `isHandedOverAfter`);
 * Go matches any identifier of the same name there, so a shadowing local or a named argument hides
 * a leak from it. Only a property that can hold the cursor hands it over: one holding a value read
 * from it (`rawQuery(...).count`) leaves the cursor open whatever happens to that value.
 */
internal object SqliteCursorWithoutClose :
    FirPropertyChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "SqliteCursorWithoutClose"
    override val declarationCheckers = object : DeclarationCheckers() {
        override val propertyCheckers = setOf(SqliteCursorWithoutClose)
    }

    private val queryNames = setOf(Name.identifier("rawQuery"), Name.identifier("query"))
    private val closeNames = setOf("close", "use")
    private const val USE = "use"
    private val databaseTypes = setOf(
        classIdOf("android.database.sqlite", "SQLiteDatabase"),
        classIdOf("androidx.sqlite.db", "SupportSQLiteDatabase"),
    )

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(declaration: FirProperty) {
        val source = declaration.source ?: return
        if (source.kind !is KtRealSourceElementKind) return
        if (source.elementType != KtNodeTypes.PROPERTY) return
        val name = identifierText(source) ?: return
        val cursor = openedCursorType(declaration) ?: return
        if (hasCleanupCallInScope(source, name)) return
        if (canHoldResource(declaration.returnTypeRef.coneType, cursor) && isHandedOverAfter(declaration, source)) return
        report(
            anchor(source),
            "Cursor '$name' from rawQuery/query should be wrapped in use { } or explicitly closed with .close().",
        )
    }

    // The type of a database query the initializer, delegate, or written getter body makes in the
    // property's own scope that nothing closes on the spot, or null when there is none.
    context(context: CheckerContext)
    private fun openedCursorType(declaration: FirProperty): ConeKotlinType? {
        val roots = buildList<FirElement> {
            declaration.initializer?.let(::add)
            declaration.delegate?.let(::add)
            val getter = declaration.getter
            if (getter != null && getter.source?.kind is KtRealSourceElementKind) {
                getter.body?.let(::add)
            }
        }
        var found: ConeKotlinType? = null
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
                        if (callee == USE) usesAnywhere = true
                        if (found == null && nested == 0 && element !in closedOnTheSpot && isDatabaseQuery(element)) {
                            found = element.resolvedType
                        }
                    }
                    else -> Unit
                }
                if (boundary) nested++
                element.acceptChildren(this)
                if (boundary) nested--
            }
        }
        roots.forEach { it.accept(visitor) }
        return found.takeUnless { usesAnywhere }
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

    // Go reports the property's name identifier.
    private fun anchor(source: KtSourceElement): KtSourceElement = identifierAnchor(source) ?: source
}
