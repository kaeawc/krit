package dev.jasonpearson.krit.fir.checkers.database

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.unsubstitutedScope
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.DeclarationCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.FirDeclarationChecker
import org.jetbrains.kotlin.fir.declarations.FirNamedFunction
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.resolve.lookupSuperTypes
import org.jetbrains.kotlin.fir.resolve.toClassSymbol
import org.jetbrains.kotlin.fir.scopes.getFunctions
import org.jetbrains.kotlin.fir.types.ConeClassLikeType
import org.jetbrains.kotlin.fir.types.ConeDefinitelyNotNullType
import org.jetbrains.kotlin.fir.types.ConeIntersectionType
import org.jetbrains.kotlin.fir.types.ConeKotlinType
import org.jetbrains.kotlin.fir.types.ConeTypeParameterType
import org.jetbrains.kotlin.fir.types.classId
import org.jetbrains.kotlin.fir.types.coneType
import org.jetbrains.kotlin.fir.types.lowerBoundIfFlexible
import org.jetbrains.kotlin.lexer.KtTokens
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.text
import org.jetbrains.kotlin.util.getChildren

/**
 * Port of the Go JdbcResultSetLeakedFromFunction rule: a named function with a
 * body (block or expression) whose return type is `java.sql.ResultSet`,
 * nullable or not, reported on the function's first line (its modifier list,
 * else `fun`), like Go. Top-level, member, interface default, object,
 * companion, anonymous-object, local, extension, override, and suspend
 * functions all count, as every Go `function_declaration` does; functions
 * without a body (abstract, interface, expect, external), anonymous functions,
 * lambdas, and property accessors do not, as in Go. Compiler-generated
 * functions (a data class's `componentN`/`copy`) have no source to report.
 *
 * Go reads the declared type's text: any type whose last dotted segment is
 * `ResultSet`. Any other type named `ResultSet` (a class, a type alias or
 * import alias, or a type parameter) is reported here too when the caller has
 * to close it, since the message holds: it is `AutoCloseable` itself or a
 * subtype (a wrapper, another driver's cursor), a class with a `close` member
 * function, or a type parameter bounded by one of those.
 *
 * Deliberate differences from Go, each pinned in the golden data
 * (`JdbcResultSetLeakedFromFunction*.kt`):
 * - Go reports any declared type named `ResultSet`, so it reports a
 *   non-closeable lookalike class of that name, a type parameter of that name
 *   with no closeable bound, and a function type whose text ends in
 *   `.ResultSet` (`() -> java.sql.ResultSet`). None has anything to close, so
 *   none is reported here.
 * - Resolution sees a `java.sql.ResultSet` return Go misses: an inferred
 *   (expression-body) return type, a type alias, an import alias, and a
 *   parenthesized type, nullable or not. It also sees a closeable type named
 *   `ResultSet` whose written text Go does not match: a generic class
 *   (`Pool.ResultSet<Int>`), an inferred return type, and a type alias or
 *   import alias with another name.
 */
internal object JdbcResultSetLeakedFromFunction :
    FirDeclarationChecker<FirNamedFunction>(MppCheckerKind.Common), FirRule {
    override val ruleId = "JdbcResultSetLeakedFromFunction"
    override val declarationCheckers = object : DeclarationCheckers() {
        override val simpleFunctionCheckers = setOf(JdbcResultSetLeakedFromFunction)
    }

    private const val RESULT_SET = "ResultSet"
    private val resultSet = ClassId(FqName("java.sql"), Name.identifier(RESULT_SET))
    private val close = Name.identifier("close")
    private const val MAX_TYPE_DEPTH = 16
    private val autoCloseable = setOf(
        ClassId(FqName("java.lang"), Name.identifier("AutoCloseable")),
        ClassId(FqName("kotlin"), Name.identifier("AutoCloseable")),
    )

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(declaration: FirNamedFunction) {
        val source = declaration.source ?: return
        if (source.kind !is KtRealSourceElementKind) return
        if (declaration.body == null) return
        if (!returnsResultSet(declaration)) return
        val name = functionNameText(source) ?: declaration.name.asString()
        report(
            source,
            "Function '$name' returns ResultSet; callers almost always forget to close it. " +
                "Accept a (ResultSet) -> R block and call .use {} instead.",
        )
    }

    context(context: CheckerContext)
    private fun returnsResultSet(declaration: FirNamedFunction): Boolean {
        val declared = declaration.returnTypeRef.coneType.lowerBoundIfFlexible()
        val expanded = declared.fullyExpandedType().lowerBoundIfFlexible()
        if (expanded is ConeClassLikeType && expanded.classId == resultSet) return true
        val named = shortName(expanded) == RESULT_SET ||
            shortName(declared) == RESULT_SET ||
            writtenTypeName(declaration) == RESULT_SET
        return named && isCloseable(expanded, depth = 0)
    }

    // The simple name of a class, type alias, or type parameter type.
    private fun shortName(type: ConeKotlinType): String? = when (type) {
        is ConeClassLikeType -> type.classId.shortClassName.asString()
        is ConeTypeParameterType -> type.lookupTag.name.asString()
        else -> null
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

    // Go's reading of the declared type: its text without a trailing `?`, and
    // only the last dotted segment.
    private fun writtenTypeName(declaration: FirNamedFunction): String? {
        val source = declaration.returnTypeRef.source ?: return null
        if (source.kind !is KtRealSourceElementKind) return null
        return source.text?.toString()?.trim()?.removeSuffix("?")?.substringAfterLast('.')
    }

    // The name as written, backticks included, which is what Go interpolates.
    private fun functionNameText(source: KtSourceElement): String? {
        val tree = source.treeStructure
        val identifier = source.lighterASTNode.getChildren(tree)
            .firstOrNull { it.tokenType == KtTokens.IDENTIFIER } ?: return null
        return tree.toString(identifier).toString()
    }
}
