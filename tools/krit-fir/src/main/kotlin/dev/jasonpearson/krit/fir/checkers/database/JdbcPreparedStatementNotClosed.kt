package dev.jasonpearson.krit.fir.checkers.database

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.canHoldResource
import dev.jasonpearson.krit.fir.support.classIdOf
import dev.jasonpearson.krit.fir.support.expandedClassType
import dev.jasonpearson.krit.fir.support.firstLineAnchor
import dev.jasonpearson.krit.fir.support.hasCleanupCallInScope
import dev.jasonpearson.krit.fir.support.identifierText
import dev.jasonpearson.krit.fir.support.isHandedOverAfter
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.DeclarationCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.FirPropertyChecker
import org.jetbrains.kotlin.fir.analysis.checkers.unsubstitutedScope
import org.jetbrains.kotlin.fir.declarations.FirProperty
import org.jetbrains.kotlin.fir.expressions.FirCheckNotNullCall
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirSafeCallExpression
import org.jetbrains.kotlin.fir.expressions.FirSmartCastExpression
import org.jetbrains.kotlin.fir.expressions.arguments
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.resolve.lookupSuperTypes
import org.jetbrains.kotlin.fir.resolve.toClassSymbol
import org.jetbrains.kotlin.fir.scopes.getFunctions
import org.jetbrains.kotlin.fir.symbols.ConeTypeParameterLookupTag
import org.jetbrains.kotlin.fir.types.ConeCapturedType
import org.jetbrains.kotlin.fir.types.ConeClassLikeType
import org.jetbrains.kotlin.fir.types.ConeDefinitelyNotNullType
import org.jetbrains.kotlin.fir.types.ConeIntersectionType
import org.jetbrains.kotlin.fir.types.ConeKotlinType
import org.jetbrains.kotlin.fir.types.ConeKotlinTypeProjection
import org.jetbrains.kotlin.fir.types.ConeTypeParameterType
import org.jetbrains.kotlin.fir.types.ProjectionKind
import org.jetbrains.kotlin.fir.types.classId
import org.jetbrains.kotlin.fir.types.coneType
import org.jetbrains.kotlin.fir.types.isSomeFunctionType
import org.jetbrains.kotlin.fir.types.lowerBoundIfFlexible
import org.jetbrains.kotlin.fir.types.resolvedType
import org.jetbrains.kotlin.fir.types.returnType
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid
import org.jetbrains.kotlin.lexer.KtTokens
import org.jetbrains.kotlin.name.Name

/**
 * Port of the Go JdbcPreparedStatementNotClosed rule: a named property
 * (local, member, or top-level) whose initializer, delegate, or getter body
 * calls `prepareStatement(...)`, with no `<name>.close(...)` or
 * `<name>.use ...` in the rest of its scope. Reported on the property's first
 * line (its first modifier or annotation, else `val`/`var`), like Go.
 *
 * Go walks every call in the property, lambdas and nested declarations
 * included, and matches the call by name; its cleanup search is a text search
 * of the later statements (or members, or top-level declarations) of the
 * property's parent for `<name>.close(` or `<name>.use` with an identifier
 * boundary before the name. This checker keeps that scope, both the calls
 * counted (a statement made in a lambda, getter, or `by lazy`, and a chain
 * such as `prepareStatement(sql).executeQuery()` that leaks the statement)
 * and the cleanup shapes (a receiver qualified as `this.stmt`, a `close` or
 * `use` nested anywhere in a later statement).
 *
 * A statement handed to other code is not leaked by this scope: a resolved
 * read of the property after it that is not the receiver of a member access,
 * an assignment target, or a comparison returns, stores, or passes the
 * statement on (see `isHandedOverAfter`), as for SqliteCursorWithoutClose.
 * Only a property that can hold the statement hands it over; one holding a
 * value read from it (`prepareStatement(sql).executeUpdate()`) does not. Go
 * reads any later identifier of the same name as a hand-over.
 *
 * Deliberate differences from Go, each pinned in the golden data
 * (`JdbcPreparedStatementNotClosed*.kt`):
 * - The `prepareStatement` call must return a closeable type
 *   (`AutoCloseable`, a subtype, a class with a `close` member function, or a
 *   type parameter bounded by one of those); a lookalike returning anything
 *   else makes no statement to close.
 * - A call closed on the spot (`prepareStatement(sql).use { }`,
 *   `?.close()`, `!!.use { }`) is not a leak. A statement made by a property
 *   nested in the initializer or getter (a local, or a member of an
 *   `object { }`) is checked and reported on that property's own line, so it
 *   does not count for an outer property whose value is not closeable (a
 *   `Boolean` result). It still counts when the outer value is closeable, or a
 *   function returning one, since the nested statement may be returned into it
 *   (`val stmt = run { val s = conn.prepareStatement(sql); s }`). A
 *   destructuring declaration is not checked on its own, so its statement
 *   always counts.
 * - A setter body is not searched: what it makes is not the property's value
 *   (Go never sees a setter body either).
 * - The cleanup is read from the syntax tree, not the text: a safe call or
 *   `!!` receiver (`stmt?.close()`, `stmt!!.use { }`), a backticked name,
 *   and a line break or comment before the call (`stmt\n    .close()`)
 *   count; a `stmt.close()` in a comment or a string literal does not, and
 *   neither does a call merely starting with `use` (`stmt.useless()`).
 * - A member or top-level property also counts a close in an earlier member
 *   or declaration (`override fun close() { stmt.close() }` above the
 *   property); Go only reads the declarations after it.
 * - A `when (val s = conn.prepareStatement(sql))` subject is a property here;
 *   tree-sitter has no property declaration for it, so Go never checks it.
 */
internal object JdbcPreparedStatementNotClosed :
    FirPropertyChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "JdbcPreparedStatementNotClosed"
    override val declarationCheckers = object : DeclarationCheckers() {
        override val propertyCheckers = setOf(JdbcPreparedStatementNotClosed)
    }

    private val prepareStatement = Name.identifier("prepareStatement")
    private val cleanupNames = setOf("close", "use")
    private val close = Name.identifier("close")
    private const val MAX_TYPE_DEPTH = 16
    private val autoCloseable = setOf(
        classIdOf("java.lang", "AutoCloseable"),
        classIdOf("kotlin", "AutoCloseable"),
    )

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(declaration: FirProperty) {
        val source = declaration.source ?: return
        if (source.kind !is KtRealSourceElementKind) return
        if (source.elementType != KtNodeTypes.PROPERTY) return
        val name = identifierText(source) ?: return
        val statement = preparedStatementType(declaration) ?: return
        if (hasCleanupCallInScope(source, name)) return
        if (holdsStatement(declaration.returnTypeRef.coneType, statement) && isHandedOverAfter(declaration, source)) return
        report(
            anchor(source),
            "PreparedStatement '$name' should be wrapped in use { } or explicitly closed with .close().",
        )
    }

    // The type of a closeable `prepareStatement(...)` call the property's
    // initializer, delegate, or written getter body makes and does not close
    // on the spot, or null when there is none. A setter body is not searched:
    // what the setter makes is not the property's value (and Go never sees a
    // setter either).
    context(context: CheckerContext)
    private fun preparedStatementType(declaration: FirProperty): ConeKotlinType? {
        val roots = buildList<FirElement> {
            declaration.initializer?.let(::add)
            declaration.delegate?.let(::add)
            val getter = declaration.getter
            if (getter != null && getter.source?.kind is KtRealSourceElementKind) {
                getter.body?.let(::add)
            }
        }
        // A nested property is checked (and reported) on its own line, so its
        // statement is not counted again here, unless this property holds a
        // closeable value too: the nested statement may be returned into it
        // (`val stmt = run { val s = conn.prepareStatement(sql); s }`).
        val skipNested = !holdsCloseable(declaration.returnTypeRef.coneType)
        var found: ConeKotlinType? = null
        val closedOnTheSpot = HashSet<FirExpression>()
        val visitor = object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                if (found != null) return
                if (skipNested && element is FirProperty && isCheckedOnItsOwn(element)) return
                when (element) {
                    is FirSafeCallExpression -> {
                        val selector = element.selector as? FirFunctionCall
                        if (selector != null && selector.calleeReference.name.asString() in cleanupNames) {
                            closedOnTheSpot += unwrap(element.receiver)
                        }
                    }
                    is FirFunctionCall -> {
                        if (element.calleeReference.name.asString() in cleanupNames) {
                            element.explicitReceiver?.let { closedOnTheSpot += unwrap(it) }
                        }
                        if (element.calleeReference.name == prepareStatement &&
                            element !in closedOnTheSpot &&
                            isCloseable(element.resolvedType, depth = 0)
                        ) {
                            found = element.resolvedType
                            return
                        }
                    }
                    else -> Unit
                }
                element.acceptChildren(this)
            }
        }
        for (root in roots) {
            root.accept(visitor)
            found?.let { return it }
        }
        return null
    }

    // The properties this checker checks by itself: a real `val`/`var`, not a
    // destructuring declaration or its entries.
    private fun isCheckedOnItsOwn(property: FirProperty): Boolean {
        val source = property.source ?: return false
        return source.kind is KtRealSourceElementKind && source.elementType == KtNodeTypes.PROPERTY
    }

    // The property's value can be the statement itself, or a function
    // returning it, so handing the property over hands the statement over. A
    // value read from the statement (`prepareStatement(sql).executeQuery()`)
    // leaves the statement open whatever happens to that value.
    context(context: CheckerContext)
    private fun holdsStatement(type: ConeKotlinType, statement: ConeKotlinType): Boolean {
        if (canHoldResource(type, statement)) return true
        val function = type.expandedClassType() ?: return false
        if (!function.isSomeFunctionType(context.session)) return false
        return canHoldResource(function.returnType(context.session), statement)
    }

    // A closeable value, or a function returning one (every call makes one).
    context(context: CheckerContext)
    private fun holdsCloseable(type: ConeKotlinType): Boolean {
        if (isCloseable(type, depth = 0)) return true
        val function = type.expandedClassType() ?: return false
        if (!function.isSomeFunctionType(context.session)) return false
        return isCloseable(function.returnType(context.session), depth = 0)
    }

    private tailrec fun unwrap(expression: FirExpression): FirExpression = when (expression) {
        is FirCheckNotNullCall -> unwrap(expression.arguments.firstOrNull() ?: return expression)
        is FirSmartCastExpression -> unwrap(expression.originalExpression)
        is FirSafeCallExpression -> unwrap(expression.selector as? FirExpression ?: return expression)
        else -> expression
    }

    // A type the caller has to close: AutoCloseable itself, a subtype of it, a
    // class with a `close` member function (inherited or declared), or a type
    // parameter with such a bound.
    context(context: CheckerContext)
    private fun isCloseable(type: ConeKotlinType, depth: Int): Boolean {
        if (depth > MAX_TYPE_DEPTH) return false
        val session = context.session
        return when (val bound = type.fullyExpandedType().lowerBoundIfFlexible()) {
            is ConeDefinitelyNotNullType -> isCloseable(bound.original, depth + 1)
            is ConeIntersectionType -> bound.intersectedTypes.any { isCloseable(it, depth + 1) }
            is ConeTypeParameterType -> bound.lookupTag.typeParameterSymbol.resolvedBounds.any {
                isCloseable(it.coneType, depth + 1)
            }
            // `Factory<out PreparedStatement>` or `BoundedFactory<*>`: the
            // captured type's upper bounds. An `in` projection bounds it only
            // from below, which proves nothing closeable.
            is ConeCapturedType -> {
                val constructor = bound.constructor
                val projected = (constructor.projection as? ConeKotlinTypeProjection)
                    ?.takeIf { it.kind == ProjectionKind.OUT || it.kind == ProjectionKind.INVARIANT }
                    ?.type
                (projected != null && isCloseable(projected, depth + 1)) ||
                    constructor.supertypes.orEmpty().any { isCloseable(it, depth + 1) } ||
                    (constructor.typeParameterMarker as? ConeTypeParameterLookupTag)
                        ?.typeParameterSymbol?.resolvedBounds.orEmpty()
                        .any { isCloseable(it.coneType, depth + 1) }
            }
            is ConeClassLikeType -> {
                if (bound.classId in autoCloseable) return true
                // The lookup tag is bound to local classes, so no class id is
                // resolved from a symbol that may be local.
                val symbol = bound.lookupTag.toClassSymbol(session) ?: return false
                lookupSuperTypes(symbol, lookupInterfaces = true, deep = true, useSiteSession = session)
                    .any { it.fullyExpandedType().classId in autoCloseable } ||
                    symbol.unsubstitutedScope().getFunctions(close).any { it.receiverParameterSymbol == null }
            }
            else -> false
        }
    }

    // Go reports the property's first line: its first modifier or
    // annotation, else `val`/`var` (a KDoc above it is not part of Go's node).
    private fun anchor(source: KtSourceElement): KtSourceElement =
        firstLineAnchor(source, KtTokens.VAL_KEYWORD, KtTokens.VAR_KEYWORD) ?: source
}
