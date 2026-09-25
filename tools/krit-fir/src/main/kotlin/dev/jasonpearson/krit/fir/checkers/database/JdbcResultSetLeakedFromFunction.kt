package dev.jasonpearson.krit.fir.checkers.database

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirSession
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.DeclarationCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.FirDeclarationChecker
import org.jetbrains.kotlin.fir.declarations.FirNamedFunction
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.resolve.lookupSuperTypes
import org.jetbrains.kotlin.fir.resolve.toClassSymbol
import org.jetbrains.kotlin.fir.types.ConeClassLikeType
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
 * `ResultSet`. A class of that name that is `AutoCloseable` (a wrapper, another
 * driver's cursor) is reported here too, since the message holds: the caller
 * must close it and `.use {}` applies.
 *
 * Deliberate differences from Go, each pinned in the golden data
 * (`JdbcResultSetLeakedFromFunction*.kt`):
 * - Go reports any declared type named `ResultSet`, so it reports a
 *   non-closeable lookalike class of that name. There is nothing to close, so
 *   it is not reported here.
 * - Resolution sees a `java.sql.ResultSet` return Go misses: an inferred
 *   (expression-body) return type, a type alias, an import alias, and a
 *   parenthesized type. It also sees a closeable generic class named
 *   `ResultSet` (`Pool.ResultSet<Int>`), whose text Go does not match.
 */
internal object JdbcResultSetLeakedFromFunction :
    FirDeclarationChecker<FirNamedFunction>(MppCheckerKind.Common), FirRule {
    override val ruleId = "JdbcResultSetLeakedFromFunction"
    override val declarationCheckers = object : DeclarationCheckers() {
        override val simpleFunctionCheckers = setOf(JdbcResultSetLeakedFromFunction)
    }

    private const val RESULT_SET = "ResultSet"
    private val resultSet = ClassId(FqName("java.sql"), Name.identifier(RESULT_SET))
    private val autoCloseable = setOf(
        ClassId(FqName("java.lang"), Name.identifier("AutoCloseable")),
        ClassId(FqName("kotlin"), Name.identifier("AutoCloseable")),
    )

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(declaration: FirNamedFunction) {
        val source = declaration.source ?: return
        if (source.kind !is KtRealSourceElementKind) return
        if (declaration.body == null) return
        if (!returnsResultSet(declaration, context.session)) return
        val name = functionNameText(source) ?: declaration.name.asString()
        report(
            source,
            "Function '$name' returns ResultSet; callers almost always forget to close it. " +
                "Accept a (ResultSet) -> R block and call .use {} instead.",
        )
    }

    private fun returnsResultSet(declaration: FirNamedFunction, session: FirSession): Boolean {
        val declared = declaration.returnTypeRef.coneType
        val expanded = declared.fullyExpandedType(session).lowerBoundIfFlexible() as? ConeClassLikeType ?: return false
        if (expanded.classId == resultSet) return true
        val named = expanded.classId?.shortClassName?.asString() == RESULT_SET ||
            (declared.lowerBoundIfFlexible() as? ConeClassLikeType)?.classId?.shortClassName?.asString() == RESULT_SET ||
            writtenTypeName(declaration) == RESULT_SET
        if (!named) return false
        // The lookup tag is bound to local classes, so no class id is resolved
        // from a symbol that may be local.
        val symbol = expanded.lookupTag.toClassSymbol(session) ?: return false
        return lookupSuperTypes(symbol, lookupInterfaces = true, deep = true, useSiteSession = session)
            .any { it.fullyExpandedType(session).classId in autoCloseable }
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
